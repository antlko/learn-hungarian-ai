<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Table2, LayoutGrid } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import type { DailyLedger } from '@/types'

/**
 * Activity by day, as a calendar: one column per week, one cell per day.
 *
 * Shade = MINUTES spent against the learner's own daily target, in four steps
 * with a legend — never points. Colour is not the only carrier: each cell's
 * accessible name states the day and its minutes, the detail below prints the
 * split by loop, and a table lists the same days. Nothing here accumulates, and
 * there is no streak art; whether a day counted is a state, named in words,
 * with the half that failed (queue or minutes) when it did not
 * (stats.Day.Met: `dueLeft === 0 AND minutes >= target`).
 */
const props = defineProps<{ days: DailyLedger[]; dailyMinutes: number }>()
const { t } = useI18n()

const WEEKS = 26
const asTable = ref(false)

const byDay = computed(() => new Map(props.days.map((d) => [d.day, d])))

function key(d: Date) {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

interface Cell {
  day: string
  future: boolean
  noData: boolean
  total: number
  cards: number
  reading: number
  listening: number
  metQueue: boolean
  metMinutes: boolean
  dueLeft: number
  level: 0 | 1 | 2 | 3 | 4
}

function build(date: Date, today: Date): Cell {
  const k = key(date)
  const d = byDay.value.get(k)
  const future = date > today
  if (!d) {
    return { day: k, future, noData: true, total: 0, cards: 0, reading: 0, listening: 0, metQueue: false, metMinutes: false, dueLeft: 0, level: 0 }
  }
  const listening = d.listenS / 60
  const reading = (d.readS ?? 0) / 60
  const total = d.activeMs / 60000
  const goal = props.dailyMinutes || 1
  const ratio = total / goal
  return {
    day: k,
    future,
    noData: false,
    total,
    cards: Math.max(0, total - listening - reading),
    reading,
    listening,
    metQueue: d.dueLeft === 0,
    metMinutes: d.activeMs >= props.dailyMinutes * 60000,
    dueLeft: d.dueLeft,
    level: total <= 0 ? 0 : ratio < 0.5 ? 1 : ratio < 1 ? 2 : ratio < 2 ? 3 : 4,
  }
}

/** Columns of 7 (Mon..Sun), oldest week first. */
const weeks = computed(() => {
  const today = new Date()
  today.setHours(12, 0, 0, 0)
  const dow = (today.getDay() + 6) % 7 // Monday = 0
  const start = new Date(today)
  start.setDate(today.getDate() - dow - (WEEKS - 1) * 7)
  const out: Cell[][] = []
  for (let w = 0; w < WEEKS; w++) {
    const col: Cell[] = []
    for (let i = 0; i < 7; i++) {
      const d = new Date(start)
      d.setDate(start.getDate() + w * 7 + i)
      col.push(build(d, today))
    }
    out.push(col)
  }
  return out
})

const flat = computed(() => weeks.value.flat())
const todayKey = key(new Date())
const selected = ref<string>(todayKey)
const detail = computed(() => flat.value.find((c) => c.day === selected.value) ?? null)

const SHADE = [
  'bg-muted',
  'bg-tint-mist-edge',
  'bg-ch-cards/50',
  'bg-ch-cards/75',
  'bg-ch-cards',
]

const mins = (v: number) => Math.round(v)

function label(c: Cell) {
  if (c.future) return c.day
  if (c.noData || c.total <= 0) return `${c.day}: ${t('progress.chart.nothing')}`
  return `${c.day}: ${t('progress.calendar.minutes', { n: mins(c.total) })}`
}

function unmet(c: Cell) {
  if (!c.metQueue && !c.metMinutes)
    return t('progress.chart.unmetBoth', { n: c.dueLeft, done: mins(c.total), goal: props.dailyMinutes })
  if (!c.metQueue) return t('progress.chart.unmetQueue', { n: c.dueLeft })
  return t('progress.chart.unmetMinutes', { done: mins(c.total), goal: props.dailyMinutes })
}

async function select(day: string, focus = false) {
  selected.value = day
  if (focus) {
    await nextTick()
    document.getElementById(`cal-${day}`)?.focus()
  }
}

function onKey(e: KeyboardEvent, c: Cell) {
  const idx = flat.value.findIndex((x) => x.day === c.day)
  const delta: Record<string, number> = { ArrowUp: -1, ArrowDown: 1, ArrowLeft: -7, ArrowRight: 7 }
  if (!(e.key in delta)) return
  e.preventDefault()
  const next = flat.value[idx + delta[e.key]]
  if (next && !next.future) select(next.day, true)
}

const rowsWithData = computed(() =>
  flat.value.filter((c) => !c.noData && !c.future).reverse().slice(0, 60),
)
const hasAny = computed(() => flat.value.some((c) => c.total > 0))
</script>

<template>
  <div class="flex flex-col gap-3">
    <div class="flex items-center gap-2">
      <p class="text-[11px] leading-relaxed text-muted-foreground">
        {{ $t('progress.calendar.lead', { n: dailyMinutes }) }}
      </p>
      <Button size="sm" variant="ghost" class="ml-auto shrink-0" @click="asTable = !asTable">
        <component :is="asTable ? LayoutGrid : Table2" class="size-4" aria-hidden="true" />
        {{ asTable ? $t('progress.calendar.asCalendar') : $t('progress.chart.asTable') }}
      </Button>
    </div>

    <div v-if="!asTable" class="overflow-x-auto pb-1">
      <div
        class="flex w-max gap-[3px]"
        role="group"
        :aria-label="$t('progress.calendar.alt')"
      >
        <div v-for="(col, wi) in weeks" :key="wi" class="flex flex-col gap-[3px]">
          <button
            v-for="c in col"
            :id="`cal-${c.day}`"
            :key="c.day"
            type="button"
            :disabled="c.future"
            :tabindex="c.day === selected ? 0 : -1"
            :aria-label="label(c)"
            :aria-pressed="c.day === selected"
            class="size-[11px] rounded-[3px] focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring sm:size-3.5"
            :class="[
              c.future ? 'opacity-0' : SHADE[c.level],
              c.day === selected ? 'ring-2 ring-foreground' : '',
            ]"
            @click="select(c.day)"
            @keydown="onKey($event, c)"
          />
        </div>
      </div>
    </div>

    <div v-if="!asTable" class="flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
      <span>{{ $t('progress.calendar.less') }}</span>
      <span v-for="(s, i) in SHADE" :key="i" class="size-3 rounded-[3px]" :class="s" aria-hidden="true" />
      <span>{{ $t('progress.calendar.more') }}</span>
    </div>

    <div v-if="!asTable && detail" class="rounded-lg border border-tint-cream-edge bg-tint-cream p-3 text-sm" aria-live="polite">
      <p class="font-medium tabular-nums">{{ detail.day }}</p>
      <p v-if="detail.noData || detail.total <= 0" class="text-muted-foreground">
        {{ $t('progress.chart.nothing') }}
      </p>
      <template v-else>
        <ul class="mt-1 flex flex-col gap-0.5 text-xs tabular-nums">
          <li class="flex justify-between">
            <span class="text-muted-foreground">{{ $t('progress.chart.series.cards') }}</span>
            <span>{{ $t('progress.calendar.minutes', { n: mins(detail.cards) }) }}</span>
          </li>
          <li class="flex justify-between">
            <span class="text-muted-foreground">{{ $t('progress.chart.series.reading') }}</span>
            <span>{{ $t('progress.calendar.minutes', { n: mins(detail.reading) }) }}</span>
          </li>
          <li class="flex justify-between">
            <span class="text-muted-foreground">{{ $t('progress.chart.series.listening') }}</span>
            <span>{{ $t('progress.calendar.minutes', { n: mins(detail.listening) }) }}</span>
          </li>
        </ul>
        <p class="mt-2 text-xs" :class="detail.metQueue && detail.metMinutes ? 'text-known' : 'text-muted-foreground'">
          {{ detail.metQueue && detail.metMinutes ? $t('progress.chart.normMet') : unmet(detail) }}
        </p>
      </template>
    </div>

    <div v-if="asTable" class="max-h-64 overflow-auto rounded-md border border-border-subtle">
      <table class="w-full text-xs tabular-nums">
        <thead class="sticky top-0 bg-background text-left text-muted-foreground">
          <tr>
            <th class="px-2 py-1 font-medium">{{ $t('progress.chart.day') }}</th>
            <th class="px-2 py-1 text-right font-medium">{{ $t('progress.chart.total') }}</th>
            <th class="px-2 py-1 font-medium">{{ $t('progress.chart.norm') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in rowsWithData" :key="c.day" class="border-t border-border-subtle">
            <td class="px-2 py-1">{{ c.day }}</td>
            <td class="px-2 py-1 text-right">{{ $t('progress.calendar.minutes', { n: mins(c.total) }) }}</td>
            <td class="px-2 py-1">
              {{ c.metQueue && c.metMinutes ? $t('progress.chart.met') : $t('progress.chart.unmet') }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <p v-if="!hasAny" class="text-xs text-muted-foreground">{{ $t('progress.chart.empty') }}</p>
  </div>
</template>
