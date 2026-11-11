<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { CircleHelp } from '@lucide/vue'
import { Check } from '@lucide/vue'
import HowItWorks from '@/components/review/HowItWorks.vue'
import WordProgress from '@/components/common/WordProgress.vue'
import { daysInterval, type Interval } from '@/lib/interval'
import { useI18n } from 'vue-i18n'
import type { Memory } from '@/types'

/**
 * Where this card stands, in one line under the progress strip.
 *
 * [R18 §4] [ADR-039] [ADR-070] A POSITION, not a score: the stage the scheduler
 * reports («новая» · «учу · шаг 2 из 2» · «повторяю» · «закреплено»), drawn as
 * a chip with the word's progress bar beside it (0–100, the stages are marks on
 * it). It goes down on a mistake, 100 is not an end and nothing here
 * accumulates. After the answer the bar moves to the new value and the change
 * shows for a moment. The tally «верно N из M» lived here and is gone —
 * it is a history, and history lives in the words browser; the chance of
 * recall was taken off the card earlier (ADR-026, amended 2026-09-03).
 *
 * The «?» opens HowItWorks, the one place that explains the stages, the four
 * buttons and what a miss does. After the answer the interval it had against
 * the one it has now is shown, because that is a measured fact about the
 * answer just given.
 */
const props = withDefaults(
  defineProps<{
    memory: Memory
    /** The card's memory AFTER the answer, once there is one. */
    after?: Memory | null
    /** Whether the card's grade preview applies (it is absent from practice cards). */
    scheduled?: boolean
  }>(),
  { after: null, scheduled: true },
)

const { t } = useI18n()

/** The card AFTER the answer once there is one, so the chip moves with the verdict. */
const shown = computed(() => props.after ?? props.memory)
const seen = computed(() => props.memory.total > 0 || props.memory.stage !== 'new')
const howOpen = ref(false)

/** Stage chip tints (ADR-059): новое = mist, учу = cream, повторяю = sage, закреплено = sage + check. Ink text on all. */
const stageTint = computed(() => {
  switch (shown.value.stage) {
    case 'new':
      return 'border-tint-mist-edge bg-tint-mist'
    case 'young':
      return 'border-tint-sage-edge bg-tint-sage'
    case 'mature':
      return 'border-tint-sage-edge bg-tint-sage'
    default:
      return 'border-tint-cream-edge bg-tint-cream'
  }
})

function label(i: Interval) {
  return t(`review.units.${i.unit}`, { n: i.n })
}

/** «3 дн → 8 дн»: the interval before against the one after. */
const change = computed(() => {
  if (!props.after) return null
  const before = props.memory
  const after = props.after
  // A learning step is minutes; the day-count would print as 0.
  if (after.stage === 'learning' || after.stage === 'relearning') return null
  return {
    from: seen.value ? label(daysInterval(before.intervalDays)) : t('review.memory.unseen'),
    to: label(daysInterval(after.intervalDays)),
    up: after.intervalDays > before.intervalDays,
  }
})

/**
 * The change the answer made, shown for ~1.5 s and then dropped (the bar keeps
 * the new value). A same-day answer on a settled card gives +0 by the
 * scheduler's own arithmetic; say so quietly rather than leave a bare «+0».
 */
const DELTA_MS = 1500
const delta = ref<number | null>(null)
let deltaTimer: number | undefined
watch(
  () => props.after,
  (a) => {
    window.clearTimeout(deltaTimer)
    if (!a) {
      delta.value = null
      return
    }
    delta.value = a.progress - props.memory.progress
    deltaTimer = window.setTimeout(() => (delta.value = null), DELTA_MS)
  },
  { immediate: true },
)
onBeforeUnmount(() => window.clearTimeout(deltaTimer))

const settledToday = computed(
  () =>
    props.after !== null &&
    props.after.progress === props.memory.progress &&
    props.after.stage !== 'learning' &&
    props.after.stage !== 'relearning' &&
    props.after.stage !== 'new',
)

/** «шаг 2 из 2»: `stepsLeft` counts DOWN from `stepsTotal`. Absent once on days. */
const step = computed(() => {
  const m = shown.value
  if (!m.stepsTotal || !m.stepsLeft) return null
  return { n: m.stepsTotal - m.stepsLeft + 1, total: m.stepsTotal }
})
</script>

<template>
  <div class="flex min-h-8 flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
    <!-- Keyed on the stage so a change animates in rather than swapping silently. -->
    <span
      :key="shown.stage"
      class="animate-tick-up inline-flex h-8 items-center gap-1 rounded-md border px-2.5 text-xs font-medium text-foreground"
      :class="stageTint"
    >
      <Check v-if="shown.stage === 'mature'" class="size-3.5" aria-hidden="true" />
      {{ $t(`review.memory.chip.${shown.stage}`) }}
      <template v-if="step"> · {{ $t('review.memory.stepShort', step) }}</template>
    </span>
    <!-- Where the word is now; the chip above names the stage in words. -->
    <div class="min-w-64 max-w-96 flex-1">
      <WordProgress :value="shown.progress" :stage="shown.stage" size="card" :delta="delta" />
    </div>
    <button
      type="button"
      class="flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-full text-muted-foreground/70 hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"
      :title="$t('review.how.link')"
      :aria-label="$t('review.how.link')"
      @click="howOpen = true"
    >
      <CircleHelp class="size-4" aria-hidden="true" />
    </button>
    <HowItWorks :open="howOpen" :memory="memory" :scheduled="scheduled" @close="howOpen = false" />

    <!-- After the answer: where the interval went. Keyed so it animates in. -->
    <span
      v-if="change"
      :key="change.to"
      class="animate-tick-up flex items-center gap-1 font-mono tabular-nums"
      :class="change.up ? 'text-known' : 'text-learning'"
    >
      {{ $t('review.memory.intervalChange', { from: change.from, to: change.to }) }}
    </span>
    <p v-if="settledToday" class="animate-tick-up basis-full text-xs text-muted-foreground" data-testid="same-day-note">
      {{ $t('review.memory.sameDay') }}
    </p>
  </div>
</template>
