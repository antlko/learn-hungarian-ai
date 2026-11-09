import { onBeforeUnmount, ref, watch, type Ref } from 'vue'
import { MOTION, prefersReducedMotion } from '@/lib/motion'

/**
 * A number that counts to its value instead of appearing (ADR-059). ≤600ms, one
 * rAF loop, ease-out. Under reduced motion it jumps. The animated figure is
 * for the eye only: render `final` in an `sr-only` node (or aria-label) so a
 * screen reader hears the value once and not every intermediate frame.
 */
export function useCountUp(target: Ref<number>, duration: number = MOTION.countUp) {
  const display = ref(prefersReducedMotion() ? target.value : 0)
  let raf = 0

  function run(to: number) {
    cancelAnimationFrame(raf)
    const from = display.value
    if (prefersReducedMotion() || from === to || duration <= 0) {
      display.value = to
      return
    }
    const t0 = performance.now()
    const step = (now: number) => {
      const k = Math.min(1, (now - t0) / Math.min(duration, MOTION.countUp))
      const eased = 1 - Math.pow(1 - k, 3)
      display.value = Math.round(from + (to - from) * eased)
      if (k < 1) raf = requestAnimationFrame(step)
    }
    raf = requestAnimationFrame(step)
  }

  run(target.value)
  watch(target, run)
  onBeforeUnmount(() => cancelAnimationFrame(raf))
  return { display, final: target }
}
