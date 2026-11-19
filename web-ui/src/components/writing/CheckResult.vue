<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Copy, Square, Volume2 } from '@lucide/vue'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import Note from '@/components/common/Note.vue'
import VoicePicker from '@/components/reader/VoicePicker.vue'
import ErrorCard from './ErrorCard.vue'
import { useUiStore } from '@/stores/ui'
import { useTtsStore, type PlayItem } from '@/stores/tts'
import { useSpeechStore } from '@/stores/speech'
import { chunkForTts } from '@/lib/tts'
import { copyText, readShowAll, selectAll, shortDate, writeShowAll } from '@/lib/writing'
import type { WritingCheck } from '@/types'

/**
 * One check, in full (docs/UX.md §7.4). The checked text is drawn from
 * `segments` by interpolation — a model's or a learner's `<img onerror>` stays
 * literal text — and the browser never touches an offset. An error is marked by
 * a red wavy underline, a tint AND a superscript number: colour is not the only
 * signal. Error numbers on the wire are 1-based, segment indices 0-based.
 */
const props = defineProps<{ check: WritingCheck }>()
const emit = defineEmits<{ external: [] }>()
const { t } = useI18n()
const ui = useUiStore()
const tts = useTtsStore()
const speech = useSpeechStore()

// The corrected text, read aloud by the same neural voices as the reader
// (Piper through the server; the device voice when the engine is off).
const playItems = computed<PlayItem[]>(() => {
  const out: PlayItem[] = []
  let idx = 0
  for (const line of (props.check.corrected || '').split('\n')) {
    for (const sent of line.match(/[^.!?]+[.!?]*/g) ?? []) {
      if (!/\p{L}/u.test(sent)) continue
      for (const piece of chunkForTts(sent)) out.push({ sentIdx: idx, text: piece })
      idx++
    }
  }
  return out
})
const listening = computed(() => tts.status !== 'idle')
async function listen() {
  if (listening.value) return tts.stop()
  const ok = await tts.play(playItems.value)
  if (!ok) toast.error(t(speech.supported ? 'speech.noVoice' : 'speech.unsupported'))
}
onMounted(() => void tts.load())
onBeforeUnmount(() => tts.stop())

const showAll = ref(readShowAll())
function toggleAll() {
  showAll.value = !showAll.value
  writeShowAll(showAll.value)
}

const sourceLabel = computed(() =>
  props.check.source === 'local' ? t('writing.result.sourceLocal') : t('writing.result.sourceExternal'),
)

/** The error number is printed once, on the first segment that carries it. */
const firstSegment = computed(() => {
  const seen = new Set<number>()
  const out: number[][] = []
  for (const seg of props.check.segments) {
    const fresh: number[] = []
    for (const i of seg.errors) {
      if (!seen.has(i)) {
        seen.add(i)
        fresh.push(i)
      }
    }
    out.push(fresh)
  }
  return out
})

const active = ref(0)
async function jump(errIndex: number) {
  const err = props.check.errors[errIndex]
  if (!err) return
  active.value = err.n
  await nextTick()
  const el = document.getElementById(`werr-${err.n}`)
  el?.scrollIntoView({ block: 'center', behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' })
  el?.focus({ preventScroll: true })
}

const hasRawBanner = computed(() => props.check.status === 'partial' || props.check.status === 'failed')
const showRaw = ref(false)

const copied = ref<'' | 'ok' | 'fallback'>('')
const correctedEl = ref<HTMLElement | null>(null)
async function copyCorrected() {
  if (await copyText(props.check.corrected)) {
    copied.value = 'ok'
    window.setTimeout(() => (copied.value = ''), 2000)
  } else {
    copied.value = 'fallback'
    selectAll(correctedEl.value)
  }
}
</script>

<template>
  <section class="flex flex-col gap-4" :aria-label="t('writing.result.header', { model: check.model })">
    <header class="flex flex-col gap-1">
      <h2 class="font-display text-lg font-medium">
        {{ t('writing.result.header', { model: check.model }) }}
        <span class="font-sans text-sm font-normal text-muted-foreground">· {{ sourceLabel }}</span>
      </h2>
      <p class="text-xs text-muted-foreground">{{ shortDate(check.createdAt, ui.locale) }}</p>
      <p class="text-xs text-muted-foreground">{{ t('writing.result.standing') }}</p>
    </header>

    <Note v-if="check.interrupted" variant="limit">{{ t('writing.result.interrupted') }}</Note>
    <Note v-else-if="check.truncated" variant="limit">{{ t('writing.result.truncated') }}</Note>

    <Note v-if="check.status === 'failed'" variant="error" :title="check.source === 'local' ? t('writing.check.failedFormat') : t('writing.result.failedTitle')">
      <span v-if="check.source === 'local'">{{ t('writing.check.failedFormatBody') }}</span>
      <template v-if="check.source === 'local'" #action>
        <div class="mt-1">
          <Button size="sm" variant="outline" @click="emit('external')">{{ t('writing.check.toExternal') }}</Button>
        </div>
      </template>
    </Note>
    <Note v-else-if="check.status === 'partial'" variant="limit">
      {{ t('writing.result.partial') }}
      <span v-if="check.dropped > 0">({{ t('writing.result.dropped', { n: check.dropped }) }})</span>
    </Note>
    <div v-if="hasRawBanner && check.raw">
      <button
        type="button"
        class="inline-flex min-h-9 cursor-pointer items-center text-sm font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-ring"
        :aria-expanded="showRaw"
        @click="showRaw = !showRaw"
      >
        {{ showRaw ? t('writing.result.hideRaw') : t('writing.result.showRaw') }}
      </button>
      <pre
        v-if="showRaw"
        class="mt-2 max-h-96 overflow-auto rounded-lg border border-border-subtle bg-background p-3 text-xs leading-relaxed whitespace-pre-wrap break-words"
      >{{ check.raw }}</pre>
    </div>

    <p v-if="check.level" class="text-sm text-muted-foreground">{{ t('writing.result.level', { level: check.level }) }}</p>

    <div v-if="check.status !== 'failed'" class="grid gap-5 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)] lg:items-start">
      <div class="flex min-w-0 flex-col gap-2 lg:sticky lg:top-4 lg:max-h-[calc(100dvh-6rem)] lg:overflow-y-auto">
        <h3 class="text-sm font-medium">{{ t('writing.result.textTitle') }}</h3>
        <p v-if="check.errors.length" class="text-xs text-muted-foreground">{{ t('writing.result.textHint') }}</p>
        <p
          lang="hu"
          class="rounded-xl border border-border-subtle bg-card p-4 text-base leading-8 whitespace-pre-wrap break-words"
        ><template v-for="(seg, i) in check.segments" :key="i"><span
            v-if="seg.errors.length"
            role="button"
            tabindex="0"
            :aria-label="t('writing.card.jump', { n: check.errors[seg.errors[0]!]?.n ?? seg.errors[0]! + 1 })"
            class="cursor-pointer rounded-sm bg-error/10 underline decoration-error decoration-wavy decoration-2 underline-offset-4 hover:bg-error/20 focus-visible:outline-2 focus-visible:outline-ring"
            :class="active === (check.errors[seg.errors[0]!]?.n ?? 0) ? 'bg-error/25' : ''"
            @click="jump(seg.errors[0]!)"
            @keydown.enter.prevent="jump(seg.errors[0]!)"
            @keydown.space.prevent="jump(seg.errors[0]!)"
          >{{ seg.text }}</span><template v-else>{{ seg.text }}</template><sup
            v-for="idx in firstSegment[i]"
            :key="idx"
            class="ml-px mr-0.5 font-sans text-[11px] font-semibold text-error"
            aria-hidden="true"
          >{{ check.errors[idx]?.n ?? idx + 1 }}</sup></template></p>
      </div>

      <div class="flex min-w-0 flex-col gap-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h3 class="text-sm font-medium">{{ t('writing.result.errorsTitle', { n: check.errors.length }) }}</h3>
          <button
            v-if="check.errors.length"
            type="button"
            role="switch"
            :aria-checked="showAll"
            class="inline-flex min-h-9 cursor-pointer items-center gap-2 text-sm focus-visible:outline-2 focus-visible:outline-ring"
            @click="toggleAll"
          >
            <span
              class="relative h-5 w-9 rounded-full border border-input transition-colors"
              :class="showAll ? 'bg-primary' : 'bg-secondary'"
              aria-hidden="true"
            >
              <span
                class="absolute top-0.5 size-3.5 rounded-full bg-background shadow transition-all"
                :class="showAll ? 'left-[1.1rem]' : 'left-0.5 bg-muted-foreground'"
              />
            </span>
            {{ t('writing.result.showAll') }}
          </button>
        </div>
        <p v-if="!check.errors.length && check.status === 'ok'" class="text-sm text-muted-foreground">
          {{ t('writing.result.noErrors') }}
        </p>
        <ErrorCard v-for="e in check.errors" :key="e.n" :error="e" :show-all="showAll" :active="active === e.n" />
      </div>
    </div>

    <div v-if="check.summary" class="flex flex-col gap-1">
      <h3 class="text-sm font-medium">{{ t('writing.result.summary') }}</h3>
      <p class="text-sm leading-relaxed whitespace-pre-line">{{ check.summary }}</p>
    </div>
    <div v-if="check.strengths.length" class="flex flex-col gap-1">
      <h3 class="text-sm font-medium">{{ t('writing.result.strengths') }}</h3>
      <ul class="list-disc space-y-0.5 pl-5 text-sm leading-relaxed">
        <li v-for="(s, i) in check.strengths" :key="i">{{ s }}</li>
      </ul>
    </div>
    <div v-if="check.vocab.length" class="flex flex-col gap-1">
      <h3 class="text-sm font-medium">{{ t('writing.result.vocab') }}</h3>
      <ul class="list-disc space-y-0.5 pl-5 text-sm leading-relaxed">
        <li v-for="(v, i) in check.vocab" :key="i">{{ v }}</li>
      </ul>
    </div>
    <div v-if="check.corrected" class="flex flex-col gap-2">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h3 class="text-sm font-medium">{{ t('writing.result.corrected') }}</h3>
        <div class="flex flex-wrap items-center gap-2">
          <Button size="sm" variant="outline" :disabled="!playItems.length" @click="listen">
            <Square v-if="listening" class="size-4" aria-hidden="true" />
            <Volume2 v-else class="size-4" aria-hidden="true" />
            {{ listening ? t('writing.result.listenStop') : t('writing.result.listen') }}
          </Button>
          <Button v-if="tts.piperReady" size="sm" variant="ghost" @click="tts.openPicker()">
            {{ tts.voiceName || t('writing.result.voice') }}
          </Button>
          <Button size="sm" variant="outline" @click="copyCorrected">
            <Copy class="size-4" aria-hidden="true" />
            {{ copied === 'ok' ? t('writing.result.copied') : t('writing.result.copy') }}
          </Button>
        </div>
      </div>
      <p
        ref="correctedEl"
        lang="hu"
        class="rounded-xl border border-border-subtle bg-card p-4 text-base leading-7 whitespace-pre-wrap break-words"
      >{{ check.corrected }}</p>
      <p v-if="copied === 'fallback'" class="text-xs text-muted-foreground" role="status">{{ t('writing.result.copyFallback') }}</p>
    </div>
    <VoicePicker />
  </section>
</template>
