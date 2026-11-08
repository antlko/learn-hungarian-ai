<script setup lang="ts">
import { computed } from 'vue'
import { buttonVariants } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useUiStore } from '@/stores/ui'
import type { UnitState } from '@/types'

/**
 * Six months, twenty-four units — grouped by month so the whole thing reads as a
 * plan rather than a wall. A row carries only what decides "am I here yet":
 * status, title, and how much of its vocabulary is already in the deck.
 *
 * Every row is a live button, `ahead` ones included. The API refuses nothing —
 * the order is advice — so greying a later unit out would be the interface
 * inventing a restriction that does not exist. See internal/course.StatusAhead.
 */
const props = defineProps<{ units: UnitState[]; selected: string }>()
const emit = defineEmits<{ select: [code: string] }>()

const ui = useUiStore()

const months = computed(() => {
  // The API returns units in teaching order, so a running group is enough and a
  // sort would only risk disagreeing with the server about the order.
  const groups: { month: number; units: UnitState[] }[] = []
  for (const u of props.units) {
    const last = groups[groups.length - 1]
    if (last && last.month === u.unit.month) last.units.push(u)
    else groups.push({ month: u.unit.month, units: [u] })
  }
  return groups
})

/**
 * The dot. It answers "where am I", and it must not answer it wrongly.
 *
 * `done` keeps its own colour and stays derived from the deck: a unit whose
 * words are all in play IS done, however they got there, and that is the
 * invariant the whole plan screen rests on.
 *
 * Everything else follows **enrolment**, not coverage. Coverage alone painted
 * a month-2 unit as the current one because the reader had happened to add
 * three of its words — the learner had never opened it. Starting a unit is
 * something a person does, and only that reads as started.
 */
function dotClass(u: UnitState) {
  if (u.status === 'done') return 'bg-known'
  if (u.enrolled) return 'bg-primary'
  if (u.status === 'current') return 'bg-primary/40'
  return 'bg-muted-foreground/30'
}

/**
 * The fraction, and why it is not zero.
 *
 * «12/40» on a unit nobody has started is a true number that reads as a false
 * claim. `fromHere` is the part this unit actually added, so the row can say
 * the rest came from somewhere else.
 */
function elsewhere(u: UnitState) {
  const n = u.wordsInPlay - (u.fromHere ?? 0)
  return n > 0 ? n : 0
}

const title = (u: UnitState) => (ui.locale === 'ru' ? u.unit.titleRu : u.unit.titleEn)
</script>

<template>
  <div class="flex flex-col gap-5">
    <section v-for="g in months" :key="g.month" class="flex flex-col gap-1.5">
      <h3 class="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
        {{ $t('plan.month', { n: g.month }) }}
      </h3>
      <ul class="stagger-in flex flex-col gap-0.5">
        <li v-for="u in g.units" :key="u.unit.code">
          <button
            type="button"
            :class="
              cn(
                buttonVariants({ variant: 'ghost' }),
                'h-auto w-full justify-start gap-3 px-3 py-2 text-left font-normal',
                u.unit.code === selected && 'bg-accent text-foreground',
              )
            "
            :aria-current="u.unit.code === selected ? 'true' : undefined"
            @click="emit('select', u.unit.code)"
          >
            <span class="size-2 shrink-0 rounded-full" :class="dotClass(u)" aria-hidden="true" />
            <!-- The dot is colour alone; a screen reader gets the word instead. -->
            <span class="sr-only">
              {{
                u.status === 'done'
                  ? $t('plan.status.done')
                  : u.enrolled
                    ? $t('plan.status.started')
                    : u.status === 'current'
                      ? $t('plan.status.next')
                      : $t('plan.status.notStarted')
              }}
            </span>
            <span class="min-w-0 flex-1 truncate">{{ title(u) }}</span>
            <span
              class="shrink-0 font-mono text-xs tabular-nums text-muted-foreground"
              :title="
                elsewhere(u) > 0 && u.status !== 'done'
                  ? $t('plan.fromElsewhere', { n: elsewhere(u) })
                  : undefined
              "
            >
              {{ u.wordsInPlay }}/{{ u.wordsTotal }}
            </span>
          </button>
        </li>
      </ul>
    </section>
  </div>
</template>
