<script setup lang="ts">
import { computed } from 'vue'
import ProgressRing from '@/components/visual/ProgressRing.vue'
import { Button } from '@/components/ui/button'
import InfoTip from '@/components/common/InfoTip.vue'
import MissedList from '@/components/review/MissedList.vue'
import type { Missed } from '@/stores/review'

/**
 * The end of one set: what happened, measured, and the way to the next one.
 *
 * [R18 §2] A set finishing is a real event and is marked — with a two-note cue
 * and this card — but it is not celebrated: no praise line, no points, nothing
 * that goes up by itself ([R18 §4], [R14]). The figures are the ones a learner
 * can check against the cards they just answered. «Закрепилось» names the
 * words whose next review moved three weeks or more out — Anki's "mature",
 * said to be a convention rather than a threshold.
 *
 * The misses are LISTED with their answers, not re-served: the card returns
 * through the server's queue once its learning step has elapsed — in one of
 * the next sets if it has by then, later today otherwise. Reading the pair
 * over here is the one thing a miss gets in the meantime.
 */
const props = defineProps<{
  stats: {
    answered: number
    right: number
    missed: Missed[]
    matured: string[]
    activeMs: number
  }
  /** «набор 2 из 5». */
  set: { n: number; of: number }
  /** Cards still on the served list, freshly fetched. */
  left: number
}>()

const emit = defineEmits<{ next: []; stop: [] }>()

const minutes = computed(() => Math.max(1, Math.round(props.stats.activeMs / 60_000)))

/** Right out of answered: a share with a counted denominator, and the label says the counts. */
const ringValue = computed(() => (props.stats.answered ? props.stats.right / props.stats.answered : 0))

/** Tinted tiles (ADR-059): ink text on every tint. */
const tiles = computed(() => [
  { key: 'answered', n: props.stats.answered, cls: 'border-tint-mist-edge bg-tint-mist' },
  { key: 'right', n: props.stats.right, cls: 'border-tint-sage-edge bg-tint-sage' },
  { key: 'missed', n: props.stats.missed.length, cls: 'border-tint-blush-edge bg-tint-blush' },
  { key: 'minutes', n: minutes.value, cls: 'border-tint-cream-edge bg-tint-cream' },
])
</script>

<template>
  <div class="flex flex-col items-center gap-3 py-6 text-center">
    <ProgressRing
      :value="ringValue"
      :size="96"
      :stroke="8"
      :state="stats.answered > 0 && stats.right === stats.answered ? 'settled' : 'progress'"
      :label="`${stats.right}/${stats.answered}`"
    />
    <h1 class="text-xl font-medium">{{ $t('review.set.done') }}</h1>
    <p class="text-sm text-muted-foreground">
      {{ $t('review.set.label', { n: set.n, of: set.of }) }}
      · {{ $t('review.set.ofAnswered', { right: stats.right, total: stats.answered }) }}
    </p>

    <!-- The way on comes FIRST, so a phone reaches it without scrolling past
         the figures. -->
    <div class="flex w-full max-w-sm flex-col gap-2 sm:w-auto sm:max-w-none sm:flex-row sm:justify-center">
      <Button size="lg" class="h-12 sm:h-auto" @click="emit('next')">
        {{ $t('review.set.next') }}
        <span class="hidden font-mono text-[10px] opacity-70 sm:inline">space</span>
      </Button>
      <Button variant="ghost" class="h-11 sm:h-auto" @click="emit('stop')">
        {{ $t('review.set.stop') }}
      </Button>
    </div>

    <dl class="mt-1 grid w-full max-w-xl grid-cols-2 gap-3 sm:grid-cols-4">
      <div
        v-for="(tile, i) in tiles"
        :key="tile.key"
        class="card-in rounded-xl border p-3 text-center"
        :class="tile.cls"
        :style="{ '--i': i }"
      >
        <dt class="text-[11px] tracking-wide text-foreground/80 uppercase">
          {{ $t(`review.set.summary.${tile.key}`) }}
        </dt>
        <dd class="font-mono text-2xl tabular-nums text-foreground">{{ tile.n }}</dd>
      </div>
    </dl>

    <!-- The misses, to read over. Left-aligned inside a centred screen on
         purpose: it is a list to read, not a figure to glance at. -->
    <div
      v-if="stats.missed.length"
      class="card-in w-full max-w-md rounded-xl border border-tint-blush-edge bg-tint-blush px-5 py-3 text-left"
    >
      <p class="flex items-center gap-1 text-[11px] tracking-wide text-foreground/80 uppercase">
        {{ $t('review.set.missedTitle') }}
        <InfoTip>{{ $t('review.set.missedBody') }}</InfoTip>
      </p>
      <MissedList :items="stats.missed" class="mt-2" />
    </div>

    <p
      v-if="stats.matured.length"
      class="max-w-md text-xs leading-relaxed text-muted-foreground"
      :title="$t('review.set.maturedHint')"
    >
      {{ $t('review.set.maturedList', { words: stats.matured.join(', ') }) }}
    </p>

    <p v-if="left > 0" class="text-xs text-muted-foreground">
      {{ $t('review.set.left', { n: left }) }}
    </p>

  </div>
</template>
