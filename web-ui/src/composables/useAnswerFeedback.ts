import { computed, ref } from 'vue'
import { play } from '@/lib/sound'

/**
 * One verdict, two channels.
 *
 * [R1] The testing effect grows when feedback follows the retrieval attempt —
 * see .claude/research/01-retrieval-practice.md. Every screen where the learner
 * is graded (review, grammar drills, placement, the ear trainer) owes the same
 * response the moment the server's answer lands: a sound and a movement. This
 * exists so all four give the same one, rather than four near-misses.
 *
 * THE SOUND NEVER DELAYS THE UI. `signal` sets the reactive state first and
 * nothing here is awaited: `play()` builds oscillator nodes and returns, it
 * swallows its own failures, and it is a no-op when sound is off or the
 * platform has no WebAudio. A learner who answers and immediately presses Enter
 * for the next card is never waiting on audio.
 *
 * The movement is the two answer tokens from the design system, applied by
 * binding `flashClass` and `flashKey` on the element that carries the result —
 * the key is what makes a second identical verdict replay rather than sit
 * still, because CSS will not restart an animation on an element that never
 * changed.
 */
export type Verdict = 'correct' | 'wrong'

export function useAnswerFeedback() {
  const verdict = ref<Verdict | null>(null)
  /** Monotonic. Bind as `:key` so an unchanged verdict still re-animates. */
  const flashKey = ref(0)

  const flashClass = computed(() => {
    if (verdict.value === 'correct') return 'animate-answer-correct'
    if (verdict.value === 'wrong') return 'animate-answer-wrong'
    return ''
  })

  function signal(correct: boolean): void {
    verdict.value = correct ? 'correct' : 'wrong'
    flashKey.value += 1
    play(verdict.value)
  }

  /** Call when the next item is shown, so a stale tint cannot survive it. */
  function clear(): void {
    verdict.value = null
  }

  return { verdict, flashKey, flashClass, signal, clear }
}
