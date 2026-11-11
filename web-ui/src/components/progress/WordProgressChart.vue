<script setup lang="ts">
import { computed } from 'vue'
import Note from '@/components/common/Note.vue'

/**
 * Words by progress: ten columns, one per tenth of the 0–100 scale.
 *
 * [ADR-070] Counts of WORDS with a reviewed card, by their progress number —
 * the same number the word's own bar shows, so a column and a bar never
 * disagree. Columns are tinted by band (0–19 learning, 20–69 young, 70–100
 * settled) in the stage-chip tones; the marks at 20 and 70 carry the stage
 * names, and the exact counts are printed on the columns and repeated in a
 * table for screen readers, so colour is never the only channel. Plain divs,
 * like the other progress components. It is a picture of where the deck is
 * today, not a target: counts go down as well as up.
 */
const props = defineProps<{ progress: number[] | null }>()

const counts = computed(() => {
  const p = props.progress ?? []
  return Array.from({ length: 10 }, (_, i) => Math.max(0, p[i] ?? 0))
})
const total = computed(() => counts.value.reduce((a, b) => a + b, 0))
const max = computed(() => Math.max(1, ...counts.value))

const cols = computed(() =>
  counts.value.map((n, i) => ({
    n,
    from: i * 10,
    to: i === 9 ? 100 : i * 10 + 9,
    // Heights are a share of the tallest column, with a sliver for any non-zero count.
    h: n === 0 ? 0 : Math.max(4, Math.round((n / max.value) * 100)),
    tone: i < 2 ? 'bg-learning' : 'bg-known',
  })),
)
</script>

<template>
  <div class="flex flex-col gap-2">
    <Note v-if="total === 0" variant="info" bare>{{ $t('progress.byProgress.empty') }}</Note>

    <template v-else>
      <div class="relative" role="img" :aria-label="$t('progress.byProgress.alt')">
        <div class="flex h-28 items-end gap-1" aria-hidden="true">
          <div
            v-for="c in cols"
            :key="c.from"
            class="flex h-full min-w-0 flex-1 flex-col items-center justify-end gap-0.5"
            :title="$t('progress.byProgress.bar', { from: c.from, to: c.to, n: c.n })"
          >
            <span class="text-[10px] leading-none tabular-nums text-muted-foreground">{{ c.n || '' }}</span>
            <div
              class="w-full rounded-t-sm motion-safe:transition-[height] motion-safe:duration-500"
              :class="c.tone"
              :style="{ height: `${c.h * 0.8}%` }"
            />
          </div>
        </div>
        <!-- The stage marks: boundaries at 20 and 70, named with the stage words. -->
        <span class="pointer-events-none absolute inset-y-0 left-[20%] border-l border-dashed border-foreground/35" aria-hidden="true" />
        <span class="pointer-events-none absolute inset-y-0 left-[70%] border-l border-dashed border-foreground/35" aria-hidden="true" />
        <div class="relative mt-1 h-3 text-[10px] leading-none text-muted-foreground" aria-hidden="true">
          <span class="absolute left-0 whitespace-nowrap">{{ $t('review.memory.chip.learning') }}</span>
          <span class="absolute left-[20%] whitespace-nowrap pl-1">{{ $t('review.memory.chip.young') }}</span>
          <span class="absolute left-[70%] whitespace-nowrap pl-1">{{ $t('review.memory.chip.mature') }}</span>
        </div>
      </div>
      <p class="text-[11px] leading-relaxed text-muted-foreground">{{ $t('progress.byProgress.hint') }}</p>

      <table class="sr-only">
        <caption>{{ $t('progress.byProgress.title') }}</caption>
        <thead>
          <tr>
            <th scope="col">{{ $t('progress.byProgress.range') }}</th>
            <th scope="col">{{ $t('progress.byProgress.words') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in cols" :key="c.from">
            <th scope="row">{{ c.from }}–{{ c.to }}%</th>
            <td>{{ c.n }}</td>
          </tr>
        </tbody>
      </table>
    </template>
  </div>
</template>
