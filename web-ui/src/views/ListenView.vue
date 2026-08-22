<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Headphones, Repeat, Upload } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import Note from '@/components/common/Note.vue'
import { api } from '@/api'
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
 */
const materials = ref<TextSummary[]>([])
const detail = ref<TextDetail | null>(null)
const listen = ref<ListenState | null>(null)
const error = ref('')
const uploading = ref(false)

const upTitle = ref('')
const upTranscript = ref('')
const upFile = ref<File | null>(null)

const player = ref<HTMLAudioElement | null>(null)
const speed = ref(1.0)
const loopA = ref<number | null>(null)
const loopB = ref<number | null>(null)

// Dictation (pass 3).
const dictSentence = ref(0)
const dictTyped = ref('')
const dictChecked = ref(false)

let heardAccum = 0
let lastTick = 0
let reportTimer: number | undefined

const withAudio = computed(() => materials.value.filter((m) => m.audioPath))

/** The coverage pre-check: below ~90% element coverage, warn before playback. */
const coverageWarning = computed(() => {
  const c = detail.value?.coverage
  return !!c && c.elementCoverage < 0.9
})

/** Sentences reconstructed from stored tokens, for pass-3 dictation. */
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

async function load() {
  try {
    const res = await api.texts()
    materials.value = res.texts
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function open(id: number) {
  error.value = ''
  try {
    detail.value = await api.text(id)
    listen.value = await api.listenState(id)
    dictSentence.value = 0
    dictTyped.value = ''
    dictChecked.value = false
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function upload() {
  if (!upFile.value || !upTranscript.value.trim()) return
  uploading.value = true
  error.value = ''
  try {
    const form = new FormData()
    form.set('title', upTitle.value)
    form.set('transcript', upTranscript.value)
    form.set('audio', upFile.value)
    const res = await api.listenUpload(form)
    upTitle.value = ''
    upTranscript.value = ''
    upFile.value = null
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

function tick() {
  const p = player.value
  if (!p || p.paused) return
  const now = performance.now()
  if (lastTick > 0) heardAccum += (now - lastTick) / 1000
  lastTick = now
  // A/B loop.
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
      positionS: p?.currentTime ?? 0,
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
  void report()
})
</script>

<template>
  <div class="mx-auto flex w-full max-w-3xl flex-col gap-6">
    <header class="flex items-center justify-between gap-3">
      <h1 class="text-3xl font-semibold tracking-tight">{{ $t('listen.title') }}</h1>
    </header>
    <Note v-if="error" variant="error">{{ error }}</Note>

    <!-- ── player ──────────────────────────────────────────────────────── -->
    <template v-if="detail && listen">
      <Card variant="raised">
        <CardHeader class="flex flex-row items-start justify-between gap-3 pb-4">
          <CardTitle class="text-lg">{{ detail.text.title }}</CardTitle>
          <Button variant="ghost" size="sm" @click="detail = null">{{ $t('common.back') }}</Button>
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

          <!-- The three passes, the learner's own claim. -->
          <div class="flex flex-wrap items-center gap-1.5">
            <button
              v-for="p in [1, 2, 3] as const"
              :key="p"
              type="button"
              class="cursor-pointer rounded-md border px-2.5 py-1 text-xs transition-colors"
              :class="
                listen.pass === p
                  ? 'border-primary bg-primary/10 font-medium text-primary'
                  : 'border-border text-muted-foreground hover:bg-accent hover:text-foreground'
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

          <audio
            ref="player"
            :src="'/media/' + detail.text.audioPath"
            controls
            class="w-full"
            @play="onPlay"
            @pause="onPause"
          />

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
          </div>

          <!-- Pass 1 hides the transcript on purpose; that IS the pass. -->
          <div
            v-if="listen.pass >= 2"
            class="max-h-64 overflow-y-auto rounded-lg border border-border-subtle bg-background p-4 text-sm leading-relaxed"
            lang="hu"
          >
            {{ detail.text.body }}
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

    <!-- ── the library + upload ────────────────────────────────────────── -->
    <template v-else>
      <Card v-if="withAudio.length" variant="raised">
        <CardHeader class="pb-3">
          <CardTitle class="text-base">{{ $t('listen.materials') }}</CardTitle>
        </CardHeader>
        <CardContent class="pb-4">
          <ul class="stagger-in flex flex-col">
            <li v-for="m in withAudio" :key="m.id">
              <button
                type="button"
                class="flex w-full cursor-pointer items-center justify-between gap-3 rounded-md px-2.5 py-2.5 text-left text-sm transition-colors hover:bg-accent"
                @click="open(m.id)"
              >
                <span class="flex items-center gap-2">
                  <Headphones class="size-4 text-muted-foreground" aria-hidden="true" />
                  {{ m.title }}
                </span>
                <span class="font-mono text-xs tabular-nums text-muted-foreground">
                  {{ Math.round(m.elementCoverage * 100) }}%
                </span>
              </button>
            </li>
          </ul>
        </CardContent>
      </Card>

      <Note v-else variant="info">{{ $t('listen.empty') }}</Note>

      <Card variant="quiet">
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
          <div class="flex flex-wrap items-center gap-3">
            <input type="file" accept="audio/*" class="text-xs text-muted-foreground" @change="onFile" />
            <Button :disabled="uploading || !upFile || !upTranscript.trim()" @click="upload">
              <Upload class="size-4" aria-hidden="true" />
              {{ uploading ? $t('common.saving') : $t('listen.upload') }}
            </Button>
          </div>
          <p class="text-xs leading-relaxed text-muted-foreground">
            {{ $t('listen.whyTranscript') }}
          </p>
        </CardContent>
      </Card>
    </template>
  </div>
</template>
