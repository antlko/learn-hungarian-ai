<script setup lang="ts">
import { computed } from 'vue'
import { useUiStore } from '@/stores/ui'
import type { TopicFeature } from '@/types'

/**
 * The atoms a topic teaches, with the learner's state on each.
 *
 * This is the two-state-space thesis on the grammar screen: a topic is not one
 * thing you either know or do not, it is N grammatical facts each with its own
 * schedule. "Your inessive is solid, your delative is at 0.41" is only sayable
 * because features carry independent FSRS state.
 *
 * A feature never graded shows a dash, not 0% — no data is not failure.
 */
const props = defineProps<{ features: TopicFeature[]; harmonyLatencyMs: number }>()
const ui = useUiStore()

const label = (f: TopicFeature) => (ui.locale === 'ru' ? f.labelRu : f.labelEn)

/** The nine locative cells, laid out as the 3×3 table they actually are. [R15] */
const grid = computed(() => {
  const rows = ['interior', 'surface', 'proximity']
  const cols = ['where', 'whither', 'whence']
  const cell: Record<string, TopicFeature> = {}
  for (const f of props.features) if (f.grid) cell[f.grid] = f
  const complete = rows.every((r) => cols.every((c) => cell[`${r}/${c}`]))
  return complete ? { rows, cols, cell } : null
})

function tone(f: TopicFeature | undefined) {
  if (!f?.seen) return 'border-border bg-muted/20 text-muted-foreground'
  if (f.retrievability >= 0.8) return 'border-known/40 bg-known/10 text-known'
  if (f.retrievability >= 0.5) return 'border-learning/40 bg-learning/10 text-learning'
  return 'border-error/40 bg-error/10 text-error'
}
</script>

<template>
  <div class="flex flex-col gap-3">
    <!-- The locative system is a table, and teaching it as a table is the whole
         point: for a Russian speaker the semantics are already there. [R15] -->
    <table v-if="grid" class="w-full max-w-md border-separate border-spacing-1 text-xs">
      <thead>
        <tr>
          <th class="w-20"></th>
          <th v-for="c in grid.cols" :key="c" class="font-medium text-muted-foreground">
            {{ $t(`grammar.column.${c}`) }}
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in grid.rows" :key="r">
          <th class="text-right font-medium text-muted-foreground">
            {{ $t(`grammar.row.${r}`) }}
          </th>
          <td v-for="c in grid.cols" :key="c">
            <div
              class="rounded-md border px-2 py-1.5 text-center"
              :class="tone(grid.cell[`${r}/${c}`])"
            >
              <div class="font-mono">
                {{ Object.values(grid.cell[`${r}/${c}`]?.allomorphs ?? {})[0] || '—' }}
              </div>
              <div class="text-[10px] opacity-70">
                {{
                  grid.cell[`${r}/${c}`]?.seen
                    ? `${Math.round((grid.cell[`${r}/${c}`]?.retrievability ?? 0) * 100)}%`
                    : '—'
                }}
              </div>
            </div>
          </td>
        </tr>
      </tbody>
    </table>

    <ul class="stagger-in flex flex-col gap-1">
      <li
        v-for="f in features"
        :key="f.id"
        class="flex items-center justify-between gap-3 rounded-md border border-border-subtle bg-background px-2.5 py-1.5 text-xs"
      >
        <span class="flex min-w-0 flex-col">
          <span class="truncate">{{ label(f) }}</span>
          <span class="font-mono text-[10px] text-muted-foreground">{{ f.id }}</span>
        </span>
        <span class="flex shrink-0 items-center gap-2 font-mono">
          <span v-if="!f.seen" class="text-muted-foreground">{{ $t('grammar.notSeen') }}</span>
          <template v-else>
            <!-- Two numbers that are easy to confuse, so both are labelled.
                 Retrievability is "could you recall it right now" and drives the
                 schedule; accuracy is "how often you have been right lately" and
                 drives the gate. Showing only the first put a green 100% beside
                 a topic sitting at 45%. -->
            <span :class="tone(f).split(' ').pop()">
              {{ $t('grammar.recall') }} {{ Math.round(f.retrievability * 100) }}%
            </span>
            <span class="text-muted-foreground">
              {{ $t('grammar.hitRate') }} {{ Math.round(f.accuracy * 100) }}%
            </span>
            <!-- Latency, because for a motor skill correct-but-slow is not
                 mastery and the number has to be visible to mean anything. -->
            <span v-if="f.medianMs > 0" class="text-muted-foreground">
              {{ (f.medianMs / 1000).toFixed(1) }}s
            </span>
            <span v-if="f.automated" class="text-known">{{ $t('grammar.automated') }}</span>
          </template>
        </span>
      </li>
    </ul>
  </div>
</template>
