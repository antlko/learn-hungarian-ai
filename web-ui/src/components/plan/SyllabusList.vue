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

const dots: Record<UnitState['status'], string> = {
  done: 'bg-known',
  current: 'bg-primary',
  ahead: 'bg-muted-foreground/30',
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
            <span class="size-2 shrink-0 rounded-full" :class="dots[u.status]" aria-hidden="true" />
            <!-- The dot is colour alone; a screen reader gets the word instead. -->
            <span class="sr-only">{{ $t(`plan.status.${u.status}`) }}</span>
            <span class="min-w-0 flex-1 truncate">{{ title(u) }}</span>
            <span class="shrink-0 font-mono text-xs tabular-nums text-muted-foreground">
              {{ u.wordsInPlay }}/{{ u.wordsTotal }}
            </span>
          </button>
        </li>
      </ul>
    </section>
  </div>
</template>
