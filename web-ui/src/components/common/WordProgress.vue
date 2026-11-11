<script setup lang="ts">
import { computed } from 'vue'
import { ArrowDown } from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import type { MemoryStage } from '@/types'

/**
 * Where a word is now, 0–100, drawn as one bar with the stages as marks on it.
 *
 * [ADR-070] The number comes from the scheduler's own state (`Memory.progress`);
 * nothing here computes it. The marks at 20 and 70 are where «учу» ends and
 * where «закреплено» begins, so the number and the stage chip always agree.
 * It goes DOWN on a mistake: that is a fact about the word, so a fall is drawn
 * quietly (muted, an arrow) and never in the error colour. It is progress and
 * nothing else — never a score, points or a level; 100 means about half a year
 * between showings, and the word still comes back.
 *
 * Reduced motion: the width does not animate. The fill tone is the stage
 * chip's (learning = amber token, young/mature = known token); the number and
 * the words carry the meaning, colour never does alone (UX.md §14.1).
 */
const props = withDefaults(
  defineProps<{
    /** null renders nothing: a word with no card has no progress. */
    value: number | null
    stage?: MemoryStage | 'known' | '' | null
    size?: 'row' | 'card' | 'detail'
    /** Change from the answer just given. Absent = no pill. */
    delta?: number | null
  }>(),
  { stage: null, size: 'card', delta: null },
)

const { t } = useI18n()

const pct = computed(() => Math.max(0, Math.min(100, Math.round(props.value ?? 0))))

const tone = computed(() =>
  props.stage === 'young' || props.stage === 'mature' ? 'bg-known' : 'bg-learning',
)

const height = computed(() =>
  props.size === 'row' ? 'h-1.5' : props.size === 'card' ? 'h-2' : 'h-2.5',
)

/** Only the full-width detail bar has room to name its marks; elsewhere the
 * stage chip beside the bar says it in words. */
const named = computed(() => props.size === 'detail')
const labelled = computed(() => props.size !== 'row')

const stageName = computed(() => {
  const s = props.stage
  return s && s !== 'known' ? t(`review.memory.chip.${s}`) : ''
})

const valueText = computed(() =>
  stageName.value
    ? t('wordProgress.aria', { n: pct.value, stage: stageName.value })
    : t('wordProgress.ariaPlain', { n: pct.value }),
)

/** «+4», «−58», «+0»: the real minus sign, so the pill never reads as a hyphen. */
const deltaText = computed(() => {
  const d = props.delta
  if (d === null || d === undefined) return ''
  return d < 0 ? `−${Math.abs(d)}` : `+${d}`
})
const deltaSpoken = computed(() => {
  const d = props.delta
  if (d === null || d === undefined) return ''
  if (d > 0) return t('wordProgress.up', { n: d })
  if (d < 0) return t('wordProgress.down', { n: Math.abs(d) })
  return t('wordProgress.same')
})
</script>

<template>
  <div
    v-if="value !== null"
    class="flex min-w-0 gap-2"
    :class="size === 'detail' ? 'items-start' : 'items-center'"
    data-testid="word-progress"
  >
    <div class="flex min-w-12 flex-1 flex-col gap-1" :class="size === 'row' ? 'max-w-28' : ''">
      <div
        role="progressbar"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-valuenow="pct"
        :aria-valuetext="valueText"
        class="relative w-full overflow-hidden rounded-full bg-muted"
        :class="height"
      >
        <div
          class="h-full rounded-full motion-safe:transition-[width] motion-safe:duration-500"
          :class="tone"
          :style="{ width: `${pct}%` }"
        />
        <!-- The two marks: where «учу» ends and where «закреплено» begins. -->
        <span class="absolute inset-y-0 left-[20%] w-px bg-foreground/35" aria-hidden="true" />
        <span class="absolute inset-y-0 left-[70%] w-px bg-foreground/35" aria-hidden="true" />
      </div>
      <!-- The marks named with the existing stage words; the row has ticks only. -->
      <div
        v-if="named"
        class="relative h-3 text-[10px] leading-none text-muted-foreground"
        aria-hidden="true"
      >
        <span class="absolute left-0 whitespace-nowrap">{{ $t('review.memory.chip.learning') }}</span>
        <span class="absolute left-[20%] whitespace-nowrap pl-0.5">{{ $t('review.memory.chip.young') }}</span>
        <span class="absolute right-0 whitespace-nowrap">{{ $t('review.memory.chip.mature') }}</span>
      </div>
    </div>
    <span
      class="shrink-0 tabular-nums"
      :class="size === 'row' ? 'text-xs' : size === 'card' ? 'text-sm font-medium' : '-mt-1 text-sm font-medium'"
      aria-hidden="true"
    >
      {{ pct }}%<span v-if="labelled" class="ml-1 text-xs font-normal text-muted-foreground">{{
        $t('wordProgress.label')
      }}</span>
    </span>
    <span
      v-if="deltaText"
      :key="`${deltaText}-${pct}`"
      class="animate-tick-up inline-flex shrink-0 items-center gap-0.5 rounded-full px-1.5 text-xs font-medium tabular-nums"
      :class="[(delta ?? 0) > 0 ? 'text-success' : 'text-muted-foreground', size === 'detail' ? '-mt-0.5' : '']"
      data-testid="word-progress-delta"
    >
      <ArrowDown v-if="(delta ?? 0) < 0" class="size-3" aria-hidden="true" />
      <span aria-hidden="true">{{ deltaText }}</span>
      <span class="sr-only">{{ deltaSpoken }}</span>
    </span>
  </div>
</template>
