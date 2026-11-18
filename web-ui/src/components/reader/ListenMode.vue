<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Headphones, Repeat, SkipForward, Speech } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import InfoTip from '@/components/common/InfoTip.vue'
import Note from '@/components/common/Note.vue'
import { api } from '@/api'
import { describeError } from '@/lib/errors'
import { RATES } from '@/lib/tts'
import { useSpeechStore } from '@/stores/speech'
import { useTtsStore, type PlayItem } from '@/stores/tts'
import type { ListenState, TextDetail } from '@/types'

/**
 * Listening mode of the reader: the three-pass protocol as UI state. [R13]
 *
 *   pass 1 — gist, transcript hidden. Comprehension without a crutch.
 *   pass 2 — with the transcript (the reader's own text, current sentence tinted).
 *   pass 3 — dictation. Production from the ear.
 *
 * The pass is the learner's own claim, persisted server-side; the app never
 * advances it. Seconds actually heard are reported with the REAL pass (1|2|3)
 * and feed the listening-hours gate on word-order topics — hours accumulate,
 * they are never claimed in a lump. This component is the only reporter while
 * the mode is on: the reader's plain-playback reporter (pass 0) and its
 * read_s clock stand down (ReaderView), so one second is counted once.
 *
 * Two sources of sound. A text with an uploaded recording plays through the
 * audio element (resume position, A/B loop). Any other text is read by the
 * shared sentence engine (`stores/tts.ts`: Piper when it is there, the device
 * voice when not), sentence by sentence, so Repeat and Next are sentence
 * repeats — which is what shadowing needs. Synthetic speech is worse input than
 * a native recording and the card says so.
 */
const props = defineProps<{
  detail: TextDetail
  /** The reader's utterances: one per sentence, in order. */
  items: PlayItem[]
}>()
const emit = defineEmits<{
  'update:pass': [pass: number]
  /** A recording is sounding: the reader stops its read_s clock meanwhile. */
  'update:recPlaying': [playing: boolean]
}>()

const { t } = useI18n()
const tts = useTtsStore()
const speech = useSpeechStore()

const listen = ref<ListenState | null>(null)
const error = ref('')
const player = ref<HTMLAudioElement | null>(null)
const speed = ref(tts.rate)
const loopA = ref<number | null>(null)
const loopB = ref<number | null>(null)
/** The sentence the chain is on (index into `items`). */
const idx = ref(0)

// Dictation (pass 3).
const dictSentence = ref(0)
const dictTyped = ref('')
const dictChecked = ref(false)

let heardAccum = 0
let lastTick = 0
let reportTimer: number | undefined
let resumed = false

const hasRecording = computed(() => Boolean(props.detail.text.audioPath))
const coverageWarning = computed(() => props.detail.coverage.elementCoverage < 0.9)
const canSpeak = computed(() => tts.piperReady || speech.usable)
const speaking = computed(() => tts.status !== 'idle')

/**
 * Sentences reconstructed from stored tokens, for dictation. The offsets are
 * BYTE offsets into UTF-8, so the slice happens in bytes and is decoded back.
 */
const sentences = computed(() => {
  const d = props.detail
  const bySent = new Map<number, { start: number; end: number }>()
  for (const tk of d.tokens) {
    const cur = bySent.get(tk.sentIdx)
    if (!cur) bySent.set(tk.sentIdx, { start: tk.start, end: tk.end })
    else cur.end = Math.max(cur.end, tk.end)
  }
  const enc = new TextEncoder().encode(d.text.body)
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
  const tg = [...target.toLowerCase()]
  const y = [...dictTyped.value.toLowerCase()]
  const out: { ch: string; ok: boolean }[] = []
  for (let i = 0; i < Math.max(tg.length, y.length); i++) {
    out.push({ ch: [...target][i] ?? '·', ok: tg[i] === y[i] })
  }
  return out
})

async function loadState() {
  try {
    listen.value = await api.listenState(props.detail.text.id)
    emit('update:pass', listen.value.pass)
    applyResume()
  } catch (e) {
    error.value = describeError(e)
  }
}

/** The element exists only once the state has loaded: give it the chosen speed, then resume. */
function onMetadata() {
  const p = player.value
  if (p) {
    p.playbackRate = speed.value
    p.preservesPitch = true
  }
  applyResume()
}

/** Resume a recording where the learner left it, once both the state and the file are known. */
function applyResume() {
  const p = player.value
  const l = listen.value
  if (resumed || !p || !l || p.readyState < 1) return
  resumed = true
  if (l.positionS > 0) p.currentTime = l.positionS
}

// ── playback ──────────────────────────────────────────────────────────────

/** The chain: from `from`, sentence after sentence, ending on its own. */
async function startFrom(from: number) {
  const ok = await tts.play(props.items, from)
  if (!ok) error.value = t(speech.supported ? 'speech.noVoice' : 'speech.unsupported')
}

function toggleReading() {
  if (speaking.value) tts.stop()
  else void startFrom(idx.value)
}

function repeatSentence() {
  if (hasRecording.value) return
  void startFrom(idx.value)
}

function skipSentence() {
  idx.value = Math.min(idx.value + 1, Math.max(0, props.items.length - 1))
  if (speaking.value) void startFrom(idx.value)
}

// The chain moves the counter, whoever started it.
watch(
  () => tts.currentSent,
  (s) => {
    if (s === null) return
    const i = props.items.findIndex((it) => it.sentIdx === s)
    if (i >= 0) idx.value = i
  },
)

const sounding = () =>
  hasRecording.value
    ? !!player.value && !player.value.paused && !player.value.ended
    : tts.audible

function tick() {
  if (!sounding()) {
    lastTick = 0
    return
  }
  const now = performance.now()
  if (lastTick > 0) heardAccum += (now - lastTick) / 1000
  lastTick = now
}

async function report() {
  const l = listen.value
  if (!l) return
  // Under a second is noise rather than listening, but the report still goes:
  // position and pass have to be persisted so a reload resumes where the
  // learner left off. Playback is not the place to surface a network error.
  const heard = Math.min(60, heardAccum)
  // Under a second stays banked for the next report instead of being dropped.
  if (heard >= 1) heardAccum -= heard
  const pos = hasRecording.value ? (player.value?.currentTime ?? 0) : 0
  await api
    .listenProgress(props.detail.text.id, {
      pass: l.pass,
      positionS: pos,
      secondsHeard: heard < 1 ? 0 : heard,
    })
    .catch(() => {})
}

function setPass(p: 1 | 2 | 3) {
  const l = listen.value
  if (!l || l.pass === p) return
  void report() // what was heard so far belongs to the pass it was heard in
  l.pass = p
  emit('update:pass', p)
  void report()
}

function setLoopPoint() {
  const p = player.value
  if (!p) return
  if (loopA.value === null) loopA.value = p.currentTime
  else if (loopB.value === null && p.currentTime > loopA.value) loopB.value = p.currentTime
  else {
    loopA.value = null
    loopB.value = null
  }
}

function onTimeUpdate() {
  const p = player.value
  if (p && loopA.value !== null && loopB.value !== null && p.currentTime >= loopB.value) {
    p.currentTime = loopA.value
  }
}

function applySpeed() {
  tts.setRate(speed.value)
  const p = player.value
  if (!p) return
  p.playbackRate = speed.value
  // Half-speed listening that squeaks is useless: pitch must survive.
  p.preservesPitch = true
}
watch(speed, applySpeed)

function onPlay() {
  lastTick = performance.now()
  emit('update:recPlaying', true)
}
function onPause() {
  tick()
  lastTick = 0
  emit('update:recPlaying', false)
  void report()
}

// Engine playback ended or was paused: bank the seconds under the current pass.
watch(
  () => tts.audible,
  (on) => {
    if (hasRecording.value) return
    if (on) lastTick = performance.now()
    else {
      tick()
      lastTick = 0
      // Buffering between two sentences also drops `audible`; that is not a pause.
      if (tts.status !== 'playing') void report()
    }
  },
)

function nextDictSentence() {
  dictSentence.value = Math.min(dictSentence.value + 1, sentences.value.length - 1)
  dictTyped.value = ''
  dictChecked.value = false
}

function onHidden() {
  if (document.visibilityState !== 'hidden') return
  tick()
  void report()
}

onMounted(() => {
  document.addEventListener('visibilitychange', onHidden)
  window.addEventListener('pagehide', onHidden)
  // Plain playback from the audio bar hands over to this mode's own reporter.
  tts.stop()
  void loadState()
  applySpeed()
  reportTimer = window.setInterval(() => {
    tick()
    // Just under the server's 60 s per-report clamp: flushing more often is
    // four POSTs a minute for identical accounting. Pausing, changing pass and
    // leaving flush too, so nothing is lost by waiting.
    if (heardAccum >= 50) void report()
  }, 1000)
})

onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', onHidden)
  window.removeEventListener('pagehide', onHidden)
  window.clearInterval(reportTimer)
  tick()
  void report()
  tts.stop()
  // The bar's menu only knows whole steps; leave it on one.
  const near = RATES.reduce((a, b) => (Math.abs(b - speed.value) < Math.abs(a - speed.value) ? b : a))
  tts.setRate(near)
  emit('update:recPlaying', false)
})
</script>

<template>
  <div class="flex flex-col gap-3">
    <Note v-if="error" variant="error">{{ error }}</Note>
    <p v-if="!listen && !error" class="text-sm text-muted-foreground">{{ $t('common.loading') }}</p>

    <Card v-if="listen" variant="raised" class="animate-card-in">
      <CardHeader class="flex flex-row items-start justify-between gap-3 pb-3">
        <div class="flex flex-col gap-1">
          <CardTitle class="flex items-center gap-0.5 text-base">
            {{ $t('reader.audio.listenMode') }}
            <InfoTip>
              <p>{{ $t('reader.listen.intro.what') }}</p>
              <p class="mt-2">{{ $t('reader.listen.intro.passes') }}</p>
              <p class="mt-2">{{ $t('reader.listen.intro.caveat') }}</p>
            </InfoTip>
          </CardTitle>
          <span class="flex items-center gap-1.5 text-xs text-muted-foreground">
            <component
              :is="hasRecording ? Headphones : Speech"
              class="size-3.5"
              aria-hidden="true"
            />
            {{ hasRecording ? $t('reader.listen.sourceRecording') : $t('reader.listen.sourceSpeech') }}
          </span>
        </div>
      </CardHeader>
      <CardContent class="flex flex-col gap-4 pb-5">
        <!-- Coverage pre-check: an honest warning, not a block. [R6] The
             thresholds are English-derived and the message says so. -->
        <Note v-if="coverageWarning" variant="limit" class="text-xs">
          {{
            $t('reader.listen.coverageWarn', {
              pct: Math.round(detail.coverage.elementCoverage * 100),
            })
          }}
        </Note>

        <!-- Synthetic speech is not a native recording, and the difference
             matters for a language whose vowel length carries meaning. -->
        <Note v-if="!hasRecording" variant="info" class="text-xs">
          {{ $t('reader.listen.synthCaveat') }}
        </Note>

        <!-- The three passes, the learner's own claim. -->
        <div class="flex flex-wrap items-center gap-1.5" role="group">
          <button
            v-for="p in [1, 2, 3] as const"
            :key="p"
            type="button"
            class="min-h-9 cursor-pointer rounded-full border px-3 py-1 text-xs transition-colors focus-visible:outline-2 focus-visible:outline-ring max-md:min-h-11"
            :class="
              listen.pass === p
                ? 'border-tint-mist-edge bg-tint-mist font-medium text-foreground'
                : 'border-border text-muted-foreground hover:bg-tint-mist/60 hover:text-foreground'
            "
            :aria-pressed="listen.pass === p"
            @click="setPass(p)"
          >
            {{ $t(`reader.listen.pass${p}`) }}
          </button>
          <span class="ml-auto font-mono text-xs tabular-nums text-muted-foreground">
            {{ $t('reader.listen.heard', { min: Math.round(listen.listenedS / 60) }) }}
          </span>
        </div>

        <!-- A recording plays through the element; everything else is read. -->
        <audio
          v-if="hasRecording"
          ref="player"
          :src="'/media/' + detail.text.audioPath"
          controls
          class="w-full"
          @loadedmetadata="onMetadata"
          @play="onPlay"
          @pause="onPause"
          @ended="onPause"
          @timeupdate="onTimeUpdate"
        />

        <div v-else class="flex flex-col gap-2">
          <div class="flex flex-wrap items-center gap-2">
            <Button class="max-md:min-h-11" :disabled="!canSpeak" @click="toggleReading">
              <Speech class="size-4" aria-hidden="true" />
              {{ speaking ? $t('reader.listen.stopReading') : $t('reader.listen.startReading') }}
            </Button>
            <Button
              variant="outline"
              size="sm"
              class="max-md:min-h-11"
              :disabled="!canSpeak"
              @click="repeatSentence"
            >
              <Repeat class="size-3.5" aria-hidden="true" />
              {{ $t('reader.listen.repeatSentence') }}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              class="max-md:min-h-11"
              :disabled="!canSpeak || idx >= items.length - 1"
              @click="skipSentence"
            >
              <SkipForward class="size-3.5" aria-hidden="true" />
              {{ $t('reader.listen.nextSentence') }}
            </Button>
            <span class="font-mono text-xs tabular-nums text-muted-foreground">
              {{ Math.min(idx + 1, items.length) }} / {{ items.length }}
            </span>
          </div>
          <p v-if="!canSpeak" class="text-xs leading-relaxed text-uncertain">
            {{ speech.supported ? $t('speech.noVoiceHint') : $t('speech.unsupportedHint') }}
          </p>
        </div>

        <div class="flex flex-wrap items-center gap-3">
          <label class="flex items-center gap-2 text-xs text-muted-foreground">
            {{ $t('reader.listen.speed') }}
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
            <Button size="sm" variant="outline" class="max-md:min-h-11" @click="setLoopPoint">
              <Repeat class="size-3.5" aria-hidden="true" />
              {{
                loopA === null
                  ? $t('reader.listen.loopSetA')
                  : loopB === null
                    ? $t('reader.listen.loopSetB')
                    : $t('reader.listen.loopClear')
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

        <!-- Pass 1 hides the transcript on purpose (the reader's text is hidden
             too); that IS the pass. -->
        <Note v-if="listen.pass === 1" variant="info" class="text-xs">
          {{ $t('reader.listen.pass1Hint') }}
        </Note>

        <!-- Pass 3: dictation with a character diff. -->
        <div
          v-if="listen.pass === 3 && sentences.length"
          class="flex flex-col gap-2 rounded-lg border border-border-subtle bg-background p-4"
        >
          <p class="text-xs tabular-nums text-muted-foreground">
            {{ $t('reader.listen.dictation', { n: dictSentence + 1, of: sentences.length }) }}
          </p>
          <form class="flex items-center gap-2" @submit.prevent="dictChecked = true">
            <Input
              v-model="dictTyped"
              lang="hu"
              autocomplete="off"
              spellcheck="false"
              :placeholder="$t('reader.listen.dictationPlaceholder')"
              class="font-mono"
            />
            <Button type="submit" size="sm" class="max-md:min-h-11">{{ $t('review.check') }}</Button>
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
            <Button size="sm" variant="secondary" class="max-md:min-h-11" @click="nextDictSentence">
              {{ $t('review.next') }}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
