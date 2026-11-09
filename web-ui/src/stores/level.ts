import { ref } from 'vue'
import { defineStore } from 'pinia'
import { api } from '@/api'
import type { LevelEstimate } from '@/types'

/**
 * The estimated level (ADR-047), fetched for the top bar's chip.
 *
 * One fetch per session, then at most one per minute: the shell asks again
 * when an answer lands or the learner returns to Today, and the throttle keeps
 * that cheap. No polling. A failure leaves `level` as it was (null on a first
 * failure) and the chip simply does not draw — chrome never raises an alarm.
 */
const MIN_GAP_MS = 60_000

export const useLevelStore = defineStore('level', () => {
  const level = ref<LevelEstimate | null>(null)
  const loading = ref(false)
  let lastFetch = 0

  async function refresh(force = false) {
    if (loading.value) return
    if (!force && Date.now() - lastFetch < MIN_GAP_MS) return
    lastFetch = Date.now()
    loading.value = true
    try {
      level.value = await api.level()
    } catch {
      // Best-effort: keep whatever we had.
    } finally {
      loading.value = false
    }
  }

  return { level, loading, refresh }
})
