<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useFeaturesStore } from '@/stores/features'
import type { FeatureMastery, MorphFeature } from '@/types'

const { t } = useI18n()
const catalogue = useFeaturesStore()

/**
 * The 3×3 locative grid, coloured by mastery.
 *
 * Teaching the nine cells AS A TABLE is the point ([R15]): for a Russian
 * speaker the where/whither/whence logic already exists — only the encoding is
 * new. The colours make the per-cell state visible, which no wordform-atomic
 * tool can do, because no such tool has per-suffix state to show.
 */
const props = defineProps<{
  features: MorphFeature[]
  mastery: FeatureMastery[]
  selected?: string
}>()
const emit = defineEmits<{ select: [featureId: string] }>()

const rows = ['interior', 'surface', 'proximity'] as const
const cols = ['where', 'whither', 'whence'] as const

/**
 * One cell.
 *
 * Three states, and the difference between them is the whole point: a suffix
 * you have answered well, a suffix you have met but not enough times to draw a
 * number from, and a suffix you have never met. The grid used to print an em
 * dash for the last two alike, so seven of nine cells read as broken rather
 * than as untouched — which was the true and much less alarming answer.
 *
 * The measure is the rolling accuracy, matching the heatmap beside it; it used
 * to be FSRS retrievability, which says how recently you revised rather than
 * how well you know it.
 */
interface Cell {
  suffix: string
  id: string
  /** Null when the learner has never met this feature at all. */
  mastery: FeatureMastery | null
}

const cells = computed(() => {
  const m = new Map(props.mastery.map((x) => [x.featureId, x]))
  const out = new Map<string, Cell>()
  for (const f of props.features) {
    if (!f.grid) continue
    out.set(f.grid, {
      suffix: Object.values(f.allomorphs).filter(Boolean)[0] ?? '',
      id: f.id,
      mastery: m.get(f.id) ?? null,
    })
  }
  return out
})

const cell = (r: string, c: string): Cell | undefined => cells.value.get(`${r}/${c}`)

function tone(x: Cell | undefined) {
  if (!x?.mastery) return 'border-border bg-muted/20 text-muted-foreground'
  // Not enough answers is a state of its own, and it is drawn as one: a dashed
  // outline, no fill, no colour it has not earned.
  if (!x.mastery.ready) return 'border-dashed border-border bg-transparent text-muted-foreground'
  return x.mastery.weak
    ? 'border-tint-cream-edge bg-tint-cream text-foreground'
    : 'border-tint-sage-edge bg-tint-sage text-foreground'
}

/** What the second line of a cell says. Never a percentage we cannot support. */
function figure(x: Cell | undefined): string {
  if (!x?.mastery) return t('progress.neverMet')
  if (!x.mastery.ready) return t('progress.answers', { n: x.mastery.observations })
  return `${Math.round(x.mastery.accuracy * 100)}%`
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
          <button
            v-if="cell(r, c)"
            type="button"
            class="w-full rounded-md border px-2 py-2 text-center focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring"
            :class="[tone(cell(r, c)), cell(r, c)!.id === selected ? 'ring-2 ring-foreground' : '']"
            :aria-label="`${catalogue.label(cell(r, c)!.id)}: ${figure(cell(r, c))}`"
            :aria-pressed="cell(r, c)!.id === selected"
            @click="emit('select', cell(r, c)!.id)"
          >
            <div v-if="cell(r, c)?.suffix" class="font-mono" lang="hu">
              -{{ cell(r, c)!.suffix }}
            </div>
            <div class="text-[10px] opacity-75">{{ figure(cell(r, c)) }}</div>
          </button>
        </td>
      </tr>
    </tbody>
  </table>
</template>
