import { ref } from 'vue'
import { defineStore } from 'pinia'
import { api } from '@/api'
import type { DailyLedger } from '@/types'

/**
 * The shell's numbers: due now, new words waiting, the streak. One fetch shared
 * by the sidebar badge and the topbar chips, throttled and refreshed on
 * navigation so the chrome never goes stale enough to contradict the screen
 * under it. Every figure comes from the same Breakdown the review tabs show
 * (ADR-021) — chrome printing a different number than the queue is exactly the
 * confusion this app is not allowed to cause again.
 */
export const useDeckStore = defineStore('deck', () => {
  const dueNow = ref(0)
  const newNotes = ref(0)
  const streak = ref(0)
  const best = ref(0)
  const todayMet = ref(false)
  const loaded = ref(false)

  /**
   * The recent ledger and the target, so the streak can EXPLAIN itself.
   *
   * The flame used to be a bare number with a tooltip, and a learner could not
   * tell what a day required or which half of today was missing. The rule is
   * exactly `dueLeft === 0 AND activeMs >= dailyMinutes` (stats.Day.Met), and
   * showing it needs the rows — which this fetch already downloads and used to
   * throw away.
   */
  const days = ref<DailyLedger[]>([])
  const dailyMinutes = ref(90)
  const todayActiveMs = ref(0)
  const todayDueLeft = ref(0)

  let lastFetch = 0

  async function refresh(force = false) {
    if (!force && Date.now() - lastFetch < 10_000) return
    lastFetch = Date.now()
    try {
      const ov = await api.overview()
      dueNow.value = ov.queue?.dueNow ?? ov.dueNow ?? 0
      newNotes.value = ov.queue?.newNotes ?? 0
      streak.value = ov.streak.current
      best.value = ov.streak.best
      todayMet.value = ov.streak.todayMet
      days.value = ov.days ?? []
      dailyMinutes.value = ov.dailyMinutes
      todayActiveMs.value = ov.today?.activeMs ?? 0
      todayDueLeft.value = ov.queue?.dueNow ?? ov.dueNow ?? 0
      loaded.value = true
    } catch {
      // Chrome must never take a page down; a stale badge beats an error bar.
    }
  }

  return {
    dueNow,
    newNotes,
    streak,
    best,
    todayMet,
    days,
    dailyMinutes,
    todayActiveMs,
    todayDueLeft,
    loaded,
    refresh,
  }
})
