import { onMounted, onUnmounted } from 'vue'
import { api } from '@/api'

/**
 * Credits the seconds a text was actually audible to the day's listening total.
 *
 * Playback outside listening mode is still listening: the curriculum's word
 * gate reads `daily_log.listen_s`, so a learner who listens to a whole reading
 * text through the audio bar must move it, or the bar would quietly be worth
 * less than the screen it replaced. Reported as `pass: 0` (no pass, no saved
 * position). The server clamps one report to 60 s.
 *
 * Time is measured between `begin()` and `end()` on the wall clock — sound is
 * coming out only between them (not while buffering, not while paused) — and
 * banked every 15 s, on pause, when the tab is hidden and on unmount. A dropped
 * report loses seconds, never breaks the screen.
 */
export function useListenReporter(textId: () => number, flushEveryS = 15) {
  let startedAt: number | null = null
  let banked = 0
  // Captured at begin(): on unmount the route may already point at another text.
  let id = 0
  let timer: ReturnType<typeof setInterval> | undefined

  /** Move the running segment into the bank. */
  function bank() {
    if (startedAt === null) return
    const now = performance.now()
    banked += (now - startedAt) / 1000
    startedAt = now
  }

  async function flush() {
    bank()
    const whole = Math.floor(banked)
    if (whole < 1) return
    banked -= whole
    if (!id) return
    try {
      await api.listenProgress(id, { pass: 0, positionS: 0, secondsHeard: whole })
    } catch {
      // Lost seconds, not a lost screen.
    }
  }

  function begin() {
    if (startedAt !== null) return
    id = textId()
    startedAt = performance.now()
    timer ??= setInterval(() => void flush(), flushEveryS * 1000)
  }

  function end() {
    bank()
    startedAt = null
    clearInterval(timer)
    timer = undefined
    void flush()
  }

  function onVisibility() {
    if (document.visibilityState === 'hidden') void flush()
  }

  function onPageHide() {
    void flush()
  }

  onMounted(() => {
    document.addEventListener('visibilitychange', onVisibility)
    window.addEventListener('pagehide', onPageHide)
  })
  onUnmounted(() => {
    document.removeEventListener('visibilitychange', onVisibility)
    window.removeEventListener('pagehide', onPageHide)
    end()
  })

  return { begin, end, flush }
}
