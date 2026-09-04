/**
 * Durations in the units a learner reads: minutes now, days soon, months later.
 *
 * Used for the grade preview under the rating buttons («Снова 1 мин · Хорошо
 * 3 дн») and for the interval change after an answer. Returns a number and a
 * unit key rather than a string, so the caller can pluralise it in the UI
 * locale through `review.units.*`.
 */
export type IntervalUnit = 'min' | 'h' | 'd' | 'wk' | 'mo' | 'yr'

export interface Interval {
  n: number
  unit: IntervalUnit
}

/** The gap from `now` (ms) to an ISO instant, never below one minute. */
export function untilInterval(iso: string, now = Date.now()): Interval {
  const ms = Math.max(0, new Date(iso).getTime() - now)
  return fromMs(ms)
}

/** A whole number of days as an interval. */
export function daysInterval(days: number): Interval {
  return fromMs(Math.max(0, days) * 86_400_000)
}

function fromMs(ms: number): Interval {
  const min = ms / 60_000
  if (min < 60) return { n: Math.max(1, Math.round(min)), unit: 'min' }
  const h = min / 60
  if (h < 24) return { n: Math.round(h), unit: 'h' }
  const d = h / 24
  if (d < 14) return { n: Math.round(d), unit: 'd' }
  if (d < 61) return { n: Math.round(d / 7), unit: 'wk' }
  if (d < 365) return { n: Math.round(d / 30.4), unit: 'mo' }
  return { n: Math.round((d / 365) * 10) / 10, unit: 'yr' }
}
