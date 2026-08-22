<script setup lang="ts">
import { computed } from 'vue'
import type { FeatureMastery, MorphFeature } from '@/types'

/**
 * The 3×3 locative grid, coloured by mastery.
 *
 * Teaching the nine cells AS A TABLE is the point ([R15]): for a Russian
 * speaker the where/whither/whence logic already exists — only the encoding is
 * new. The colours make the per-cell state visible, which no wordform-atomic
 * tool can do, because no such tool has per-suffix state to show.
 */
const props = defineProps<{ features: MorphFeature[]; mastery: FeatureMastery[] }>()

const rows = ['interior', 'surface', 'proximity'] as const
const cols = ['where', 'whither', 'whence'] as const

const cells = computed(() => {
  const m = new Map(props.mastery.map((x) => [x.featureId, x]))
  const out = new Map<string, { suffix: string; r: number | null; id: string }>()
  for (const f of props.features) {
    if (!f.grid) continue
    const state = m.get(f.id)
    out.set(f.grid, {
      suffix: Object.values(f.allomorphs).filter(Boolean)[0] ?? '',
      r: state ? state.retrievability : null,
      id: f.id,
    })
  }
  return out
})

function tone(r: number | null) {
  if (r === null) return 'border-border bg-muted/20 text-muted-foreground'
  if (r >= 0.8) return 'border-known/40 bg-known/15 text-known'
  if (r >= 0.5) return 'border-learning/40 bg-learning/15 text-learning'
  return 'border-error/40 bg-error/15 text-error'
}
</script>

<template>
  <table class="w-full max-w-md border-separate border-spacing-1 text-xs">
    <thead>
      <tr>
        <th></th>
        <th v-for="c in cols" :key="c" class="pb-1 font-medium text-muted-foreground">
          {{ $t(`grammar.column.${c}`) }}
        </th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="r in rows" :key="r">
        <th class="pr-1 text-right font-medium text-muted-foreground">
          {{ $t(`grammar.row.${r}`) }}
        </th>
        <td v-for="c in cols" :key="c">
          <div
            class="rounded-md border px-2 py-2 text-center"
            :class="tone(cells.get(`${r}/${c}`)?.r ?? null)"
            :title="cells.get(`${r}/${c}`)?.id"
          >
            <div class="font-mono" lang="hu">-{{ cells.get(`${r}/${c}`)?.suffix || '?' }}</div>
            <div class="text-[10px] opacity-75">
              {{
                cells.get(`${r}/${c}`)?.r === null || cells.get(`${r}/${c}`) === undefined
                  ? '—'
                  : Math.round((cells.get(`${r}/${c}`)!.r as number) * 100) + '%'
              }}
            </div>
          </div>
        </td>
      </tr>
    </tbody>
  </table>
</template>
