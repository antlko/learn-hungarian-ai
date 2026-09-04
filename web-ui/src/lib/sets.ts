/**
 * The review set size and order — per-device preferences, like sound and the
 * key strip.
 *
 * [R18] A near, visible finish line is one a learner starts and completes; a
 * 70-card queue is neither. The day's queue is dealt in sets of this many
 * cards (ADR-026). It is a WHIM, not a setting: settings are admin-only and
 * policy, and how long a sitting feels comfortable is neither — practice round
 * length already made that call (ADR-024). Stored beside the other device
 * preferences and read by the review screen and by Today, which prints the day
 * in sets from the same number.
 *
 * The server never sees it. The queue it serves is the same as before; sets
 * are how the client deals it out, so every review number (ADR-021) is
 * untouched.
 */
import { ref, watch } from 'vue'

export const SET_SIZES = [10, 15, 20, 30] as const
export type SetSize = (typeof SET_SIZES)[number]
export const DEFAULT_SET_SIZE: SetSize = 15

export type ReviewOrder = 'due' | 'random'

const SIZE_KEY = 'tanulom.review.setSize'
const ORDER_KEY = 'tanulom.review.order'

function readSize(): SetSize {
  try {
    const n = Number(localStorage.getItem(SIZE_KEY))
    return (SET_SIZES as readonly number[]).includes(n) ? (n as SetSize) : DEFAULT_SET_SIZE
  } catch {
    return DEFAULT_SET_SIZE
  }
}

function readOrder(): ReviewOrder {
  try {
    return localStorage.getItem(ORDER_KEY) === 'random' ? 'random' : 'due'
  } catch {
    return 'due'
  }
}

/** Module-level and reactive, so the review store and Today read one value. */
export const setSize = ref<SetSize>(readSize())
export const reviewOrder = ref<ReviewOrder>(readOrder())

watch(setSize, (n) => {
  try {
    localStorage.setItem(SIZE_KEY, String(n))
  } catch {
    // Private-mode storage refusals keep the value for this session only.
  }
})
watch(reviewOrder, (o) => {
  try {
    localStorage.setItem(ORDER_KEY, o)
  } catch {
    // As above.
  }
})

/** How many sets a queue of `cards` deals into at the current size. */
export function setsFor(cards: number): number {
  return cards <= 0 ? 0 : Math.ceil(cards / setSize.value)
}
