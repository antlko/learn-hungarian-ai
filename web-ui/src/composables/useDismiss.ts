import { onUnmounted, watch, type Ref } from 'vue'

/**
 * Close a popover on a click outside `[selector]`, or on Escape. Listeners
 * exist only while it is open.
 */
export function useDismiss(open: Ref<boolean>, selector: string) {
  function onClick(e: MouseEvent) {
    if (!(e.target as HTMLElement | null)?.closest(selector)) open.value = false
  }
  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') open.value = false
  }
  function off() {
    document.removeEventListener('click', onClick, { capture: true })
    document.removeEventListener('keydown', onKey)
  }
  watch(open, (o) => {
    off()
    if (o) {
      document.addEventListener('click', onClick, { capture: true })
      document.addEventListener('keydown', onKey)
    }
  })
  onUnmounted(off)
}
