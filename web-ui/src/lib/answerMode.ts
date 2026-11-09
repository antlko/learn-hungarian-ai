/**
 * How a productive card (cloze / produce) is answered on THIS device: by typing,
 * or by picking one of the server's options. A per-device preference, like the
 * accent row (lib/accentKeys.ts) and the set size (lib/sets.ts).
 *
 * Default: options on a touch screen (`pointer: coarse`), typing otherwise.
 * Choosing is recognition and weaker than typing — docs/PEDAGOGY.md §2.10 — so
 * a keyboard device starts on the stronger one. The card falls back to typing
 * by itself when the server sent no options.
 */
import { ref, watch } from 'vue'

export type AnswerMode = 'type' | 'choose'

const KEY = 'tanulom.review.answerMode'

function fallback(): AnswerMode {
  try {
    return window.matchMedia('(pointer: coarse)').matches ? 'choose' : 'type'
  } catch {
    return 'type'
  }
}

function read(): AnswerMode {
  try {
    const v = localStorage.getItem(KEY)
    if (v === 'type' || v === 'choose') return v
  } catch {
    // Storage refused: use the device default.
  }
  return fallback()
}

export const answerMode = ref<AnswerMode>(read())

watch(answerMode, (v) => {
  try {
    localStorage.setItem(KEY, v)
  } catch {
    // Private-mode storage refusals keep the value for this session only.
  }
})
