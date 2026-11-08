<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Table2, BarChart3 } from '@lucide/vue'
import { Button, buttonVariants } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import type { DailyLedger } from '@/types'

/**
 * Where the days went: minutes per day, split by what they were spent on.
 *
 * Replaces the flat 30-bar strip on the Today screen, which drew `active_ms`
 * as one undifferentiated height and so could not answer the only question a
 * learner actually has about it — «куда ушло время».
 *
 * # What it draws, and what it refuses to
 *
 * Three stacked series, all of them minutes, all measured:
 *
 *   * **listening** — `listen_s`, clamped at 60 s per report on the server
 *   * **reading**   — `read_s`, the same clamp (ADR-044; before it, reading
 *                     reported nothing at all)
 *   * **cards**     — the remainder of `active_ms`. Reviews, free practice and
 *                     grammar drills are the only other writers of that column,
 *                     so the remainder is exactly those three. It is labelled
 *                     as what it is rather than as "review", because free
 *                     practice is in there and free practice schedules nothing.
 *
 * The dashed line is the learner's own `dailyMinutes` target. It is on the same
 * axis as the bars because it IS minutes — a threshold, not a second measure.
 *
 * **There is no second y-axis, and `due_left` is not a line on this chart.**
 * Cards-remaining and minutes-spent are different scales, and drawing them
 * against one axis invites exactly the comparison that is meaningless. The
 * queue half of the day's obligation is drawn instead as the status row under
 * the bars: met / not met, which is a state and not a magnitude.
 *
 * # Not a score
 *
 * Nothing here accumulates and nothing has a target except the one the learner
 * set. A tall day is not worth more than a short one, there is no total across
 * the window, and the window itself can be narrowed — all of which is what
 * keeps this a record of effort rather than the point economy [R14] and
 * [R16 §3] rule out.
 */
const props = withDefaults(
  defineProps<{
    /** Newest first, exactly as `/api/stats/overview` returns them. */
    days: DailyLedger[]
    /** The learner's stated minutes, for the target line. */
    dailyMinutes: number
    /** Which window to open on. A week reads at a glance; a month does not. */
    initialRange?: number
  }>(),
  { initialRange: 7 },
)

/**
 * The window, owned here and switched without a round trip.
 *
 * The page hands over everything the longest range needs (the server's
 * default is 90 rows of a few fields — nothing), so switching is instant and
 * neither screen needs refetch plumbing of its own.
 */
const RANGES = [7, 30, 90] as const
const range = ref<number>(props.initialRange)

const { t, d: fmtDate } = useI18n()

const SERIES = [
  { key: 'cards', fill: 'fill-ch-cards', swatch: 'bg-ch-cards' },
  { key: 'reading', fill: 'fill-ch-reading', swatch: 'bg-ch-reading' },
  { key: 'listening', fill: 'fill-ch-listening', swatch: 'bg-ch-listening' },
] as const

/** The ledger, by local calendar day, for lookups. */
const byDay = computed(() => new Map(props.days.map((d) => [d.day, d])))

/** A local calendar day, keyed the way `daily_log` stores it. */
function dayKey(date: Date) {
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

/**
 * Every day in the window, oldest-left — INCLUDING the ones with no row.
 *
 * `daily_log` only has rows for days something happened on, so mapping the
 * response straight to bars drew N bars for N rows and spaced them evenly
 * across a month. A fortnight of study then rendered as a solid block against
 * an empty left margin, with the axis still claiming a full month: the chart
 * silently closed up the gaps it was supposed to be showing.
 *
 * So the series is built from the CALENDAR and the ledger is looked up into
 * it. A day with no row is a real day on which nothing happened, and it gets
 * a slot and a mark that says so.
 */
const rows = computed(() => {
  const today = new Date()
  const out = []
  for (let i = range.value - 1; i >= 0; i--) {
    const date = new Date(today)
    date.setDate(today.getDate() - i)
    const key = dayKey(date)
    const d = byDay.value.get(key)
    if (!d) {
      // Nothing was recorded. `met` is false, but the two halves are UNKNOWN
      // rather than met — a missing row is not a cleared queue, and saying
      // «очередь пуста» about a day nobody opened the app would be inventing
      // a fact.
      out.push({
        day: key,
        cards: 0,
        reading: 0,
        listening: 0,
        total: 0,
        met: false,
        metQueue: false,
        metMinutes: false,
        dueLeft: 0,
        noData: true,
      })
      continue
    }
    const listening = d.listenS / 60
    const reading = (d.readS ?? 0) / 60
    // The remainder, floored at zero: the three columns are written by
    // different code paths in the same second, and a rounding disagreement
    // must not draw a negative segment.
    const cards = Math.max(0, d.activeMs / 60000 - listening - reading)
    // The day's obligation has TWO halves and they fail independently
    // (stats.Day.Met). Tracking them apart is what lets the tooltip name the
    // one that actually failed: «норма не выполнена — в очереди осталось 0»
    // on a day that cleared its queue and simply ran short of minutes is a
    // sentence that points at the wrong thing.
    const metQueue = d.dueLeft === 0
    const metMinutes = d.activeMs >= props.dailyMinutes * 60000
    out.push({
      day: key,
      cards,
      reading,
      listening,
      total: cards + reading + listening,
      met: metQueue && metMinutes,
      metQueue,
      metMinutes,
      dueLeft: d.dueLeft,
      noData: false,
    })
  }
  return out
})

/**
 * Whether anything at all has been recorded in this window.
 *
 * It no longer blanks the chart — a week of empty days IS the answer, and
 * drawing seven empty slots says it better than a sentence. It only decides
 * whether the «пока нечего показать» line appears beneath.
 */
const hasAny = computed(() => rows.value.some((r) => r.total > 0))

/**
 * The y scale: the tallest day, or the target, whichever is larger — so the
 * target line is always on the chart. Rounded up to a readable step.
 */
const peak = computed(() => {
  const highest = Math.max(...rows.value.map((r) => r.total), props.dailyMinutes, 1)
  const step = highest > 240 ? 60 : highest > 90 ? 30 : 15
  return Math.ceil(highest / step) * step
})

const W = 720
const H = 160
/** Room under the plot for the status row. */
const STATUS_H = 6
const GAP = 2
/**
 * Headroom above the plot for the topmost gridline's label, which sits at
 * y = 0 and would otherwise be clipped by the viewBox. Negative y is legal in
 * SVG; the viewBox simply has to include it.
 */
const PAD_TOP = 14

const geometry = computed(() => {
  const n = rows.value.length || 1
  const slot = W / n
  // Thin marks: a bar never fills its slot. Floored at 1px so a 365-day
  // window still draws something.
  const bar = Math.max(1, Math.min(slot - 1, slot * 0.7))
  return { slot, bar, n }
})

function y(minutes: number) {
  return H - (minutes / peak.value) * H
}

/** One day's stacked segments, bottom-up, with a 2px surface gap between. */
function segments(r: (typeof rows.value)[number]) {
  const out: { key: string; fill: string; y: number; h: number; top: boolean }[] = []
  let acc = 0
  const parts = [
    { key: 'cards', fill: 'fill-ch-cards', v: r.cards },
    { key: 'reading', fill: 'fill-ch-reading', v: r.reading },
    { key: 'listening', fill: 'fill-ch-listening', v: r.listening },
  ].filter((p) => p.v > 0)

  parts.forEach((p, i) => {
    const y0 = y(acc)
    const y1 = y(acc + p.v)
    // The gap eats from the TOP of each segment except the last, so the stack
    // still reaches the baseline and the total height stays honest.
    const last = i === parts.length - 1
    const h = Math.max(1, y0 - y1 - (last ? 0 : GAP))
    out.push({ key: p.key, fill: p.fill, y: y1 + (last ? 0 : GAP), h, top: last })
    acc += p.v
  })
  return out
}

const targetY = computed(() => y(props.dailyMinutes))

/** Gridlines at the target and at the peak, and nothing else. */
const ticks = computed(() => {
  const out = [{ minutes: peak.value, y: y(peak.value) }]
  if (props.dailyMinutes > 0 && props.dailyMinutes < peak.value) {
    out.push({ minutes: props.dailyMinutes, y: targetY.value })
  }
  return out
})

const hover = ref<number | null>(null)
const hovered = computed(() => (hover.value === null ? null : rows.value[hover.value] ?? null))

const asTable = ref(false)

function minutes(v: number) {
  return Math.round(v)
}

/**
 * Why a day did not count, naming the half that failed.
 *
 * The rule is `dueLeft === 0 AND minutes >= target` (stats.Day.Met), and the
 * two halves fail independently. A single "not met" line that always blamed
 * the queue told a learner who had cleared their queue that they had not.
 */
function unmetReason(r: (typeof rows.value)[number]) {
  if (!r.metQueue && !r.metMinutes) {
    return t('progress.chart.unmetBoth', {
      n: r.dueLeft,
      done: minutes(r.total),
      goal: props.dailyMinutes,
    })
  }
  if (!r.metQueue) return t('progress.chart.unmetQueue', { n: r.dueLeft })
  return t('progress.chart.unmetMinutes', { done: minutes(r.total), goal: props.dailyMinutes })
}

function dayLabel(iso: string) {
  // The stored day is a local calendar day, not an instant; parsing it as UTC
  // and formatting it back is what keeps it from sliding by one.
  const [Y, M, D] = iso.split('-').map(Number)
  if (!Y || !M || !D) return iso
  return fmtDate(new Date(Y, M - 1, D), { month: 'short', day: 'numeric' })
}

/** The first and last day, as the only x labels. A tick per day is noise. */
const xLabels = computed(() => {
  const r = rows.value
  if (r.length < 2) return null
  return { first: dayLabel(r[0]!.day), last: dayLabel(r[r.length - 1]!.day) }
})
</script>

<template>
  <div class="flex flex-col gap-3">
    <!-- Controls in one row above the chart. -->
    <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
      <p class="text-xs font-medium">{{ $t('progress.chart.title') }}</p>

      <div class="flex items-center gap-1">
        <button
          v-for="r in RANGES"
          :key="r"
          type="button"
          :class="
            cn(
              buttonVariants({ variant: r === range ? 'secondary' : 'ghost', size: 'sm' }),
              'h-6 px-2 text-[11px]',
            )
          "
          :aria-pressed="r === range"
          @click="range = r"
        >
          {{ $t('progress.chart.range', { n: r }) }}
        </button>
      </div>

      <Button
        variant="ghost"
        size="sm"
        class="ml-auto h-6 gap-1.5 px-2 text-[11px]"
        :aria-pressed="asTable"
        @click="asTable = !asTable"
      >
        <component :is="asTable ? BarChart3 : Table2" class="size-3.5" aria-hidden="true" />
        {{ asTable ? $t('progress.chart.asChart') : $t('progress.chart.asTable') }}
      </Button>
    </div>

    <!--
      The table view. Not a fallback — a peer: a chart is a picture of numbers
      and some people need the numbers. It is also what makes the colour
      encoding non-essential.
    -->
    <div v-if="asTable" class="max-h-72 overflow-auto rounded-lg border">
      <table class="w-full text-xs">
        <thead class="sticky top-0 bg-card">
          <tr class="border-b text-left text-muted-foreground">
            <th scope="col" class="px-3 py-1.5 font-medium">{{ $t('progress.chart.day') }}</th>
            <th
              v-for="sr in SERIES"
              :key="sr.key"
              scope="col"
              class="px-3 py-1.5 text-right font-medium"
            >
              {{ $t(`progress.chart.series.${sr.key}`) }}
            </th>
            <th scope="col" class="px-3 py-1.5 text-right font-medium">
              {{ $t('progress.chart.total') }}
            </th>
            <th scope="col" class="px-3 py-1.5 font-medium">{{ $t('progress.chart.norm') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in [...rows].reverse()" :key="r.day" class="border-b border-border-subtle">
            <td class="px-3 py-1 whitespace-nowrap">{{ dayLabel(r.day) }}</td>
            <td class="px-3 py-1 text-right font-mono tabular-nums">{{ minutes(r.cards) }}</td>
            <td class="px-3 py-1 text-right font-mono tabular-nums">{{ minutes(r.reading) }}</td>
            <td class="px-3 py-1 text-right font-mono tabular-nums">{{ minutes(r.listening) }}</td>
            <td class="px-3 py-1 text-right font-mono tabular-nums font-medium">
              {{ minutes(r.total) }}
            </td>
            <td class="px-3 py-1" :class="r.met ? 'text-known' : 'text-muted-foreground'">
              {{
                r.total <= 0
                  ? $t('progress.chart.nothing')
                  : r.met
                    ? $t('progress.chart.met')
                    : $t('progress.chart.unmet')
              }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <template v-else>
      <div class="relative">
        <svg
          :viewBox="`0 ${-PAD_TOP} ${W} ${H + STATUS_H + PAD_TOP + 2}`"
          class="h-40 w-full"
          role="img"
          :aria-label="$t('progress.chart.alt', { n: rows.length })"
          @pointerleave="hover = null"
        >
          <!-- Recessive gridlines, with the value on the line itself. -->
          <g v-for="tk in ticks" :key="tk.minutes">
            <line
              :x1="0"
              :x2="W"
              :y1="tk.y"
              :y2="tk.y"
              class="stroke-border-subtle"
              :stroke-dasharray="tk.minutes === dailyMinutes ? '4 4' : undefined"
            />
            <text :x="2" :y="tk.y - 3" class="fill-muted-foreground text-[10px]">
              {{ tk.minutes }} {{ $t('common.min') }}
            </text>
          </g>

          <!-- The bars. -->
          <g v-for="(r, i) in rows" :key="r.day">
            <!--
              A day with nothing on it. Drawn, not skipped: the whole point of
              a calendar axis is that a gap is visible, and an absent bar is
              indistinguishable from a bar too short to see.
            -->
            <rect
              v-if="r.total <= 0"
              :x="i * geometry.slot + (geometry.slot - geometry.bar) / 2"
              :y="H - 2"
              :width="geometry.bar"
              height="2"
              rx="1"
              class="fill-border"
            />
            <rect
              v-for="seg in segments(r)"
              :key="seg.key"
              :x="i * geometry.slot + (geometry.slot - geometry.bar) / 2"
              :y="seg.y"
              :width="geometry.bar"
              :height="seg.h"
              :rx="seg.top ? Math.min(4, geometry.bar / 2) : 0"
              :class="seg.fill"
            />
            <!-- The status row: the queue half of the obligation, as a state. -->
            <rect
              v-if="!r.noData"
              :x="i * geometry.slot + (geometry.slot - geometry.bar) / 2"
              :y="H + 2"
              :width="geometry.bar"
              :height="STATUS_H - 2"
              rx="1"
              :class="r.met ? 'fill-known' : 'fill-border'"
            />
            <!-- One transparent hit target per day, wider than the bar. -->
            <rect
              :x="i * geometry.slot"
              :y="0"
              :width="geometry.slot"
              :height="H + STATUS_H"
              fill="transparent"
              @pointerenter="hover = i"
            />
          </g>

          <!-- The hovered day, marked. -->
          <rect
            v-if="hover !== null"
            :x="hover * geometry.slot"
            :y="0"
            :width="geometry.slot"
            :height="H"
            class="fill-foreground/5"
            pointer-events="none"
          />
        </svg>

        <!-- The tooltip. Positioned in the flow rather than over the cursor:
             at 365 bars a cursor-following box covers the data it describes. -->
        <div
          v-if="hovered"
          class="pointer-events-none absolute top-0 right-0 flex flex-col gap-0.5 rounded-md border bg-popover px-2.5 py-1.5 text-[11px] shadow-raised"
        >
          <p class="font-medium">{{ dayLabel(hovered.day) }}</p>
          <!-- Three zeroes and a «норма не выполнена» is a lot of words for
               "nothing happened". -->
          <p v-if="hovered.total <= 0" class="text-muted-foreground">
            {{ $t('progress.chart.nothing') }}
          </p>
          <template v-else>
          <p v-for="sr in SERIES" :key="sr.key" class="flex items-center gap-1.5">
            <span class="size-2 shrink-0 rounded-sm" :class="sr.swatch" aria-hidden="true" />
            <span class="text-muted-foreground">{{
              $t(`progress.chart.series.${sr.key}`)
            }}</span>
            <span class="ml-auto font-mono tabular-nums">{{ minutes(hovered[sr.key]) }}</span>
          </p>
          <p class="mt-0.5 border-t border-border-subtle pt-0.5">
            {{ $t('progress.chart.total') }}:
            <span class="font-mono tabular-nums">{{ minutes(hovered.total) }}</span>
            {{ $t('common.min') }}
          </p>
          <p :class="hovered.met ? 'text-known' : 'text-muted-foreground'">
            {{ hovered.met ? $t('progress.chart.met') : unmetReason(hovered) }}
          </p>
          </template>
        </div>
      </div>

      <!-- x labels: the ends only. -->
      <div v-if="xLabels" class="flex justify-between text-[10px] text-muted-foreground">
        <span>{{ xLabels.first }}</span>
        <span>{{ xLabels.last }}</span>
      </div>

      <!-- A window in which nothing was recorded at all. The empty slots above
           already say it; this says why they are empty rather than broken. -->
      <p v-if="!hasAny" class="text-[11px] text-muted-foreground">
        {{ $t('progress.chart.empty') }}
      </p>

      <!-- The legend is always present: three series must never be identified
           by colour alone. -->
      <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-[11px] text-muted-foreground">
        <span v-for="sr in SERIES" :key="sr.key" class="flex items-center gap-1.5">
          <span class="size-2 rounded-sm" :class="sr.swatch" aria-hidden="true" />
          {{ $t(`progress.chart.series.${sr.key}`) }}
        </span>
        <span class="flex items-center gap-1.5">
          <span class="size-2 rounded-sm bg-known" aria-hidden="true" />
          {{ $t('progress.chart.normMet') }}
        </span>
      </div>
    </template>
  </div>
</template>
