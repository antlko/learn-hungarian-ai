<script setup lang="ts">
import InfoTip from '@/components/common/InfoTip.vue'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Headphones, Repeat, SkipForward, Speech, Upload } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import Note from '@/components/common/Note.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import { api } from '@/api'
import { useSpeechStore } from '@/stores/speech'
import type { ListenState, TextDetail, TextSummary } from '@/types'

/**
 * The listening loop: the three-pass protocol as UI state. [R13]
 *
 *   pass 1 — gist, transcript hidden. Comprehension without a crutch.
 *   pass 2 — with the transcript. Form–sound mapping.
 *   pass 3 — dictation / shadowing. Production from the ear.
 *
 * The pass is the learner's own claim, persisted server-side; the app never
 * advances it. Seconds actually played are reported as they happen and feed
 * the listening-hours gate on word-order topics — hours accumulate, they are
 * never claimed in a lump.
 *
 * **Two sources of sound, and a material needs only a transcript.** This screen
 * used to demand an uploaded audio file before it would do anything, so on a
 * fresh install it was an empty list above an upload form — a loop nobody
 * started, because finding a recording WITH a matching transcript is a research
 * task, not a study session. Any text can now be listened to: an uploaded
 * recording plays through the audio element, and everything else is read by the
 * learner's own device, sentence by sentence.
 *
 * Synthetic speech is worse input than a native recording and the screen says
 * so rather than blurring the two — the material list labels which is which,
 * and a device with no Hungarian voice gets the reason, not a dead button.
 */
const speech = useSpeechStore()

const materials = ref<TextSummary[]>([])
const detail = ref<TextDetail | null>(null)
const listen = ref<ListenState | null>(null)
const error = ref('')
const uploading = ref(false)
const loading = ref(true)

const upTitle = ref('')
const upTranscript = ref('')
const upFile = ref<File | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)

const player = ref<HTMLAudioElement | null>(null)
const speed = ref(1.0)
const loopA = ref<number | null>(null)
const loopB = ref<number | null>(null)

// Device-speech playback position: which sentence is being read.
const spokenIndex = ref(0)
const speakingAll = ref(false)

// Dictation (pass 3).
const dictSentence = ref(0)
const dictTyped = ref('')
const dictChecked = ref(false)

let heardAccum = 0
let lastTick = 0
let reportTimer: number | undefined

/** Whether this material has a recording, as opposed to being read aloud. */
const hasRecording = computed(() => Boolean(detail.value?.text.audioPath))

/** The coverage pre-check: below ~90% element coverage, warn before playback. */
const coverageWarning = computed(() => {
  const c = detail.value?.coverage
  return !!c && c.elementCoverage < 0.9
})

/**
 * Sentences reconstructed from stored tokens.
 *
 * Used for pass-3 dictation and, on a material with no recording, as the unit
 * of playback. The offsets are BYTE offsets into UTF-8, so the slice happens in
 * bytes and is decoded back — the same trap that put the review screen's cloze
 * blank in the middle of a word.
 */
const sentences = computed(() => {
  const d = detail.value
  if (!d) return []
  const bySent = new Map<number, { start: number; end: number }>()
  for (const t of d.tokens) {
    const cur = bySent.get(t.sentIdx)
    if (!cur) bySent.set(t.sentIdx, { start: t.start, end: t.end })
    else cur.end = Math.max(cur.end, t.end)
  }
  const body = d.text.body
  const enc = new TextEncoder().encode(body)
  const dec = new TextDecoder()
  return [...bySent.entries()]
    .sort(([a], [b]) => a - b)
    .map(([, r]) => dec.decode(enc.slice(r.start, r.end)).trim())
    .filter(Boolean)
})

/** Char diff for dictation: per-character match against the target sentence. */
const dictDiff = computed(() => {
  if (!dictChecked.value) return []
  const target = sentences.value[dictSentence.value] ?? ''
  const typed = dictTyped.value
  const out: { ch: string; ok: boolean }[] = []
  const t = [...target.toLowerCase()]
  const y = [...typed.toLowerCase()]
  for (let i = 0; i < Math.max(t.length, y.length); i++) {
    out.push({ ch: [...target][i] ?? '·', ok: t[i] === y[i] })
  }
  return out
})

/**
 * What is missing before this material can be created.
 *
 * The upload button used to be disabled with no explanation, which is
 * indistinguishable from a broken button — press it, nothing happens, conclude
 * the app does not work. It now says what it wants.
 */
const uploadBlocker = computed(() => {
  if (!upTranscript.value.trim()) return 'listen.needTranscript'
  return ''
})

async function load() {
  loading.value = true
  try {
    const res = await api.texts()
    materials.value = res.texts
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

async function open(id: number) {
  error.value = ''
  stopSpeaking()
  try {
    detail.value = await api.text(id)
    listen.value = await api.listenState(id)
    spokenIndex.value = 0
    dictSentence.value = 0
    dictTyped.value = ''
    dictChecked.value = false
  } catch (e) {
    error.value = (e as Error).message
  }
}

function close() {
  stopSpeaking()
  void report()
  detail.value = null
}

async function upload() {
  if (uploadBlocker.value) return
  uploading.value = true
  error.value = ''
  try {
    const form = new FormData()
    form.set('title', upTitle.value)
    form.set('transcript', upTranscript.value)
    // Optional. Without it the material is read by the device.
    if (upFile.value) form.set('audio', upFile.value)
    const res = await api.listenUpload(form)
    upTitle.value = ''
    upTranscript.value = ''
    upFile.value = null
    if (fileInput.value) fileInput.value.value = ''
    await load()
    await open(res.id)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    uploading.value = false
  }
}

function onFile(e: Event) {
  upFile.value = (e.target as HTMLInputElement).files?.[0] ?? null
}

// ── playback ──────────────────────────────────────────────────────────────

/** True while sound is actually coming out, from either source. */
const playing = computed(() =>
  hasRecording.value ? !!player.value && !player.value.paused : speakingAll.value,
)

function speakCurrent() {
  const line = sentences.value[spokenIndex.value]
  if (!line) {
    speakingAll.value = false
    return
  }
  const started = speech.speak(line, speed.value)
  if (!started) {
    speakingAll.value = false
    error.value = ''
    return
  }
  lastTick = performance.now()
}

/**
 * Read the transcript aloud, sentence by sentence.
 *
 * Chained on the end of each utterance rather than queued in one go, so pausing
 * lands on a sentence boundary and the A/B repeat below can be a sentence
 * repeat, which is what shadowing actually needs. [R13]
 */
function toggleSpeaking() {
  if (speakingAll.value) {
    stopSpeaking()
    return
  }
  if (!speech.usable) return
  speakingAll.value = true
  speakNextChain()
}

function speakNextChain() {
  const line = sentences.value[spokenIndex.value]
  if (!line || !speakingAll.value) {
    speakingAll.value = false
    return
  }
  lastTick = performance.now()
  const ok = speech.speak(line, speed.value)
  if (!ok) {
    speakingAll.value = false
    return
  }
  // The store owns the utterance; poll its `speaking` flag to chain, which
  // avoids reaching into the engine from two places.
  const wait = window.setInterval(() => {
    if (!speakingAll.value) {
      window.clearInterval(wait)
      return
    }
    if (!speech.speaking) {
      window.clearInterval(wait)
      if (spokenIndex.value < sentences.value.length - 1) {
        spokenIndex.value += 1
        speakNextChain()
      } else {
        speakingAll.value = false
        void report()
      }
    }
  }, 120)
}

function stopSpeaking() {
  speakingAll.value = false
  speech.cancel()
  lastTick = 0
}

function repeatSentence() {
  if (hasRecording.value) return
  stopSpeaking()
  speakCurrent()
}

function skipSentence(delta: number) {
  spokenIndex.value = Math.min(
    Math.max(0, spokenIndex.value + delta),
    Math.max(0, sentences.value.length - 1),
  )
  if (speakingAll.value) {
    speech.cancel()
    speakNextChain()
  }
}

function tick() {
  if (!playing.value) return
  const now = performance.now()
  if (lastTick > 0) heardAccum += (now - lastTick) / 1000
  lastTick = now
  const p = player.value
  if (!p) return
  // A/B loop, recordings only: with device speech the unit is the sentence.
  if (loopA.value !== null && loopB.value !== null && p.currentTime >= loopB.value) {
    p.currentTime = loopA.value
  }
}

async function report() {
  const p = player.value
  const d = detail.value
  const l = listen.value
  if (!d || !l) return
  // Under a second is noise rather than listening, but the report still goes:
  // position and pass have to be persisted so a reload resumes where the
  // learner left off. A blip on the way is dropped — playback is not the place
  // to surface a network error, and the next report carries the position again.
  const heard = Math.min(60, heardAccum)
  heardAccum = 0
  await api
    .listenProgress(d.text.id, {
      pass: l.pass,
      // Seconds, and only meaningful for a recording. A device-speech material
      // has no timeline to store, so it reports zero rather than smuggling a
      // sentence index into a field documented as seconds.
      positionS: hasRecording.value ? (p?.currentTime ?? 0) : 0,
      secondsHeard: heard < 1 ? 0 : heard,
    })
    .catch(() => {})
}

function setPass(p: 1 | 2 | 3) {
  if (!listen.value) return
  listen.value.pass = p
  void report()
}

function setLoopPoint() {
  const p = player.value
  if (!p) return
  if (loopA.value === null) {
    loopA.value = p.currentTime
  } else if (loopB.value === null && p.currentTime > loopA.value) {
    loopB.value = p.currentTime
  } else {
    loopA.value = null
    loopB.value = null
  }
}

watch(speed, (s) => {
  const p = player.value
  if (!p) return
  p.playbackRate = s
  // Half-speed listening that squeaks is useless: pitch must survive.
  p.preservesPitch = true
})

watch(
  () => detail.value?.text.id,
  async () => {
    const p = player.value
    const l = listen.value
    if (p && l && l.positionS > 0) p.currentTime = l.positionS
  },
)

function nextDictSentence() {
  dictSentence.value = Math.min(dictSentence.value + 1, sentences.value.length - 1)
  dictTyped.value = ''
  dictChecked.value = false
}

function onPlay() {
  lastTick = performance.now()
}

function onPause() {
  lastTick = 0
  void report()
}

onMounted(() => {
  void load()
  reportTimer = window.setInterval(() => {
    tick()
    // Just under the server's 60 s per-report clamp: flushing more often than
    // that is four POSTs a minute for identical accounting. Nothing is lost by
    // waiting, because pausing, changing pass and unmounting all flush too.
    if (heardAccum >= 50) void report()
  }, 1000)
})
onUnmounted(() => {
  window.clearInterval(reportTimer)
  stopSpeaking()
  void report()
})
</script>

<template>
  <div class="page-narrow flex w-full min-w-0 flex-col gap-6">
    <header class="flex items-center justify-between gap-3">
      <h1 class="page-title">{{ $t('listen.title') }}</h1>
    </header>
    <Note v-if="error" variant="error">{{ error }}</Note>

    <!--
      What this screen trains, before anything plays.

      Unlike the ear trainer this one never auto-played, but `listen.pass1Hint`
      was its only explanation and it appears halfway down, inside pass 1. The
      three-pass protocol is the whole method here and it was documented only
      in the script's docblock — visible to us and to nobody who uses the app.
    -->
    <Note v-if="!detail" variant="info" class="text-xs">
      <p class="mb-1 flex items-center gap-0.5 font-medium">
        {{ $t('listen.intro.title') }}
        <InfoTip>
          <p>{{ $t('listen.intro.passes') }}</p>
          <p class="mt-2">{{ $t('listen.intro.caveat') }}</p>
        </InfoTip>
      </p>
      <p>{{ $t('listen.intro.what') }}</p>
    </Note>

    <!-- ── player ──────────────────────────────────────────────────────── -->
    <template v-if="detail && listen">
      <Card variant="raised" class="animate-card-in">
        <CardHeader class="flex flex-row items-start justify-between gap-3 pb-4">
          <div class="flex flex-col gap-1">
            <CardTitle class="text-lg">{{ detail.text.title }}</CardTitle>
            <span class="flex items-center gap-1.5 text-xs text-muted-foreground">
              <component
                :is="hasRecording ? Headphones : Speech"
                class="size-3.5"
                aria-hidden="true"
              />
              {{ hasRecording ? $t('listen.sourceRecording') : $t('listen.sourceDevice') }}
            </span>
          </div>
          <Button variant="ghost" size="sm" @click="close">{{ $t('common.back') }}</Button>
        </CardHeader>
        <CardContent class="flex flex-col gap-4 pb-6">
          <!--
            Coverage pre-check: an honest warning, not a block. [R6] The
            thresholds are English-derived and the message says so — which is
            information about the measurement, not a defect in the text.
          -->
          <Note v-if="coverageWarning" variant="limit" class="text-xs">
            {{
              $t('listen.coverageWarn', {
                pct: Math.round(detail.coverage.elementCoverage * 100),
              })
            }}
          </Note>

          <!-- Synthetic speech is not a native recording, and the difference
               matters for a language whose vowel length carries meaning. Say it
               once, here, rather than letting the two look interchangeable. -->
          <Note v-if="!hasRecording" variant="info" class="text-xs">
            {{ $t('listen.deviceVoiceCaveat') }}
          </Note>

          <!-- The three passes, the learner's own claim. -->
          <div class="flex flex-wrap items-center gap-1.5">
            <button
              v-for="p in [1, 2, 3] as const"
              :key="p"
              type="button"
              class="min-h-9 cursor-pointer rounded-full border px-3 py-1 text-xs transition-colors"
              :class="
                listen.pass === p
                  ? 'border-tint-mist-edge bg-tint-mist font-medium text-foreground'
                  : 'border-border text-muted-foreground hover:bg-tint-mist/60 hover:text-foreground'
              "
              :aria-pressed="listen.pass === p"
              @click="setPass(p)"
            >
              {{ $t(`listen.pass${p}`) }}
            </button>
            <span class="ml-auto font-mono text-xs tabular-nums text-muted-foreground">
              {{ $t('listen.heard', { min: Math.round(listen.listenedS / 60) }) }}
            </span>
          </div>

          <!-- A recording plays through the element; everything else is read. -->
          <audio
            v-if="hasRecording"
            ref="player"
            :src="'/media/' + detail.text.audioPath"
            controls
            class="w-full"
            @play="onPlay"
            @pause="onPause"
          />

          <div v-else class="flex flex-col gap-2">
            <div class="flex flex-wrap items-center gap-2">
              <Button :disabled="!speech.usable" @click="toggleSpeaking">
                <Speech class="size-4" aria-hidden="true" />
                {{ speakingAll ? $t('listen.stopReading') : $t('listen.startReading') }}
              </Button>
              <Button
                variant="outline"
                size="sm"
                :disabled="!speech.usable"
                @click="repeatSentence"
              >
                <Repeat class="size-3.5" aria-hidden="true" />
                {{ $t('listen.repeatSentence') }}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                :disabled="!speech.usable || spokenIndex >= sentences.length - 1"
                @click="skipSentence(1)"
              >
                <SkipForward class="size-3.5" aria-hidden="true" />
                {{ $t('listen.nextSentence') }}
              </Button>
              <span class="font-mono text-xs tabular-nums text-muted-foreground">
                {{ Math.min(spokenIndex + 1, sentences.length) }} / {{ sentences.length }}
              </span>
            </div>
            <p v-if="!speech.usable" class="text-xs leading-relaxed text-uncertain">
              {{ speech.supported ? $t('speech.noVoiceHint') : $t('speech.unsupportedHint') }}
            </p>
          </div>

          <div class="flex flex-wrap items-center gap-3">
            <label class="flex items-center gap-2 text-xs text-muted-foreground">
              {{ $t('listen.speed') }}
              <input
                v-model.number="speed"
                type="range"
                min="0.5"
                max="1.5"
                step="0.05"
                class="w-28 accent-primary"
              />
              <span class="font-mono tabular-nums">{{ speed.toFixed(2) }}×</span>
            </label>
            <template v-if="hasRecording">
              <Button size="sm" variant="outline" @click="setLoopPoint">
                <Repeat class="size-3.5" aria-hidden="true" />
                {{
                  loopA === null
                    ? $t('listen.loopSetA')
                    : loopB === null
                      ? $t('listen.loopSetB')
                      : $t('listen.loopClear')
                }}
              </Button>
              <span
                v-if="loopA !== null && loopB !== null"
                class="font-mono text-xs tabular-nums text-muted-foreground"
              >
                {{ loopA.toFixed(1) }}s – {{ loopB.toFixed(1) }}s
              </span>
            </template>
          </div>

          <!-- Pass 1 hides the transcript on purpose; that IS the pass. -->
          <div
            v-if="listen.pass >= 2"
            class="max-h-64 overflow-y-auto rounded-lg border border-border-subtle bg-background p-4 text-sm leading-relaxed whitespace-pre-line"
            lang="hu"
          >
            <!-- On a read-aloud material the current sentence is marked, so the
                 form–sound mapping pass 2 exists for actually lands on the
                 right words. -->
            <template v-if="!hasRecording">
              <span
                v-for="(line, i) in sentences"
                :key="i"
                class="rounded px-0.5"
                :class="i === spokenIndex ? 'bg-primary/15 text-foreground' : ''"
                >{{ line }}&#32;</span
              >
            </template>
            <template v-else>{{ detail.text.body }}</template>
          </div>
          <Note v-else variant="info" class="text-xs">{{ $t('listen.pass1Hint') }}</Note>

          <!-- Pass 3: dictation with a character diff. -->
          <div
            v-if="listen.pass === 3 && sentences.length"
            class="flex flex-col gap-2 rounded-lg border border-border-subtle bg-background p-4"
          >
            <p class="text-xs tabular-nums text-muted-foreground">
              {{ $t('listen.dictation', { n: dictSentence + 1, of: sentences.length }) }}
            </p>
            <form class="flex items-center gap-2" @submit.prevent="dictChecked = true">
              <Input
                v-model="dictTyped"
                lang="hu"
                autocomplete="off"
                spellcheck="false"
                :placeholder="$t('listen.dictationPlaceholder')"
                class="font-mono"
              />
              <Button type="submit" size="sm">{{ $t('review.check') }}</Button>
            </form>
            <p v-if="dictChecked" class="font-mono text-sm break-all" lang="hu">
              <span
                v-for="(d, i) in dictDiff"
                :key="i"
                :class="d.ok ? 'text-known' : 'text-error underline decoration-error'"
                >{{ d.ch }}</span
              >
            </p>
            <div v-if="dictChecked" class="flex gap-2">
              <Button size="sm" variant="secondary" @click="nextDictSentence">
                {{ $t('review.next') }}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </template>

    <!-- ── the library + add ───────────────────────────────────────────── -->
    <template v-else>
      <p v-if="loading" class="text-sm text-muted-foreground">{{ $t('common.loading') }}</p>

      <!--
        Every text, not only the ones with a recording attached. A text with no
        audio file is read by the device — which is what makes this list
        non-empty on the day the app is installed.
      -->
      <Card v-else-if="materials.length" variant="raised">
        <CardHeader class="pb-3">
          <CardTitle class="text-base">{{ $t('listen.materials') }}</CardTitle>
        </CardHeader>
        <CardContent class="pb-4">
          <ul class="stagger-in flex flex-col">
            <li v-for="m in materials" :key="m.id">
              <button
                type="button"
                class="flex w-full cursor-pointer items-center justify-between gap-3 rounded-lg px-2.5 py-2.5 text-left text-sm transition-colors hover:bg-tint-mist/60"
                @click="open(m.id)"
              >
                <span class="flex min-w-0 items-center gap-2">
                  <component
                    :is="m.audioPath ? Headphones : Speech"
                    class="size-4 shrink-0 text-muted-foreground"
                    aria-hidden="true"
                  />
                  <span class="truncate">{{ m.title }}</span>
                  <span v-if="!m.audioPath" class="shrink-0 text-xs text-muted-foreground">
                    {{ $t('listen.sourceDevice') }}
                  </span>
                </span>
                <span class="font-mono text-xs tabular-nums text-muted-foreground">
                  {{ Math.round(m.elementCoverage * 100) }}%
                </span>
              </button>
            </li>
          </ul>
        </CardContent>
      </Card>

      <EmptyState v-else art="balaton" :title="$t('listen.empty')" />

      <Card variant="tint-cream">
        <CardHeader class="pb-3">
          <CardTitle class="text-sm">{{ $t('listen.add') }}</CardTitle>
        </CardHeader>
        <CardContent class="flex flex-col gap-3">
          <Input v-model="upTitle" :placeholder="$t('library.title')" />
          <textarea
            v-model="upTranscript"
            :placeholder="$t('listen.transcriptHint')"
            rows="4"
            class="w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            lang="hu"
          />
          <div class="flex flex-col gap-1.5">
            <label class="text-xs text-muted-foreground" for="listen-audio">
              {{ $t('listen.audioOptional') }}
            </label>
            <input
              id="listen-audio"
              ref="fileInput"
              type="file"
              accept="audio/*"
              class="text-xs text-muted-foreground"
              @change="onFile"
            />
          </div>
          <div class="flex flex-wrap items-center gap-3">
            <Button :disabled="uploading || !!uploadBlocker" @click="upload">
              <Upload class="size-4" aria-hidden="true" />
              {{ uploading ? $t('common.saving') : $t('listen.upload') }}
            </Button>
            <!-- The reason the button is inert, beside the button. A control
                 that is disabled without saying why is a broken control. -->
            <span v-if="uploadBlocker" class="text-xs text-muted-foreground">
              {{ $t(uploadBlocker) }}
            </span>
          </div>
          <p class="text-xs leading-relaxed text-muted-foreground">
            {{ $t('listen.whyTranscript') }}
          </p>
        </CardContent>
      </Card>
    </template>
  </div>
</template>
