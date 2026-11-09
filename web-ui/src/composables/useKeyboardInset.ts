import { onMounted, onUnmounted, ref } from 'vue'

/**
 * How much of the bottom of the screen an on-screen keyboard covers, in px.
 *
 * On iOS Safari and on Android Chrome in its default mode the keyboard shrinks
 * the VISUAL viewport and leaves the layout viewport alone, so a
 * `position: fixed; bottom: 0` bar ends up under the keyboard. The inset is the
 * gap between the two; the caller lifts the bar by it. Where the layout
 * viewport shrinks with the keyboard (older Android) both values fall together
 * and the inset is 0, so nothing is lifted twice. Under 80px is browser chrome
 * (the URL bar collapsing), not a keyboard, and reads as 0.
 *
 * `height` is the visible height, for capping a sheet so its top stays on screen.
 */
export function useKeyboardInset() {
  const inset = ref(0)
  const height = ref(0)
  const vv = typeof window !== 'undefined' ? window.visualViewport : null

  function measure() {
    if (!vv) return
    const gap = window.innerHeight - vv.height - vv.offsetTop
    inset.value = gap > 80 ? Math.round(gap) : 0
    height.value = Math.round(vv.height)
  }

  onMounted(() => {
    measure()
    vv?.addEventListener('resize', measure)
    vv?.addEventListener('scroll', measure)
  })
  onUnmounted(() => {
    vv?.removeEventListener('resize', measure)
    vv?.removeEventListener('scroll', measure)
  })
  return { inset, height }
}
