/**
 * Small helpers of the «Письмо» screens (ADR-085).
 */

/** Words as the counter shows them: runs of non-space characters. */
export function countWords(text: string): number {
  const m = text.trim().match(/\S+/g)
  return m ? m.length : 0
}

/** The date of a check or an edit, short, in the UI locale. */
export function shortDate(iso: string, locale: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString(locale === 'ru' ? 'ru-RU' : 'en-GB', {
    day: 'numeric',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/**
 * Copies text to the clipboard. `false` when the browser has no clipboard (an
 * http page, an old webview) or refuses — the caller then selects the text so
 * the learner can press Ctrl+C.
 */
export async function copyText(text: string): Promise<boolean> {
  try {
    if (!navigator.clipboard?.writeText) return false
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    return false
  }
}

/** Selects all of an element's text (a textarea's value or a node's contents). */
export function selectAll(el: HTMLElement | null | undefined) {
  if (!el) return
  if (el instanceof HTMLTextAreaElement || el instanceof HTMLInputElement) {
    el.focus()
    el.select()
    return
  }
  const sel = window.getSelection()
  if (!sel) return
  const range = document.createRange()
  range.selectNodeContents(el)
  sel.removeAllRanges()
  sel.addRange(range)
}
