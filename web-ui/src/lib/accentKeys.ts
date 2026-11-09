/**
 * Whether the review screen shows the row of á é í ó ö ő ú ü ű buttons — a
 * per-device preference, like sound and the set size (lib/sets.ts).
 *
 * OFF by default: answers are not case-sensitive and diacritics are not
 * required (settings → «Принимать ответ без диакритики», on by default), so a
 * plain Latin phone keyboard is enough. The row is for the learner who wants to
 * write the exact spelling and has no long-press for ő and ű.
 */
import { ref, watch } from 'vue'

const KEY = 'tanulom.review.accentKeys'

function read(): boolean {
  try {
    return localStorage.getItem(KEY) === 'on'
  } catch {
    return false
  }
}

export const showAccentKeys = ref<boolean>(read())

watch(showAccentKeys, (v) => {
  try {
    localStorage.setItem(KEY, v ? 'on' : 'off')
  } catch {
    // Private-mode storage refusals keep the value for this session only.
  }
})
