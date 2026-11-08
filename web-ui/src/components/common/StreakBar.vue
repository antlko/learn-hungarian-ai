<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { DailyLedger } from '@/types'

/**
 * The week, as seven cells: which days counted and which did not.
 *
 * The flame was a bare number, and a learner could not tell from it what a day
 * requires, why today has not counted yet, or how the week went. Duolingo's
 * week strip is the right idea and it is the ONLY part of that design borrowed
 * here: the strip is a legible display of a rule the app already applies.
 *
 * **No freeze, no repair, no purchase.** Those are the forgiveness tokens of a
 * point economy, and an economy is precisely what [R14] and [R16 §3] rule out.
 * A missed day is a missed day; the strip says so and the app moves on.
 *
 * The rule, stated once and applied everywhere (stats.Day.Met):
 *
 *     the day counts ⟺ the queue was cleared AND the stated minutes were met
 *
 * Both halves are shown separately for today, because they fail independently
 * and "not met" without saying which half is the thing that made this
 * confusing in the first place.
 */
/**
 * `withDefaults`, not a bare optional: Vue casts an ABSENT Boolean prop to
 * `false` rather than leaving it `undefined`, so `weekStartsMonday?: boolean`
 * with no default made every caller Sunday-first without saying so — the strip
 * shipped starting on Sunday while the code read as Monday-first.
 */
const props = withDefaults(
  defineProps<{
    /** The ledger, newest first, straight from `/api/stats/overview`. */
    days: DailyLedger[]
    dailyMinutes: number
    /** Monday-first, matching most of Europe including Hungary. */
    weekStartsMonday?: boolean
  }>(),
  { weekStartsMonday: true },
)

const { d: fmtDate } = useI18n()

/** Local calendar day, formatted the way the ledger stores it. */
function key(date: Date) {
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

const week = computed(() => {
  const byDay = new Map(props.days.map((d) => [d.day, d]))
  const now = new Date()
  const todayKey = key(now)

  // Monday-first: JS weeks start on Sunday, so shift.
  const dow = now.getDay()
  const offset = props.weekStartsMonday ? (dow + 6) % 7 : dow

  return Array.from({ length: 7 }, (_, i) => {
    const date = new Date(now)
    date.setDate(now.getDate() - offset + i)
    const k = key(date)
    const row = byDay.get(k)
    const met = !!row && row.dueLeft === 0 && row.activeMs >= props.dailyMinutes * 60000
    return {
      key: k,
      met,
      today: k === todayKey,
      // A day in the future has not been missed — it has not happened. Drawn
      // as neither met nor failed, because an empty Friday on Tuesday is not
      // a gap in the learner's week.
      future: k > todayKey,
      label: fmtDate(date, { weekday: 'narrow' }),
      title: fmtDate(date, { month: 'short', day: 'numeric' }),
    }
  })
})
</script>

<template>
  <div class="flex items-end gap-1">
    <div v-for="d in week" :key="d.key" class="flex flex-col items-center gap-1">
      <span class="text-[9px] leading-none text-muted-foreground uppercase">{{ d.label }}</span>
      <span
        class="size-4 rounded-[4px] border"
        :class="[
          d.met
            ? 'border-known bg-known'
            : d.future
              ? 'border-border-subtle bg-transparent'
              : 'border-border bg-muted',
          d.today && 'ring-2 ring-ring ring-offset-1 ring-offset-popover',
        ]"
        :title="`${d.title} — ${
            d.met
              ? $t('progress.chart.met')
              : d.future
                ? $t('today.streak.future')
                : $t('progress.chart.unmet')
          }`"
      />
    </div>
  </div>
</template>
