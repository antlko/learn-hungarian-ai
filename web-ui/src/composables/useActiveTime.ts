import { onMounted, onUnmounted } from 'vue'

/**
 * Time the learner actually spent on a screen, reported to the server.
 *
 * Written for the reader, which was the one loop that reported NOTHING: an
 * hour with a text moved no counter, so «21 / 90 мин» sat still and the day's
 * obligation went unmet while the learner had been working. `active_ms` is one
 * half of what the streak requires, so reading was literally not counted as
 * study. ADR-044.
 *
 * # What "active" means here, and why it is narrow
 *
 * A wall clock would be dishonest — a tab left open overnight is not ten hours
 * of Hungarian, and a streak kept that way measures nothing ([R14]). So a
 * second counts only when BOTH hold:
 *
 *   - the document is visible (`document.visibilityState`), and
 *   - the learner has interacted within `idleAfterMs`.
 *
 * Reading is mostly still — eyes move, mice do not — so the idle window is
 * generous by design: a minute of quiet reading after a scroll still counts.
 * What it will not count is a tab someone walked away from.
 *
 * The server clamps every report to 60 s regardless (handlers_reading.go), so
 * a stuck timer here mints a minute and not a night. Both halves exist on
 * purpose: this side is honest, and the other side does not have to trust it.
 */
export interface ActiveTimeOptions {
  /** Posts `seconds` of credited time. Errors are the caller's to swallow. */
  report: (seconds: number) => Promise<unknown>
  /** No interaction for this long and the clock stops. Default 60 s. */
  idleAfterMs?: number
  /** How much to bank before reporting. Default 15 s — comfortably under the
   *  server's 60 s clamp, so a report is never truncated in normal use. */
  flushAfterSeconds?: number
  /** Whether the screen is in a state worth timing at all. Checked on every
   *  tick, so a view can stop the clock while it is showing a loading state or
   *  an error instead of content. */
  enabled?: () => boolean
}

const INTERACTIONS = ['keydown', 'pointerdown', 'wheel', 'scroll', 'touchstart'] as const

export function useActiveTime(opts: ActiveTimeOptions) {
  const idleAfterMs = opts.idleAfterMs ?? 60_000
  const flushAfter = opts.flushAfterSeconds ?? 15

  let lastInteraction = Date.now()
  let banked = 0
  let timer: ReturnType<typeof setInterval> | undefined

  function touch() {
    lastInteraction = Date.now()
  }

  function active() {
    if (typeof document !== 'undefined' && document.visibilityState !== 'visible') return false
    if (opts.enabled && !opts.enabled()) return false
    return Date.now() - lastInteraction < idleAfterMs
  }

  async function flush() {
    if (banked <= 0) return
    const seconds = banked
    // Zeroed BEFORE the await: a slow report must not double-count the
    // seconds that accumulate while it is in flight.
    banked = 0
    try {
      await opts.report(seconds)
    } catch {
      // A dropped report is a lost minute, not a broken screen. Reading is not
      // the place to surface a network error, and the next flush carries on.
    }
  }

  function tick() {
    if (!active()) return
    banked += 1
    if (banked >= flushAfter) void flush()
  }

  onMounted(() => {
    for (const ev of INTERACTIONS) {
      window.addEventListener(ev, touch, { passive: true })
    }
    // A tab coming back to the front is an interaction: the learner is here.
    document.addEventListener('visibilitychange', touch)
    timer = setInterval(tick, 1000)
  })

  onUnmounted(() => {
    for (const ev of INTERACTIONS) window.removeEventListener(ev, touch)
    document.removeEventListener('visibilitychange', touch)
    if (timer) clearInterval(timer)
    // Leaving the screen banks whatever is left, so closing a text after ten
    // seconds does not silently discard them.
    void flush()
  })

  return { flush }
}
