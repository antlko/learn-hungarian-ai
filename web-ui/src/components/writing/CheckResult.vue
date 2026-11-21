<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMediaQuery } from '@vueuse/core'
import { ArrowRight, ChevronDown, Copy, Square, Volume2 } from '@lucide/vue'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import Note from '@/components/common/Note.vue'
import VoicePicker from '@/components/reader/VoicePicker.vue'
import BottomSheet from '@/components/common/BottomSheet.vue'
import MistakeInspector from './MistakeInspector.vue'
import { useUiStore } from '@/stores/ui'
import { useTtsStore, type PlayItem } from '@/stores/tts'
import { useSpeechStore } from '@/stores/speech'
import { chunkForTts } from '@/lib/tts'
import { copyText, selectAll, shortDate } from '@/lib/writing'
import type { WritingCheck } from '@/types'

/**
 * One check, in full (docs/UX.md §7.4). The TEXT is the hero and the mistakes
 * are on demand: a mark is selected, and ONE mistake shows in the inspector
 * (a right column on a wide screen, a bottom sheet on a phone). The checked text
 * is drawn from `segments` by interpolation — a model's or a learner's
 * `<img onerror>` stays literal text — and the browser never touches an offset.
 * A mark is a red wavy underline, a tint AND a superscript number: colour is not
 * the only signal. Error numbers on the wire are 1-based, `errors` and segment
 * indices 0-based; `sel` below is an index into `errors`.
 */
const props = defineProps<{ check: WritingCheck }>()
const emit = defineEmits<{ external: [] }>()
const { t, te } = useI18n()
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


const typeOf = (e: { type: string }) => (te(`writing.type.${e.type}`) ? e.type : 'other')
const typeLabel = (type: string) => t(`writing.type.${type}`)

/** Mistake types in order of first appearance, with counts: the filter chips. */
const typeCounts = computed(() => {
  const m = new Map<string, number>()
  for (const e of props.check.errors) m.set(typeOf(e), (m.get(typeOf(e)) ?? 0) + 1)
  return [...m].map(([type, count]) => ({ type, count }))
})

const isDesktop = useMediaQuery('(min-width: 1024px)')
const tab = ref<'text' | 'corrected'>('text')
const typeFilter = ref<string | null>(null)
/** The selected mistake: an index into `check.errors`. */
const sel = ref<number | null>(null)
let viaMark = false

/** Indices the filter lets through, in order: what Prev / Next walk and the list shows. */
const visible = computed(() => {
  const out: number[] = []
  props.check.errors.forEach((e, i) => {
    if (!typeFilter.value || typeOf(e) === typeFilter.value) out.push(i)
  })
  return out
})
const selErr = computed(() => (sel.value === null ? null : (props.check.errors[sel.value] ?? null)))
const pos = computed(() => (sel.value === null ? 0 : Math.max(visible.value.indexOf(sel.value), 0)))

/** First segment that carries each mistake: where its mark is scrolled to. */
const errSeg = computed(() => {
  const m = new Map<number, number>()
  props.check.segments.forEach((seg, si) => seg.errors.forEach((i) => !m.has(i) && m.set(i, si)))
  return m
})
const markId = (si: number) => `wmark-${si}`
const markOf = (idx: number) => {
  const si = errSeg.value.get(idx)
  return si === undefined ? null : document.getElementById(markId(si))
}
const reduced = () => matchMedia('(prefers-reduced-motion: reduce)').matches

async function scrollMark(idx: number) {
  await nextTick()
  markOf(idx)?.scrollIntoView({ block: isDesktop.value ? 'center' : 'start', behavior: reduced() ? 'auto' : 'smooth' })
}

type Via = 'mark' | 'list' | 'nav' | 'key'
function select(idx: number, via: Via) {
  sel.value = idx
  if (via !== 'nav') viaMark = via === 'mark' || via === 'key'
  if (via !== 'mark') void scrollMark(idx)
  if (via === 'key') void nextTick(() => markOf(idx)?.focus({ preventScroll: true }))
}

/**
 * A segment may carry several mistakes (they overlap or sit side by side): the
 * mark selects the first, and a repeated press cycles through the others; the
 * numbers printed after the mark show how many there are.
 */
function pickMark(si: number) {
  const seg = props.check.segments[si]
  if (!seg) return
  let pool = seg.errors.filter((i) => visible.value.includes(i))
  if (!pool.length) {
    typeFilter.value = null
    pool = seg.errors
  }
  const cur = sel.value === null ? -1 : pool.indexOf(sel.value)
  select(pool[(cur + 1) % pool.length]!, 'mark')
}

function step(d: 1 | -1, via: Via) {
  const v = visible.value
  if (!v.length) return
  const cur = sel.value === null ? -1 : v.indexOf(sel.value)
  const next = cur === -1 ? (d > 0 ? 0 : v.length - 1) : (cur + d + v.length) % v.length
  select(v[next]!, via)
}

function clearSel() {
  const idx = sel.value
  sel.value = null
  if (idx !== null) void nextTick(() => markOf(idx)?.focus({ preventScroll: true }))
}

/** ← / → / j / k walk the mistakes, Esc clears; only while focus is in the text or the inspector. */
function onKey(e: KeyboardEvent, via: Via) {
  if (e.altKey || e.ctrlKey || e.metaKey) return
  const el = e.target as HTMLElement
  if (el.closest('input, textarea, select')) return
  if (e.key === 'ArrowRight' || e.key === 'j') {
    e.preventDefault()
    step(1, via)
  } else if (e.key === 'ArrowLeft' || e.key === 'k') {
    e.preventDefault()
    step(-1, via)
  } else if (e.key === 'Escape' && sel.value !== null && isDesktop.value) {
    e.preventDefault()
    clearSel()
  }
}

const sheetOpen = computed(() => !isDesktop.value && tab.value === 'text' && sel.value !== null)
function onSheetClose() {
  const idx = sel.value
  sel.value = null
  // The sheet gives focus back to whatever opened it; after Prev / Next that is a stale mark.
  if (viaMark && idx !== null) window.setTimeout(() => markOf(idx)?.focus({ preventScroll: true }), 0)
}

function toggleFilter(type: string) {
  typeFilter.value = typeFilter.value === type ? null : type
}
watch(typeFilter, () => {
  if (sel.value !== null && !visible.value.includes(sel.value)) sel.value = null
})
watch(tab, () => (sel.value = null))

const segInFilter = (errs: number[]) =>
  !typeFilter.value || errs.some((i) => typeOf(props.check.errors[i]!) === typeFilter.value)

function markLabel(errs: number[]): string {
  const first = props.check.errors[errs[0]!]
  const base = t('writing.result.markLabel', { n: first?.n ?? errs[0]! + 1, quote: first?.quote ?? '' })
  return errs.length > 1 ? `${base}. ${t('writing.result.markMore', { n: errs.length - 1 })}` : base
}

/** Said aloud (polite) when the selection changes on a phone; the desktop inspector is itself live. */
const announce = computed(() => {
  const e = selErr.value
  if (!e) return ''
  return t('writing.result.announce', { i: pos.value + 1, n: visible.value.length, quote: e.quote, fix: e.fix })
})

/** Roving focus: the arrow keys (and Home / End) move focus to the tab they select. */
function onTabKey(e: KeyboardEvent, k: 'text' | 'corrected') {
  let next: 'text' | 'corrected' | null = null
  if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') next = k === 'text' ? 'corrected' : 'text'
  else if (e.key === 'Home') next = 'text'
  else if (e.key === 'End') next = 'corrected'
  if (!next) return
  e.preventDefault()
  tab.value = next
  void nextTick(() => document.getElementById(`wtab-${next}`)?.focus())
}

/** Whitespace at the edges of a segment stays outside its mark, so no orphan underline shows at a line end. */
function edges(text: string): [string, string, string] {
  const m = /^(\s*)([\s\S]*?)(\s*)$/.exec(text)
  return m ? [m[1]!, m[2]!, m[3]!] : ['', text, '']
}

const summaryOpen = ref(false)
const SUMMARY_CLAMP = 120

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
  <section class="flex min-w-0 flex-col gap-4 max-md:gap-3" :aria-label="t('writing.result.header', { model: check.model })">
    <header class="flex flex-col gap-1 max-md:gap-0.5">
      <h2 class="font-display text-lg font-medium max-md:text-base">
        {{ t('writing.result.header', { model: check.model }) }}
        <span class="font-sans text-sm font-normal text-muted-foreground">· {{ sourceLabel }}</span>
        <span class="font-sans text-xs font-normal text-muted-foreground md:hidden">· {{ shortDate(check.createdAt, ui.locale) }}</span>
      </h2>
      <p class="text-xs text-muted-foreground max-md:hidden">{{ shortDate(check.createdAt, ui.locale) }}</p>
      <p class="text-xs leading-snug text-muted-foreground">{{ t('writing.result.standing') }}</p>
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
        class="inline-flex min-h-11 cursor-pointer items-center text-sm font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-ring"
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

    <template v-if="check.status !== 'failed'">
      <!-- The strip: the model's summary and the mistake types (chips filter the marks). -->
      <div v-if="check.summary || check.errors.length || check.status === 'ok'" class="flex flex-col gap-3 rounded-xl border border-border-subtle bg-card p-4 max-md:gap-1 max-md:p-3">
        <div v-if="check.summary" class="flex flex-col items-start gap-1">
          <p
            class="text-sm leading-relaxed whitespace-pre-line"
            :class="!summaryOpen && check.summary.length > SUMMARY_CLAMP ? 'line-clamp-2 md:line-clamp-3' : ''"
          >{{ check.summary }}</p>
          <button
            v-if="check.summary.length > SUMMARY_CLAMP"
            type="button"
            class="inline-flex min-h-11 cursor-pointer items-center text-sm font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-ring"
            :aria-expanded="summaryOpen"
            @click="summaryOpen = !summaryOpen"
          >
            {{ summaryOpen ? t('writing.result.summaryLess') : t('writing.result.summaryMore') }}
          </button>
        </div>
        <p v-if="!check.errors.length && check.status === 'ok'" class="text-sm text-muted-foreground">{{ t('writing.result.noErrors') }}</p>
        <div v-if="check.errors.length" class="flex items-center gap-2 md:flex-wrap">
          <span class="shrink-0 text-sm font-medium whitespace-nowrap">{{ t('writing.result.mistakesCount', { n: check.errors.length }, check.errors.length) }}</span>
          <div
            role="group"
            :aria-label="t('writing.result.filterLabel')"
            class="flex min-w-0 flex-1 items-center gap-2 overflow-x-auto overscroll-x-contain p-1 [scrollbar-width:none] max-md:flex-nowrap max-md:scroll-px-2 md:flex-wrap md:overflow-visible md:p-0 [&::-webkit-scrollbar]:hidden"
          >
            <button
              v-for="c in typeCounts"
              :key="c.type"
              type="button"
              :aria-pressed="typeFilter === c.type"
              class="inline-flex min-h-9 shrink-0 cursor-pointer items-center gap-1.5 rounded-full border px-3 text-sm whitespace-nowrap transition-colors focus-visible:outline-2 focus-visible:outline-ring max-md:min-h-11"
              :class="typeFilter === c.type ? 'border-primary bg-primary text-primary-foreground' : 'border-border-subtle bg-secondary text-secondary-foreground hover:bg-accent'"
              @click="toggleFilter(c.type)"
              @focus="($event.currentTarget as HTMLElement).scrollIntoView({ inline: 'nearest', block: 'nearest', behavior: 'auto' })"
            >
              {{ typeLabel(c.type) }}
              <span class="tabular-nums opacity-80">{{ c.count }}</span>
            </button>
          </div>
        </div>
      </div>

      <div
        class="grid gap-5 lg:items-start"
        :class="isDesktop && check.errors.length && tab === 'text' ? 'lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]' : ''"
      >
        <div class="flex min-w-0 flex-col gap-3">
          <div v-if="check.corrected" role="tablist" :aria-label="t('writing.result.tabsLabel')" class="flex gap-1 rounded-xl border border-border-subtle bg-secondary p-1 self-start max-sm:self-stretch">
            <button
              v-for="k in (['text', 'corrected'] as const)"
              :id="`wtab-${k}`"
              :key="k"
              type="button"
              role="tab"
              :aria-selected="tab === k"
              :aria-controls="`wpanel-${k}`"
              :tabindex="tab === k ? 0 : -1"
              class="inline-flex min-h-11 flex-1 cursor-pointer items-center justify-center rounded-lg px-4 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-ring sm:flex-none"
              :class="tab === k ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'"
              @click="tab = k"
              @keydown="onTabKey($event, k)"
            >
              {{ k === 'text' ? t('writing.result.tabText') : t('writing.result.tabCorrected') }}
            </button>
          </div>

          <div v-if="tab === 'text' || !check.corrected" id="wpanel-text" role="tabpanel" aria-labelledby="wtab-text" class="flex flex-col gap-3">
            <p v-if="check.errors.length" class="text-xs text-muted-foreground">{{ t('writing.result.textHint') }}</p>
            <p
              lang="hu"
              class="rounded-xl border border-border-subtle bg-card p-4 text-base leading-9 whitespace-pre-wrap break-words"
              @keydown="onKey($event, 'key')"
            ><template v-for="(seg, i) in check.segments" :key="i"><template v-if="seg.errors.length && edges(seg.text)[1]">{{ edges(seg.text)[0] }}<span
                :id="markId(i)"
                role="button"
                tabindex="0"
                :aria-label="markLabel(seg.errors)"
                :aria-pressed="sel !== null && seg.errors.includes(sel)"
                class="relative scroll-mt-24 cursor-pointer rounded-sm px-0.5 underline decoration-wavy decoration-2 underline-offset-4 before:absolute before:-inset-x-0.5 before:-inset-y-3 focus-visible:outline-2 focus-visible:outline-ring"
                :class="
                  sel !== null && seg.errors.includes(sel)
                    ? 'bg-error/25 decoration-error ring-2 ring-error'
                    : segInFilter(seg.errors)
                      ? 'bg-error/10 decoration-error hover:bg-error/20'
                      : 'decoration-border hover:bg-accent'
                "
                @click="pickMark(i)"
                @keydown.enter.prevent="pickMark(i)"
                @keydown.space.prevent="pickMark(i)"
              >{{ edges(seg.text)[1] }}</span><sup
                v-if="firstSegment[i]!.length"
                class="ml-px mr-0.5 font-sans text-[11px] font-semibold text-error"
                aria-hidden="true"
              >{{ firstSegment[i]!.map((idx) => check.errors[idx]?.n ?? idx + 1).join(',') }}</sup>{{ edges(seg.text)[2] }}</template><template v-else>{{ seg.text }}<sup
                v-if="firstSegment[i]!.length"
                class="ml-px mr-0.5 font-sans text-[11px] font-semibold text-error"
                aria-hidden="true"
              >{{ firstSegment[i]!.map((idx) => check.errors[idx]?.n ?? idx + 1).join(',') }}</sup></template></template></p>

            <details v-if="check.errors.length" class="group rounded-xl border border-border-subtle bg-card">
              <summary class="flex min-h-11 cursor-pointer list-none items-center justify-between gap-2 px-4 text-sm font-medium focus-visible:outline-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden">
                {{ t('writing.result.allTitle', { n: visible.length }) }}
                <ChevronDown class="size-4 transition-transform group-open:rotate-180" aria-hidden="true" />
              </summary>
              <ul class="flex flex-col border-t border-border-subtle">
                <li v-for="i in visible" :key="check.errors[i]!.n">
                  <button
                    type="button"
                    class="flex min-h-11 w-full cursor-pointer items-center gap-2 px-4 py-1.5 text-left text-sm hover:bg-accent focus-visible:outline-2 focus-visible:outline-ring"
                    :class="sel === i ? 'bg-accent' : ''"
                    :aria-current="sel === i ? 'true' : undefined"
                    @click="select(i, 'list')"
                  >
                    <span class="inline-flex h-6 min-w-6 shrink-0 items-center justify-center rounded-full border border-error px-1.5 text-xs font-semibold text-error tabular-nums">{{ check.errors[i]!.n }}</span>
                    <span class="flex min-w-0 flex-1 items-baseline gap-1.5">
                      <span lang="hu" class="truncate text-error line-through">{{ check.errors[i]!.quote }}</span>
                      <ArrowRight class="size-3.5 shrink-0 self-center text-muted-foreground" aria-hidden="true" />
                      <span lang="hu" class="truncate text-success">{{ check.errors[i]!.fix }}</span>
                    </span>
                    <span v-if="!check.errors[i]!.anchored" class="shrink-0 text-xs text-muted-foreground">{{ t('writing.result.unlocated') }}</span>
                    <span class="hidden shrink-0 rounded-full border border-border-subtle bg-secondary px-2 py-0.5 text-xs text-secondary-foreground sm:inline">{{ typeLabel(typeOf(check.errors[i]!)) }}</span>
                  </button>
                </li>
              </ul>
            </details>
          </div>

          <div v-else id="wpanel-corrected" role="tabpanel" aria-labelledby="wtab-corrected" class="flex flex-col gap-2">
            <div class="flex flex-wrap items-center gap-2">
              <Button size="sm" variant="outline" class="max-md:min-h-11" :disabled="!playItems.length" @click="listen">
                <Square v-if="listening" class="size-4" aria-hidden="true" />
                <Volume2 v-else class="size-4" aria-hidden="true" />
                {{ listening ? t('writing.result.listenStop') : t('writing.result.listen') }}
              </Button>
              <Button v-if="tts.piperReady" size="sm" variant="ghost" class="max-md:min-h-11" @click="tts.openPicker()">
                {{ tts.voiceName || t('writing.result.voice') }}
              </Button>
              <Button size="sm" variant="outline" class="max-md:min-h-11" @click="copyCorrected">
                <Copy class="size-4" aria-hidden="true" />
                {{ copied === 'ok' ? t('writing.result.copied') : t('writing.result.copy') }}
              </Button>
            </div>
            <p
              ref="correctedEl"
              lang="hu"
              class="rounded-xl border border-border-subtle bg-card p-4 text-base leading-7 whitespace-pre-wrap break-words"
            >{{ check.corrected }}</p>
            <p v-if="copied === 'fallback'" class="text-xs text-muted-foreground" role="status">{{ t('writing.result.copyFallback') }}</p>
          </div>
        </div>

        <!-- Wide screens: the inspector, sticky, as tall as its content. -->
        <aside
          v-if="isDesktop && check.errors.length && tab === 'text'"
          :aria-label="t('writing.result.inspectorTitle')"
          aria-live="polite"
          class="flex min-w-0 flex-col gap-3 rounded-xl border border-border-subtle bg-card p-4 lg:sticky lg:top-4 lg:max-h-[calc(100dvh-6rem)] lg:overflow-y-auto"
          @keydown="onKey($event, 'nav')"
        >
          <MistakeInspector v-if="selErr" :error="selErr" :pos="pos" :total="visible.length" @prev="step(-1, 'nav')" @next="step(1, 'nav')" />
          <div v-else class="flex flex-col items-start gap-3 text-sm">
            <p class="text-muted-foreground">{{ t('writing.result.pickHint') }}</p>
            <Button size="sm" variant="outline" class="min-h-11" :disabled="!visible.length" @click="step(1, 'nav')">
              {{ t('writing.result.startTour') }}
            </Button>
          </div>
        </aside>
      </div>

      <!-- Phones: the same inspector in a sheet that leaves the text in sight. -->
      <BottomSheet modeless hide-title :open="sheetOpen" :title="t('writing.result.inspectorTitle')" @close="onSheetClose">
        <div v-if="selErr" class="pt-1 pb-2" @keydown="onKey($event, 'nav')">
          <MistakeInspector :error="selErr" :pos="pos" :total="visible.length" @prev="step(-1, 'nav')" @next="step(1, 'nav')" />
        </div>
      </BottomSheet>
      <p v-if="!isDesktop" class="sr-only" role="status" aria-live="polite">{{ announce }}</p>

      <details v-if="check.strengths.length" class="group rounded-xl border border-border-subtle bg-card">
        <summary class="flex min-h-11 cursor-pointer list-none items-center justify-between gap-2 px-4 text-sm font-medium focus-visible:outline-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden">
          {{ t('writing.result.strengths') }}
          <ChevronDown class="size-4 transition-transform group-open:rotate-180" aria-hidden="true" />
        </summary>
        <ul class="list-disc space-y-0.5 border-t border-border-subtle py-3 pr-4 pl-9 text-sm leading-relaxed">
          <li v-for="(s, i) in check.strengths" :key="i">{{ s }}</li>
        </ul>
      </details>
      <details v-if="check.vocab.length" class="group rounded-xl border border-border-subtle bg-card">
        <summary class="flex min-h-11 cursor-pointer list-none items-center justify-between gap-2 px-4 text-sm font-medium focus-visible:outline-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden">
          {{ t('writing.result.vocab') }}
          <ChevronDown class="size-4 transition-transform group-open:rotate-180" aria-hidden="true" />
        </summary>
        <ul class="list-disc space-y-0.5 border-t border-border-subtle py-3 pr-4 pl-9 text-sm leading-relaxed">
          <li v-for="(v, i) in check.vocab" :key="i">{{ v }}</li>
        </ul>
      </details>
    </template>
    <VoicePicker />
  </section>
</template>
