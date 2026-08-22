import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { api } from '@/api'
import type { AnswerResponse, QueueCounts, QueueItem } from '@/types'

/** What the card is showing right now. */
export type Phase = 'question' | 'answer'

/**
 * The review session.
 *
 * Timing starts when a card is *shown*, not when the store loads, because
 * latency is graded: it separates recall from reconstruction, and for a motor
 * skill like vowel harmony being correct-but-slow is not mastery.
 */
export const useReviewStore = defineStore('review', () => {
  const queue = ref<QueueItem[]>([])
  const counts = ref<QueueCounts>({ due: 0, new: 0, learning: 0, total: 0, newCapped: 0 })
  const index = ref(0)
  const phase = ref<Phase>('question')
  const typed = ref('')
  const usedHint = ref(false)
  const result = ref<AnswerResponse | null>(null)
  const loading = ref(false)
  const submitting = ref(false)
  const error = ref('')

  let shownAt = 0

  const current = computed<QueueItem | null>(() => queue.value[index.value] ?? null)
  const remaining = computed(() => Math.max(0, queue.value.length - index.value))
  const done = computed(() => !loading.value && current.value === null)

  /** Productive directions are typed and graded by the server. */
  const isProductive = computed(() => {
    const d = current.value?.direction
    return d === 'produce' || d === 'cloze' || d === 'spell'
  })

  /** The sentence with the target span replaced by a blank, for cloze. */
  const clozeText = computed(() => {
    const c = current.value
    if (!c || c.direction !== 'cloze' || c.targetStart < 0) return c?.hu ?? ''
    return c.hu.slice(0, c.targetStart) + '_____' + c.hu.slice(c.targetEnd)
  })

  async function load() {
    loading.value = true
    error.value = ''
    try {
      const res = await api.queue()
      queue.value = res.items
      counts.value = res.counts
      index.value = 0
      resetCard()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'failed'
      queue.value = []
    } finally {
      loading.value = false
    }
  }

  function resetCard() {
    phase.value = 'question'
    typed.value = ''
    usedHint.value = false
    result.value = null
    shownAt = performance.now()
  }

  /** Reveal the answer. For receptive cards this is the whole interaction. */
  function reveal() {
    if (phase.value === 'answer') return
    phase.value = 'answer'
  }

  /** Show the answer without grading it — this caps the rating at Hard. */
  function hint() {
    usedHint.value = true
    reveal()
  }

  async function submit(rating?: number) {
    const card = current.value
    if (!card || submitting.value) return
    submitting.value = true
    error.value = ''
    try {
      const res = await api.answer({
        cardId: card.cardId,
        rating,
        typed: isProductive.value ? typed.value : undefined,
        usedHint: usedHint.value,
        durationMs: Math.round(performance.now() - shownAt),
      })
      result.value = res
      phase.value = 'answer'
      return res
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'failed'
    } finally {
      submitting.value = false
    }
  }

  function next() {
    index.value += 1
    resetCard()
  }

  return {
    queue,
    counts,
    index,
    phase,
    typed,
    usedHint,
    result,
    loading,
    submitting,
    error,
    current,
    remaining,
    done,
    isProductive,
    clozeText,
    load,
    reveal,
    hint,
    submit,
    next,
    resetCard,
  }
})
