import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { describeError } from '@/lib/errors'
import { api } from '@/api'
import { reviewOrder, setSize } from '@/lib/sets'
import type {
  AnswerResponse,
  Breakdown,
  QueueCounts,
  QueueItem,
  QueueScope,
} from '@/types'

/** What the card is showing right now. */
export type Phase = 'question' | 'answer'

/**
 * A card answered wrong, for the learner to read over again.
 *
 * The set summary and the done screen list these with their answers — the
 * one "other way to recall" a miss gets in this sitting. The card itself is
 * NOT re-served here: it comes back through the server's queue once the
 * learning step FSRS gave it has elapsed (ADR-026, amended 2026-09-03).
 */
export interface Missed {
  cardId: number
  noteId: number
  /** The graded Hungarian: the dictionary form on `produce`, the sentence form on `cloze`. */
  hu: string
  glossRu: string
  glossEn: string
}

/** What the set has done so far — measured, never invented. [R18 §4] */
interface SetStats {
  answered: number
  right: number
  missed: Missed[]
  /** Words that reached the mature stage in this set, by dictionary form. */
  matured: string[]
  activeMs: number
}

const emptySet = (): SetStats => ({
  answered: 0,
  right: 0,
  missed: [],
  matured: [],
  activeMs: 0,
})

/**
 * The review session.
 *
 * Timing starts when a card is *shown*, not when the store loads, because
 * latency is graded: it separates recall from reconstruction, and for a motor
 * skill like vowel harmony being correct-but-slow is not mastery.
 *
 * The queue the server serves is dealt out in SETS of `setSize` cards
 * (ADR-026). The server knows nothing about sets: `Breakdowns`, the counts and
 * the streak are exactly what they were, and a set is how this store hands the
 * day's list to the screen — a near finish line the learner can see [R18 §2].
 *
 * A set is always the HEAD of the served list. When one finishes, the list is
 * fetched again before the summary is shown, so a card missed a few minutes
 * ago — its learning step now elapsed — is in the next set because the
 * scheduler put it there, not because this store kept a retry list of its
 * own. Nothing is asked early, and the done screen reads fresh counts.
 */
export const useReviewStore = defineStore('review', () => {
  const queue = ref<QueueItem[]>([])
  const counts = ref<QueueCounts>({
    due: 0,
    new: 0,
    total: 0,
    newShownNotes: 0,
    newDeferredNotes: 0,
    newBacklogNotes: 0,
    introducedToday: 0,
    newPerDay: 0,
    newBudgetLeft: 0,
    dueLaterToday: 0,
  })
  /**
   * Which pile the session is drawn from — or `text:<id>`, the words mined
   * from one text (a sub-slice of the reader pile, ADR-022).
   *
   * A filter over one scheduler, never a second one: the daily new-card budget
   * is the whole deck's, so switching piles cannot buy more new material.
   */
  const scope = ref<QueueScope>('all')
  const scopes = ref<Breakdown[]>([])

  /**
   * The finished-session summary, accumulated as answers land. A competence
   * signal, not a score: what was answered, how much of it was right, how many
   * new words were started, the minutes it took — and which words were missed,
   * so the done screen can put them in front of the learner once more. Reset
   * whenever a session (review or practice) starts.
   */
  const session = ref({ answered: 0, correct: 0, newSeen: 0, activeMs: 0, missed: [] as Missed[] })

  /**
   * Review, or free practice.
   *
   * A MODE, not a fourth pile. Practice draws at random and schedules nothing:
   * no due date moves, no feature is graded, and no word gets closer to «знаю».
   * The card looks the same because it is the same question — what differs is
   * what the answer is allowed to change. See ADR-019.
   */
  const mode = ref<'review' | 'practice'>('review')
  /** Which pile practice draws from; independent of the review scope tabs. */
  const practiceScope = ref<QueueScope>('all')
  /** How many words the practice pile holds at all, for the empty state. */
  const practicePool = ref(0)
  /**
   * The round's shape: length, and the selection the learner asked for.
   * `practiceServedMode` echoes what the server actually applied — weak falls
   * back to random on a deck with nothing reviewed, and the screen must print
   * the criterion that was really used, not the one that was requested.
   * ADR-024: the criterion is the learner's explicit, visible choice.
   */
  const practiceSize = ref(20)
  const practiceMode = ref<'random' | 'weak'>('random')
  const practiceServedMode = ref<'random' | 'weak'>('random')

  // ── sets ──────────────────────────────────────────────────────────────
  /** Position within the set in play. */
  const pos = ref(0)
  const setStats = ref<SetStats>(emptySet())
  /** Sets finished this sitting — survives the between-set refresh, so the label keeps counting. */
  const setsDone = ref(0)
  /**
   * A set has just finished and its summary is (or is about to be) on screen.
   * While it is true nothing can be answered, undone or trashed: the last card
   * stays visible only until the refreshed queue lands.
   */
  const summaryOpen = ref(false)
  /** The between-set fetch is in flight. Distinct from `loading`, which empties the screen. */
  const refreshing = ref(false)

  /** Practice is one round, not a day's list: it is served whole. */
  const inSets = computed(() => mode.value === 'review')
  const size = computed(() => (inSets.value ? setSize.value : Math.max(1, queue.value.length)))
  /** The cards this set was dealt: always the head of the served list. */
  const base = computed(() => queue.value.slice(0, size.value))
  /** How many sets the served list makes, at the current size. */
  const setCount = computed(() => Math.ceil(queue.value.length / size.value))

  const index = ref(0)
  const phase = ref<Phase>('question')
  const typed = ref('')
  /**
   * The answer was SHOWN before it was graded — «Подсказка», and only that.
   * The tutor does not set it: asking a question never changes what a word's
   * answer schedules (ADR-046), and there is no longer any channel from the
   * tutor panel to this ref for a later change to quietly reconnect.
   */
  const usedHint = ref(false)
  const result = ref<AnswerResponse | null>(null)
  const loading = ref(false)
  const submitting = ref(false)
  const error = ref('')

  let shownAt = 0

  const current = computed<QueueItem | null>(() => {
    if (loading.value) return null
    return base.value[pos.value] ?? null
  })

  /** Between sets: the summary is on screen and the refreshed list has more in it. */
  const betweenSets = computed(
    () => summaryOpen.value && !refreshing.value && queue.value.length > 0,
  )
  /** Nothing left: the served list is empty, or the practice round is exhausted. */
  const done = computed(() => {
    if (loading.value || refreshing.value) return false
    if (queue.value.length === 0) return true
    return !inSets.value && pos.value >= base.value.length
  })
  /** Cards on the served list not yet answered in this sitting. */
  const remaining = computed(() => Math.max(0, queue.value.length - pos.value))

  /**
   * The set's own progress: cards answered, over the set's cards. While the
   * summary is pending the last card is still on screen, and the bar is full.
   */
  const setProgress = computed(() => ({
    done: summaryOpen.value ? base.value.length : pos.value,
    total: base.value.length,
  }))

  /** Productive directions are typed and graded by the server. */
  const isProductive = computed(() => {
    const d = current.value?.direction
    return d === 'produce' || d === 'cloze' || d === 'spell'
  })

  /**
   * Where the target word sits in the sentence.
   *
   * The split arrives from the server. It used to be done here by slicing `hu`
   * with the stored offsets, which are byte offsets into UTF-8 — one accented
   * letter before the target and the blank landed mid-word (`Jó éjszakát,
   * Timmy!` came out as `Jó é_____immy!`). Undefined when the note's target was
   * never located, in which case there is nothing to blank or highlight and the
   * sentence renders plain.
   *
   * Used by every direction, not just cloze: `recognize` underlines the target
   * so the word gloss beneath the card has something to point at.
   */
  const target = computed(() => current.value?.target)

  function resetSet() {
    pos.value = 0
    setStats.value = emptySet()
    summaryOpen.value = false
  }

  function applyQueue(res: Awaited<ReturnType<typeof api.queue>>) {
    queue.value = res.items
    counts.value = res.counts
    // Defaulted, not trusted. A response without these — an older server
    // behind a newer SPA — used to leave `scopes` undefined, and the tab
    // strip took the whole review screen down with it on a `.length`. A
    // missing field should cost the tabs, not the session.
    scopes.value = res.scopes ?? []
    scope.value = res.scope ?? scope.value
  }

  /**
   * Start a review session over `next`.
   *
   * Called by the card screen with the scope its URL asks for, and again by
   * its own reloads (a learning step maturing on the done screen, a tab
   * refocused). Nothing else starts a session: the choice of pile is made on
   * the start screen and travels in the query (ADR-028).
   */
  async function load(next?: QueueScope, opts?: { keepSession?: boolean }) {
    mode.value = 'review'
    if (next) scope.value = next
    loading.value = true
    error.value = ''
    // A reload that continues the same sitting — learning steps maturing on
    // the done screen, a change of set size — keeps the summary and the set
    // count; switching piles or arriving fresh starts a new one.
    if (!opts?.keepSession) {
      session.value = { answered: 0, correct: 0, newSeen: 0, activeMs: 0, missed: [] }
      setsDone.value = 0
    }
    // Undo reverts the LAST ANSWER, whatever pile it was in. Leaving this true
    // across a reload would offer to step back to a card that is no longer in
    // the queue: the server would revert an answer from the pile just left and
    // the client's step-back would silently not happen.
    canUndo.value = false
    try {
      applyQueue(await api.queue(scope.value, reviewOrder.value))
      index.value = 0
      resetSet()
      resetCard()
    } catch (e) {
      error.value = describeError(e)
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
    if (!card || submitting.value || summaryOpen.value) return
    submitting.value = true
    error.value = ''
    try {
      const body = {
        cardId: card.cardId,
        rating,
        typed: isProductive.value ? typed.value : undefined,
        usedHint: usedHint.value,
        durationMs: Math.round(performance.now() - shownAt),
      }
      if (mode.value === 'practice') {
        const p = await api.practiceAnswer(body)
        recordInSession(card, p.correct, p.expected, body.durationMs)
        // Widened to the review shape so one card component renders both. The
        // scheduling fields stay absent rather than zeroed: there is no next
        // due date, and a 0 would render as one.
        result.value = { ...p, rating: 0, gradedFeatures: [] } as unknown as AnswerResponse
        phase.value = 'answer'
        return result.value
      }
      const res = await api.answer(body)
      recordInSession(card, res.correct, res.expected, body.durationMs)
      recordInSet(card, res, body.durationMs)
      result.value = res
      phase.value = 'answer'
      canUndo.value = true
      return res
    } catch (e) {
      error.value = describeError(e)
    } finally {
      submitting.value = false
    }
  }

  /**
   * The miss, as the summaries print it: the string the card was graded
   * against (the server's, never derived here — ADR-017) and the word's gloss.
   */
  function missedEntry(card: QueueItem, expected?: string): Missed {
    return {
      cardId: card.cardId,
      noteId: card.noteId,
      hu: expected || card.expected || card.lemma || card.targetForm || card.hu,
      glossRu: card.glossRu,
      glossEn: card.glossEn,
    }
  }

  function recordInSession(card: QueueItem, correct: boolean, expected: string | undefined, durationMs: number) {
    const s = session.value
    s.answered += 1
    if (correct) s.correct += 1
    else if (!s.missed.some((m) => m.cardId === card.cardId)) s.missed.push(missedEntry(card, expected))
    if (card.isNew) s.newSeen += 1
    s.activeMs += durationMs
  }

  /**
   * The set's own accounting.
   *
   * A miss is listed, not re-served: its learning step comes back through the
   * server's queue — at the next set boundary if it has elapsed by then, and
   * through «сегодня вернутся» otherwise — exactly as a «Трудно» hit does.
   */
  function recordInSet(card: QueueItem, res: AnswerResponse, durationMs: number) {
    const s = setStats.value
    s.answered += 1
    s.activeMs += durationMs
    if (res.correct) s.right += 1
    else s.missed.push(missedEntry(card, res.expected))
    if (res.correct && card.memory?.stage !== 'mature' && res.memory?.stage === 'mature') {
      s.matured.push(card.lemma || card.targetForm || card.hu)
    }
  }

  /** Take one card's answer back out of both tallies, after an undo. */
  function unrecord(cardId: number) {
    const drop = (list: Missed[]) => {
      const i = list.findIndex((m) => m.cardId === cardId)
      if (i >= 0) list.splice(i, 1)
      return i >= 0
    }
    const s = setStats.value
    if (s.answered > 0) {
      s.answered -= 1
      if (!drop(s.missed)) s.right -= 1
    }
    const ss = session.value
    if (ss.answered > 0) {
      ss.answered -= 1
      if (!drop(ss.missed)) ss.correct -= 1
    }
  }

  /**
   * Move on. The last card of a set does not advance the pointer: it stays on
   * screen, answered, while the served list is fetched again, and the summary
   * replaces it when that lands. A second Enter in that gap is a no-op rather
   * than a second answer.
   */
  function next() {
    if (summaryOpen.value || !current.value) return
    if (inSets.value && pos.value + 1 >= base.value.length) {
      void finishSet()
      return
    }
    pos.value += 1
    index.value += 1
    resetCard()
  }

  /**
   * A set is over: count it, and re-fetch the served list before showing the
   * summary. The refresh is what brings a missed card back — the server serves
   * it once its step has elapsed and not before — and what makes the summary's
   * «осталось» and the done screen's countdown true rather than a snapshot
   * from the start of the sitting.
   */
  async function finishSet() {
    summaryOpen.value = true
    setsDone.value += 1
    // The answer just given is committed to the set; stepping back to it from
    // the summary would put a card the refreshed list no longer knows about
    // back on screen.
    canUndo.value = false
    await refresh()
  }

  async function refresh() {
    refreshing.value = true
    error.value = ''
    try {
      applyQueue(await api.queue(scope.value, reviewOrder.value))
      pos.value = 0
    } catch (e) {
      error.value = describeError(e)
      // Fall back to what was dealt: drop the set just answered rather than
      // serve it twice.
      queue.value = queue.value.slice(base.value.length)
      pos.value = 0
    } finally {
      refreshing.value = false
    }
  }

  /** Deal the next set — the head of the refreshed list. `space` from the summary. */
  function startNextSet() {
    if (!betweenSets.value) return
    resetSet()
    resetCard()
  }

  /**
   * Undo the last answer. Server-side delete-and-replay reverts BOTH state
   * spaces; the client only has to step back to the card it just left. Only
   * one step — the server reverts the latest answer, whoever's screen it was.
   */
  const canUndo = ref(false)

  async function undo() {
    if (!canUndo.value || submitting.value || summaryOpen.value) return
    submitting.value = true
    error.value = ''
    try {
      const res = await api.undo()
      canUndo.value = false
      // Step back to the undone card if it is the one behind us — the base
      // card one step back, still on its question — or the one still on
      // screen with its verdict showing. Either way its answer comes out of
      // the tallies, or a re-answer would count it twice.
      const prev = base.value[pos.value - 1]
      if (phase.value === 'question' && prev && prev.cardId === res.cardId) {
        pos.value -= 1
        unrecord(res.cardId)
      } else if (phase.value === 'answer' && current.value?.cardId === res.cardId) {
        unrecord(res.cardId)
      }
      resetCard()
    } catch (e) {
      error.value = describeError(e)
    } finally {
      submitting.value = false
    }
  }

  /**
   * Set the current word aside.
   *
   * «Слишком сложно» and «карточка плохая» are verdicts a learner is entitled
   * to reach, and the only previous answers were to fail the card forever or to
   * meet it at the top of every session. See ADR-025.
   *
   * Removes EVERY card of that note from the running session, not just the one
   * on screen: the learner set aside a word, and leaving its `produce` sibling
   * in the queue would hand the same word back through a different door.
   *
   * The tab strip is refreshed from the server's own Breakdowns rather than by
   * subtracting one here — ADR-021: every review number a learner sees comes
   * from one place, and a client that did its own arithmetic is how two figures
   * on one screen start disagreeing.
   */
  const marking = ref(false)

  async function knowCurrent() {
    const card = current.value
    if (!card || marking.value || summaryOpen.value) return
    marking.value = true
    error.value = ''
    try {
      const res = await api.knowNote(card.noteId, scope.value)
      if (res.scopes?.length) scopes.value = res.scopes
      // Undo reverts the last ANSWER; this was not one, and offering to step
      // back would revert something else entirely.
      canUndo.value = false
      const at = pos.value
      // Cards already served stay where they are; the word's cards ahead of
      // us leave, so the set shrinks rather than the pointer moving.
      queue.value = queue.value.filter((it, i) => i < at || it.noteId !== card.noteId)
      resetCard()
      // Marking the set's last card known ends the set the way answering it would.
      if (inSets.value && base.value.length > 0 && pos.value >= base.value.length) void finishSet()
      return res
    } catch (e) {
      error.value = describeError(e)
    } finally {
      marking.value = false
    }
  }

  /**
   * Put a word back, from the banner «Знаю» leaves behind.
   *
   * Deliberately does NOT reload the queue. The card is already gone from this
   * session's array, and rebuilding the queue would restart the sitting from
   * card one to re-insert a word the learner has just seen — the restore puts
   * it back in the DECK, and the next session serves it. The banner says so.
   */
  async function restoreKnown(noteId: number) {
    error.value = ''
    try {
      const res = await api.restoreNote(noteId, scope.value)
      if (res.scopes?.length) scopes.value = res.scopes
      return res
    } catch (e) {
      error.value = describeError(e)
    }
  }

  /** True while the session is filtered to one text's words. */
  const textScope = computed(() => String(scope.value).startsWith('text:'))

  /**
   * Start a practice round.
   *
   * Deliberately keeps `counts` untouched: those numbers describe what the deck
   * OWES, and a practice round owes nothing. Letting a session of free practice
   * rewrite them would be the screen quietly claiming progress it did not make.
   */
  async function loadPractice(next?: QueueScope) {
    if (next) practiceScope.value = next
    mode.value = 'practice'
    loading.value = true
    error.value = ''
    canUndo.value = false
    session.value = { answered: 0, correct: 0, newSeen: 0, activeMs: 0, missed: [] }
    setsDone.value = 0
    try {
      const res = await api.practiceQueue(practiceScope.value, practiceSize.value, practiceMode.value)
      queue.value = res.items
      practicePool.value = res.pool
      practiceScope.value = res.scope ?? practiceScope.value
      practiceServedMode.value = res.mode === 'weak' ? 'weak' : 'random'
      index.value = 0
      resetSet()
      resetCard()
    } catch (e) {
      error.value = describeError(e)
      queue.value = []
    } finally {
      loading.value = false
    }
  }

  return {
    scope,
    scopes,
    session,
    textScope,
    mode,
    practiceScope,
    practicePool,
    practiceSize,
    practiceMode,
    practiceServedMode,
    loadPractice,
    canUndo,
    undo,
    marking,
    knowCurrent,
    restoreKnown,
    queue,
    counts,
    index,
    phase,
    typed,
    usedHint,
    result,
    loading,
    refreshing,
    submitting,
    error,
    current,
    remaining,
    done,
    isProductive,
    target,
    load,
    reveal,
    hint,
    submit,
    next,
    resetCard,
    // sets
    inSets,
    base,
    pos,
    setCount,
    setsDone,
    setStats,
    setProgress,
    betweenSets,
    startNextSet,
  }
})
