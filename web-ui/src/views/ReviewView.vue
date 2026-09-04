<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, Clock, GraduationCap, MoonStar, Trash2, Undo2 } from '@lucide/vue'
import { onBeforeRouteUpdate, useRoute, useRouter, type LocationQuery } from 'vue-router'
import { Button, buttonVariants } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import KeyBar, { type KeyHint } from '@/components/common/KeyBar.vue'
import Note from '@/components/common/Note.vue'
import HungarianText from '@/components/common/HungarianText.vue'
import SpeakButton from '@/components/common/SpeakButton.vue'
import WordInspector from '@/components/common/WordInspector.vue'
import FeatureChips from '@/components/review/FeatureChips.vue'
import MemoryMeter from '@/components/review/MemoryMeter.vue'
import MissedList from '@/components/review/MissedList.vue'
import MorphemeDiff from '@/components/review/MorphemeDiff.vue'
import SentenceCrib from '@/components/review/SentenceCrib.vue'
import SetSummary from '@/components/review/SetSummary.vue'
import TutorAside from '@/components/review/TutorAside.vue'
import { untilInterval } from '@/lib/interval'
import { useAnswerFeedback } from '@/composables/useAnswerFeedback'
import { pickGloss } from '@/lib/gloss'
import { play } from '@/lib/sound'
import { useReviewStore } from '@/stores/review'
import { useCapabilitiesStore } from '@/stores/capabilities'
import { useFeaturesStore } from '@/stores/features'
import { useSpeechStore } from '@/stores/speech'
import { Illustration } from '@/components/art'
import { useUiStore } from '@/stores/ui'
import type { QueueScope, TutorCardRef, TutorChip } from '@/types'

/**
 * The review screen.
 *
 * Deliberately chrome-free and keyboard-driven: this is a high-repetition
 * activity done two hundred times a day, and anything that requires the mouse in
 * that loop is a design failure.
 *
 *   space / enter  reveal or advance — and start the next set from its summary
 *   1 2 3 4        grade (receptive cards)
 *   h              hint — reveals, and caps the grade at Hard
 *
 * The day's queue is dealt in SETS (ADR-026, [R18]): a near finish line and a
 * set summary of measured facts between them. A missed card is listed on that
 * summary with its answer and comes back through the server's queue once its
 * learning step has elapsed — the store re-fetches the list at every set
 * boundary. The server serves the same queue as ever; the sets are this
 * screen's.
 *
 * WHAT to review — which pile, in what shape, or a practice round — is chosen
 * on the start screen at `/review` (ADR-028) and arrives here in the query:
 * `?scope=` for review, `?practice=` for a round. Nothing on this screen
 * changes that choice; the way to a different pile is back through the start
 * screen, which the done screen links to.
 *
 * The card is the only object on the screen and everything else is set below it
 * in weight, because during a session there is nothing else to look at. It is
 * the one place in the app where a `raised` surface is not a matter of taste.
 */
const store = useReviewStore()
const ui = useUiStore()
const speech = useSpeechStore()
const catalogue = useFeaturesStore()
const caps = useCapabilitiesStore()
const route = useRoute()
const router = useRouter()
const { t } = useI18n()
// Destructured so the refs are top-level and the template can read them
// directly; nested inside an object they would not be unwrapped.
const { verdict, flashKey, flashClass, signal, clear } = useAnswerFeedback()

/** The gloss, plus whether it fell back to the other language. Never silent. */
const gloss = computed(() => pickGloss(store.current?.glossRu, store.current?.glossEn, ui.locale))

/**
 * `listen` and `spell` are AUDIO cards: the prompt is the sound, and the
 * Hungarian is hidden until the answer. Printing the sentence would turn a
 * dictation into a copying exercise.
 */
const isAudioDirection = computed(
  () => store.current?.direction === 'listen' || store.current?.direction === 'spell',
)

/**
 * Whether the reveal block should print the whole sentence again.
 *
 * Only for the directions whose card does NOT already show it: `produce`, whose
 * prompt is the L1 gloss, and the two audio directions, which withhold the text
 * because the sound is the question. `recognize` and `cloze` both have the
 * sentence on the card by then, and printing it twice makes the card look
 * broken.
 */
const showsFullSentenceOnReveal = computed(
  () => store.current?.direction === 'produce' || isAudioDirection.value,
)

/**
 * What the whole sentence means, in the UI locale.
 *
 * Distinct from `gloss`, which is the TARGET WORD's dictionary entry. Showing
 * only the latter under a full sentence is what made the card unanswerable:
 * `Jó hírt hoztunk.` over «хороший; добрый» reads as a mistranslation rather
 * than as a gloss of one word in it.
 */
const sentenceTranslation = computed(() =>
  store.current
    ? pickGloss(store.current.sentenceRu, store.current.sentenceEn, ui.locale)
    : { text: '', isFallback: false },
)

/**
 * The cue for a cloze: which word to put in the gap, and what it means.
 *
 * A cloze without it is unanswerable. `Jó ____, Timmy!` accepts reggelt,
 * napot, estét and éjszakát — four real greetings — so the card was asking the
 * learner to guess which one we meant, and then grading Case=Acc on the guess.
 * Cloze is the only direction that writes to the morphological state space, so
 * a vocabulary guess landed in the grammar heatmap as a grammar failure.
 *
 * The word is given; the FORM is the exercise. The required features are
 * deliberately NOT shown — naming the case here would be handing over the
 * answer this card exists to ask for.
 */
/**
 * When the sentence's meaning is shown.
 *
 * Withheld until the answer on every direction that is asking about
 * comprehension — there the sentence IS the question and its translation is the
 * answer. `cloze` is the exception, and it is the whole point of the direction:
 * it asks WHICH FORM, and a learner cannot choose a form for a sentence they
 * cannot read. The blank is Hungarian and the crib is not, so nothing is handed
 * over — the required features are still deliberately unnamed.
 *
 * This is what a word mined from a text needs most: the learner met it once, in
 * a paragraph they were reading at a third coverage, and the Hungarian around
 * the blank is not by itself enough to say what belongs in it.
 *
 * Printed ONCE. Where the reveal prints the whole sentence — `produce` and the
 * audio directions — the crib goes down there with it, and this one stays
 * off: the same machine translation twice on one card, once above the answer
 * box and once below it, read as the card having been assembled by accident.
 */
const showsSentenceTranslation = computed(
  () =>
    store.current?.direction === 'cloze' ||
    (store.phase === 'answer' && !showsFullSentenceOnReveal.value),
)

/**
 * What the finished screen says.
 *
 * Three different endings, not one. Clearing the day's work, running into the
 * new-card cap and finishing a practice round are different facts, and the
 * first version of this screen printed «очередь пуста» over a queue that was
 * merely rationed.
 */
const practiceDoneTitle = computed(() => {
  if (store.mode === 'practice') {
    // A round over a pile with no words in it is not a round that was run.
    return store.practicePool === 0 ? t('review.practiceEmptyTitle') : t('review.practiceRoundDone')
  }
  if (pacedOut.value) return t('review.paceDone')
  return store.counts.dueLaterToday > 0 ? t('review.allDoneForNow') : t('review.allDone')
})
const practiceDoneBody = computed(() => {
  if (store.mode === 'practice') {
    return store.practicePool === 0 ? t('review.practiceEmpty') : t('review.practiceRoundDoneBody')
  }
  if (pacedOut.value) return t('review.paceDoneBody')
  // «Возвращайтесь завтра» over cards six minutes from due was the screen
  // lying; when steps are still maturing the countdown block carries the truth.
  return store.counts.dueLaterToday > 0 ? t('review.allDoneForNowBody') : t('review.allDoneBody')
})

/**
 * The string this card is graded against, as the server computed it.
 *
 * Taken from the server's verdict when there is one and from the queue item
 * otherwise, never derived here: `produce` grades the dictionary form and
 * `cloze` the form in the sentence, and a client-side guess at that rule is how
 * a hint comes to show one word while the grader wants another. See ADR-017.
 */
const expectedAnswer = computed(
  () => store.result?.expected || store.current?.expected || '',
)

/**
 * What the verdict panel — the block that takes the answer box's place once
 * an answer is in — has to show. The diff when the server produced one, the
 * confirmed form on a hit, and otherwise the answer plainly: a hint has no
 * verdict and no diff, and «нужно: …» is the one thing it exists to state.
 */
const verdictKind = computed<'diff' | 'correct' | 'expected'>(() => {
  const r = store.result
  if (r?.diff && !r.diff.match) return 'diff'
  if (r?.correct) return 'correct'
  return 'expected'
})

const clozeCue = computed(() => {
  const c = store.current
  if (!c || c.direction !== 'cloze') return null
  const word = c.lemma ?? ''
  const meaning = gloss.value.text
  if (!word && !meaning) return null
  return { word, meaning }
})

/**
 * The placeholder for a typed answer, saying how much to type.
 *
 * `produce` prompts with the target word's gloss, and a one-word Hungarian
 * target often glosses to a Russian phrase — `viszlát` is «пока, до встречи».
 * A learner shown that reasonably types a whole phrase and is marked wrong for
 * answering the question they were asked. Naming the scope costs no screen
 * space: it goes in the placeholder that was there anyway.
 */
const typePlaceholder = computed(() => {
  const c = store.current
  // `produce` is graded against the DICTIONARY form, so that is the string
  // whose length the scope hint has to describe — not the inflected token the
  // note's sentence happens to contain.
  const answer =
    c?.direction === 'produce' ? (c.lemma || c.targetForm) : (c?.targetForm ?? '')
  const target = answer.trim()
  const oneWord = target !== '' && !/\s/.test(target)
  return oneWord ? t('review.typeAnswerWord') : t('review.typeAnswer')
})

/**
 * The word just set aside, and the way back.
 *
 * Trashing is reversible, so it asks no confirmation — a dialog on a reversible
 * action is a tax on the common case. What it owes instead is a way back that
 * is visible without going and finding a screen, which is what this banner is.
 * It survives exactly one card: long enough to change your mind, short enough
 * not to become chrome.
 *
 * Restoring from here puts the word back in the DECK, not into this session —
 * see store.restoreTrashed. The banner says which, rather than leaving the
 * learner to wonder why the card did not reappear.
 */
const trashed = ref<{ noteId: number; label: string; restored: boolean } | null>(null)

/** The name to call the word by: its dictionary form, never the sentence. */
function cardLabel(item: { lemma?: string; targetForm: string; hu: string }) {
  return item.lemma || item.targetForm || item.hu
}

async function trashCurrent() {
  const card = store.current
  if (!card || store.trashing) return
  const label = cardLabel(card)
  clear()
  const res = await store.trashCurrent()
  if (res) trashed.value = { noteId: card.noteId, label, restored: false }
}

async function untrash() {
  const t = trashed.value
  if (!t || t.restored) return
  const res = await store.restoreTrashed(t.noteId)
  if (res) trashed.value = { ...t, restored: true }
}

/**
 * The word whose inspector panel is open, if any.
 *
 * Cleared on every card: a panel left open from the previous sentence would be
 * describing a word that is no longer on screen.
 */
const inspected = ref('')

function inspect(word: string) {
  inspected.value = inspected.value === word ? '' : word
}

/**
 * The tutor beside the card (ADR-034, amended 2026-09-04): open or not, and
 * the chat about THIS WORD once its first question has created one. Offered
 * in BOTH phases: before the answer a question is a hint, recorded through
 * `usedHint` the moment it is sent, and the server withholds the answer from
 * the model.
 *
 * The id is kept WITH the note it belongs to and read back only for that
 * note, so the panel cannot open on the previous word's conversation — which
 * is what one chat a day did: a learner looking at `Örülök` was shown an
 * answer about `jöttél`, and every question after it carried that answer into
 * the model's window. A plain ref reset by a watcher would have the same bug
 * one flush ordering away; this cannot.
 */
const tutorOpen = ref(false)
const tutorThread = ref<{ note: number; id: number } | null>(null)
const tutorThreadId = computed(() =>
  tutorThread.value && tutorThread.value.note === store.current?.noteId ? tutorThread.value.id : undefined,
)

function toggleTutor() {
  if (!tutorOpen.value && !store.current) return
  tutorOpen.value = !tutorOpen.value
}

/** The chat a first question creates belongs to the word it was asked about. */
function onTutorCreated(id: number) {
  const note = store.current?.noteId
  tutorThread.value = note ? { note, id } : null
}

/** The card in front of the learner, as the tutor's request names it. */
const tutorCard = computed<TutorCardRef | null>(() => {
  const c = store.current
  if (!c) return null
  const typed = (store.result?.diff?.typed || store.typed).trim()
  return { cardId: c.cardId, phase: store.phase, typed: typed || undefined }
})

/**
 * The prepared questions the card offers (ADR-034): chosen here from the
 * direction, the phase and the outcome — the client knows those — and
 * composed on the server, which knows the facts. The matrix is UX.md §4.
 */
const tutorChips = computed<TutorChip[]>(() => {
  const c = store.current
  if (!c) return []
  const form = c.targetForm || c.lemma || ''
  const lemma = c.lemma || c.targetForm || ''
  const typed = (store.result?.diff?.typed || store.typed).trim()
  const chip = (id: string): TutorChip => ({ id, label: t(`review.tutor.chips.${id}`, { form, typed, lemma }) })
  if (store.phase === 'question') {
    switch (c.direction) {
      case 'recognize':
        return [chip('hintForm'), chip('explainTask')]
      case 'produce':
        return [chip('hintWord'), chip('explainTask')]
      case 'cloze':
        return [chip('hintRule'), chip('explainTask')]
      default:
        return [chip('explainTask')]
    }
  }
  const res = store.result
  const missed = !!res?.diff && !res.diff.match
  // `targetFeats` is null from an older server for a form with no features.
  const whyForm = (c.targetFeats?.length ?? 0) > 0 || (!!c.lemma && c.targetForm !== c.lemma) ? [chip('whyForm')] : []
  switch (c.direction) {
    case 'produce':
    case 'cloze': {
      if (missed && typed) return [chip('whyNotMine'), chip('whyForm'), chip('breakdown'), chip('translateWord')]
      const out = [chip('whyForm'), chip('breakdown'), chip('translateWord')]
      if (res?.acceptedWithAccents) out.push(chip('spelling'))
      return out
    }
    case 'spell':
      return missed ? [chip('spelling'), chip('breakdown'), chip('translateWord')] : [chip('spelling'), chip('breakdown')]
    default: // recognize, listen
      return [chip('breakdown'), ...whyForm, chip('translateWord')]
  }
})

/** A question sent before the answer is a hint — recorded now, not when the answer lands. */
function onTutorAsked(phase: 'question' | 'answer') {
  if (phase === 'question') store.consultTutor()
}

type RatingKey = 'again' | 'hard' | 'good' | 'easy'
/** Good and Easy are capped at Hard once a hint was used (ADR-034); the buttons say so. */
function capped(key: RatingKey) {
  return store.usedHint && (key === 'good' || key === 'easy')
}
function effective(key: RatingKey): RatingKey {
  return capped(key) ? 'hard' : key
}


/**
 * Whether the session ended because the day's NEW material ran out — the
 * budget genuinely spent, with words genuinely waiting behind it. Sibling
 * DEFERRALS are excluded on purpose: the old test (`newNotesCapped > 0`) was
 * also true when a word was merely postponed because its other direction was
 * answered today, and the screen then blamed a limit that was never touched
 * and sent people to Settings to fix it.
 */
const pacedOut = computed(
  () =>
    store.counts.newPerDay > 0 &&
    store.counts.newBudgetLeft === 0 &&
    store.counts.newBacklogNotes > 0,
)

/**
 * Days to work through the new cards still waiting, at the current setting.
 *
 * A projection, not a promise — it assumes the pace holds and nothing is added.
 * Zero when there is no limit, in which case there is nothing to project.
 */
const daysToClear = computed(() => {
  const perDay = store.counts.newPerDay
  if (perDay <= 0) return 0
  return Math.ceil(store.counts.new / perDay)
})

/** The served pile's Breakdown — where the buried-until-tomorrow count lives. */
const currentBreakdown = computed(() => store.scopes.find((s) => s.scope === store.scope))

/**
 * Which pile this is, named — the tabs that used to say so are on the start
 * screen now (ADR-028), and it is the one fact the card cannot show by itself.
 * A practice round names the mode and the pile it drew from.
 */
const pileLabel = computed(() => {
  if (store.mode === 'practice') {
    const from = String(store.practiceScope)
    const pile = from.startsWith('text:')
      ? t('review.textScope')
      : `${t('review.practiceFrom')} ${t(`review.scope.${from}`)}`
    return `${t('review.practice')} · ${pile}`
  }
  return store.textScope ? t('review.textScope') : t(`review.scope.${String(store.scope)}`)
})

/**
 * Work waiting in the piles this session did not draw from, so emptying one
 * pile takes neither a reload nor a guess to discover the other has cards —
 * what the tabs on this screen were for. `all == course + reader` (ADR-016)
 * and a text filter is a slice of the reader pile (ADR-022), so the difference
 * against «Всё» is the rest of the deck; the figures are the queue response's
 * own Breakdowns (ADR-021), never arithmetic over card counts.
 */
const otherPiles = computed(() => {
  if (store.mode !== 'review' || store.scope === 'all') return null
  const all = store.scopes.find((s) => s.scope === 'all')
  const here = currentBreakdown.value
  if (!all || !here) return null
  const due = Math.max(0, all.dueNow - here.dueNow)
  const fresh = Math.max(0, all.newNotes - here.newNotes)
  return due > 0 || fresh > 0 ? { due, new: fresh } : null
})

/**
 * A practice round over the pile just reviewed. A NAVIGATION, not a store
 * call: the URL then says what is on screen, and a reload gives the round
 * back rather than a review session.
 */
const practiceTo = computed(() => ({
  path: '/review/session',
  query: { practice: String(store.scope) },
}))

/**
 * The countdown to the next learning step.
 *
 * This is the honest replacement for the old ending: «На сегодня всё» over
 * cards that were 6 minutes from being due, discovered only by refreshing. The
 * clock ticks once a second while the done screen shows, and when the step
 * matures the queue reloads itself — keeping the running session summary.
 */
const nowTick = ref(Date.now())
let tickTimer: number | undefined
let lastAutoReload = 0

const nextDueInMs = computed(() => {
  const at = store.counts.nextDueAt
  if (!at || store.counts.dueLaterToday <= 0) return null
  return new Date(at).getTime() - nowTick.value
})

const nextDueLabel = computed(() => {
  const ms = nextDueInMs.value
  if (ms === null) return ''
  if (ms <= 0) return t('review.dueSoonNow')
  const totalS = Math.ceil(ms / 1000)
  const m = Math.floor(totalS / 60)
  const s = totalS % 60
  return t('review.dueSoonIn', { t: `${m}:${String(s).padStart(2, '0')}` })
})

function maybeAutoReload() {
  nowTick.value = Date.now()
  // Only the DONE screen reloads on its own. Between sets the list was
  // refreshed the moment the set finished, and the learner is reading a
  // summary; rebuilding it under them again would be the screen deciding.
  if (!store.done || store.mode !== 'review' || store.loading) return
  const ms = nextDueInMs.value
  if (ms === null || ms > 0) return
  // Back off between attempts: with client/server clocks apart, an eager loop
  // would hammer the API every tick until the card actually matures.
  if (Date.now() - lastAutoReload < 15_000) return
  lastAutoReload = Date.now()
  void store.load(undefined, { keepSession: true }).then(focusAnswer)
}

/** A tab left in the background catches up the moment it is looked at. */
function onVisible() {
  if (document.visibilityState !== 'visible') return
  if (store.done && store.mode === 'review' && !store.loading) {
    void store.load(undefined, { keepSession: true }).then(focusAnswer)
  }
}

/** The session summary: competence signals only — measured, never invented. */
const sessionAccuracy = computed(() =>
  store.session.answered ? Math.round((store.session.correct / store.session.answered) * 100) : 0,
)
const sessionMinutes = computed(() => Math.max(1, Math.round(store.session.activeMs / 60_000)))

/**
 * The set, as the bar draws it: which set this is out of how many, and how
 * many of its cards have been answered. `setsDone` keeps counting across the
 * between-set refresh, so the label does not restart at one each time the
 * list is fetched again.
 */
const setLabel = computed(() =>
  store.inSets
    ? t('review.set.label', { n: store.setsDone + 1, of: store.setsDone + store.setCount })
    : '',
)
const setPct = computed(() =>
  store.setProgress.total ? Math.round((store.setProgress.done / store.setProgress.total) * 100) : 0,
)

/**
 * What each grade would schedule, under the buttons — the scheduler's own
 * figures, labelled approximate because interval fuzz is seeded at answer
 * time. Never in practice: nothing there is scheduled. [R18 §5]
 */
function previewLabel(key: 'again' | 'hard' | 'good' | 'easy'): string {
  const p = store.current?.memory?.preview
  if (!p || store.mode !== 'review') return ''
  // From the instant the server computed them, not from now: the previews
  // are made when the queue is served, and measured against the clock the
  // 10-minute step read «~7 мин» on a card reached three minutes later.
  const i = untilInterval(p[key], p.at ? new Date(p.at).getTime() : nowTick.value)
  return t(`review.units.${i.unit}`, { n: i.n })
}

/**
 * The next set: the head of the list the store fetched when the last one
 * finished, so anything that matured in the meantime is already in it.
 */
async function nextSet() {
  if (!store.betweenSets) return
  store.startNextSet()
  await focusAnswer()
}

/**
 * The card's boundary carries the verdict, alongside the icon and the diff.
 * Colour is never the only channel — this is the third copy of the same fact,
 * for the learner who sees it before they read anything. docs/UX.md §11
 */
const cardTone = computed(() => {
  if (verdict.value === 'correct') return 'border-known/60'
  if (verdict.value === 'wrong') return 'border-error/60'
  return 'border-border'
})

async function focusAnswer() {
  await nextTick()
  const el = document.querySelector<HTMLInputElement>('[data-answer-input]')
  el?.focus()
}

/**
 * Autoplay, on the directions where the audio IS the question.
 *
 * Never on `recognize` — hearing the sentence read out gives away the card that
 * is asking whether you can read it (docs/UX.md §4). A browser that refuses to
 * speak without a gesture simply leaves the 🔊 button to be pressed; nothing
 * here depends on the autoplay succeeding.
 */
watch(
  () => store.current?.cardId,
  () => {
    inspected.value = ''
    void focusAnswer()
    if (isAudioDirection.value && store.current) speech.speak(store.current.hu)
  },
)

onUnmounted(() => speech.cancel())

/**
 * The full stop. Fires only on a session that had something in it: arriving at
 * an already-empty queue is not an achievement and must not sound like one.
 */
watch(
  () => store.done,
  (done, was) => {
    if (done && !was && store.queue.length > 0) play('sessionComplete')
  },
)

/** A comma, not a full stop: two notes when a set is finished and more follow. */
watch(
  () => store.betweenSets,
  (between, was) => {
    if (between && !was) play('setComplete')
  },
)

function advance() {
  clear()
  trashed.value = null
  store.next()
}

async function submitTyped() {
  if (store.phase === 'answer') {
    advance()
    await focusAnswer()
    return
  }
  const res = await store.submit()
  if (res) signal(res.correct)
  await showVerdict()
}

/**
 * Bring the verdict into view if the card ran past the fold. Usually a no-op:
 * the verdict takes the answer box's place, which is where the eye already
 * is — but a long sentence on a phone can still push it under the key strip,
 * and `scroll-mb` on the block keeps it clear of that.
 */
async function showVerdict() {
  await nextTick()
  document.querySelector('[data-verdict]')?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
}

/**
 * The map, as the strip draws it.
 *
 * It reflects the PHASE, because the keys do: `1`–`4` only mean something once
 * an answer is on screen, and `Esc` only when the inspector is open. Drawn dim
 * rather than removed, so the row does not reflow under the hands between every
 * card — a strip whose keys move is harder to read than one with a grey key in it.
 *
 * `u` and `Esc` reach a learner here for the first time. Both have been
 * implemented since the screen was built and were advertised in neither of the
 * two hint sentences this replaces.
 */
const keyHints = computed<KeyHint[]>(() => {
  const answering = store.phase === 'answer'
  return [
    store.isProductive
      ? { keys: ['Enter'], label: t('keys.review.check') }
      : { keys: ['Space'], label: answering ? t('keys.review.next') : t('keys.review.reveal') },
    { keys: ['1', '2', '3', '4'], label: t('keys.review.rate'), muted: store.isProductive || !answering },
    { keys: ['h'], label: t('keys.review.hint') },
    { keys: ['u'], label: t('keys.review.undo') },
    // Dimmed while the answer box has focus, because `t` is a letter and goes
    // into it — the same guard `h` and `u` sit behind. The button under the
    // card is the affordance that always works, and a strip promising a key
    // that types «t» into the answer would be worse than no strip.
    {
      keys: ['t'],
      label: t('keys.review.trash'),
      muted: store.isProductive && store.phase === 'question',
    },
    { keys: ['a'], label: t('keys.review.tutor'), muted: !store.current },
    { keys: ['Escape'], label: t('keys.review.closeInspector'), muted: !inspected.value && !tutorOpen.value },
  ]
})

async function grade(rating: number) {
  if (store.isProductive) return
  const res = await store.submit(rating)
  if (res) signal(res.correct)
  advance()
  await focusAnswer()
}

function onKey(e: KeyboardEvent) {
  if (store.done || store.loading) return
  const typing = (e.target as HTMLElement)?.tagName === 'INPUT'

  // The tutor's composer takes its own keys — Enter sends, Esc stops — and
  // the card must not answer to them. An Esc the composer did not keep
  // closes the panel.
  if ((e.target as HTMLElement)?.tagName === 'TEXTAREA') {
    if (e.key === 'Escape' && tutorOpen.value) {
      e.preventDefault()
      tutorOpen.value = false
    }
    return
  }

  if (store.betweenSets) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      void nextSet()
    }
    return
  }
  if (!store.current) return

  // Esc closes the inspector, then the tutor, rather than leaving either to
  // be dismissed by mouse.
  if (e.key === 'Escape' && (inspected.value || tutorOpen.value)) {
    e.preventDefault()
    if (inspected.value) inspected.value = ''
    else tutorOpen.value = false
    return
  }

  if (e.key === 'Enter' || (e.key === ' ' && !typing)) {
    e.preventDefault()
    if (store.isProductive) {
      void submitTyped()
    } else if (store.phase === 'question') {
      store.reveal()
    } else {
      advance()
    }
    return
  }
  if (!typing && e.key === 'h') {
    e.preventDefault()
    store.hint()
    return
  }
  if (!typing && e.key === 'a') {
    e.preventDefault()
    toggleTutor()
    return
  }
  if (!typing && e.key === 'u') {
    e.preventDefault()
    clear()
    void store.undo()
    return
  }
  // `t` is guarded on `typing` like `h`: a productive card's answer box is
  // where Hungarian gets typed, and `t` is a letter in it.
  if (!typing && e.key === 't') {
    e.preventDefault()
    void trashCurrent()
    return
  }
  if (!store.isProductive && store.phase === 'answer' && ['1', '2', '3', '4'].includes(e.key)) {
    e.preventDefault()
    void grade(Number(e.key))
  }
}

/**
 * Start the session the URL asks for.
 *
 * The intent travels in the QUERY, never in a store call made before
 * navigating: this screen loads a queue on arrival, so a round started on
 * another screen was overwritten a moment later by an ordinary review session
 * — the trash's «тренировать отложенные» looked as though it had been ignored.
 * The start screen (ADR-028), the library's and the reader's «повторить слова»
 * and the trash all navigate here carrying their choice.
 *
 * Validated the way the server validates it, and an unknown scope is a bare
 * review over the whole deck rather than a request the server would 400.
 * Practice, never review, for the trash: a trashed word must not be
 * scheduled, and the server refuses `scope=trash` on the review queue too.
 */
const REVIEW_SCOPE = /^(all|course|reader|text:\d+)$/
const PRACTICE_SCOPE = /^(all|course|reader|trash|text:\d+)$/

function startFrom(query: LocationQuery) {
  const practice = String(query.practice ?? '')
  if (PRACTICE_SCOPE.test(practice)) {
    void store.loadPractice(practice as QueueScope).then(focusAnswer)
    return
  }
  const wanted = String(query.scope ?? '')
  const scope = REVIEW_SCOPE.test(wanted) ? (wanted as QueueScope) : 'all'
  void store.load(scope).then(focusAnswer)
}

/** The same route with a new query — the done screen's «Тренировка» — is a new session. */
onBeforeRouteUpdate((to) => startFrom(to.query))

onMounted(() => {
  startFrom(route.query)
  window.addEventListener('keydown', onKey)
  void catalogue.ensure()
  // The crib's «Перевести» needs to know whether the translator is up; the
  // topbar usually asked already, but the card must not depend on order.
  if (!caps.list.length) void caps.refresh()
  document.addEventListener('visibilitychange', onVisible)
  tickTimer = window.setInterval(maybeAutoReload, 1000)
})
onUnmounted(() => {
  window.removeEventListener('keydown', onKey)
  document.removeEventListener('visibilitychange', onVisible)
  if (tickTimer) window.clearInterval(tickTimer)
})

const ratingKeys = [
  { n: 1, key: 'again' as const, variant: 'destructive' as const },
  { n: 2, key: 'hard' as const, variant: 'outline' as const },
  { n: 3, key: 'good' as const, variant: 'default' as const },
  { n: 4, key: 'easy' as const, variant: 'secondary' as const },
]
</script>

<template>
  <!--
    The card owns the middle; the inspector, when open, takes a column beside it
    on a wide screen and stacks under it on a narrow one. The whole thing shifts
    left rather than the card moving, so the sentence you are reading does not
    jump when you click a word in it.
  -->
  <div
    class="mx-auto flex w-full flex-col gap-6 lg:flex-row lg:items-start lg:justify-center"
    :class="tutorOpen ? 'max-w-6xl' : inspected ? 'max-w-5xl' : 'max-w-2xl'"
  >
    <!--
      Anchored at the top while a card is up. Centred, the column re-centred
      itself every time the reveal made the card taller, and the prompt the
      learner had just read moved up the screen with each answer. The empty and
      between-set screens are short and still sit in the middle.
    -->
    <div
      class="flex w-full min-w-0 max-w-2xl flex-col gap-6"
      :class="store.current && !store.betweenSets ? '' : 'min-h-[70vh] justify-center'"
    >
    <p v-if="store.loading" class="text-sm text-muted-foreground">{{ $t('common.loading') }}</p>

    <!--
      Nothing due. Calm, not a celebration: clearing a daily queue is the job,
      and making it the loudest moment in the app would be exactly the reward
      economy [R16] §3 says does nothing for competence. The descending fifth
      that just played is the full stop — see lib/sound.ts.
    -->
    <div v-else-if="store.done" class="flex flex-col items-center gap-3 py-16 text-center">
      <Illustration name="rosette" size="md" tone="accent" class="mb-1" />
      <!--
        Two different endings, and they used to be one.

        "Очередь пуста. Возвращайтесь завтра" printed directly above "новых
        карточек в очереди: 59" is a screen contradicting itself: the queue is
        not empty, the day's ration of NEW material is spent. Clearing the day's
        work and running into a cap are different facts and now read
        differently.
      -->
      <h1 class="font-display text-xl font-medium">
        {{ practiceDoneTitle }}
      </h1>
      <p class="max-w-sm text-sm leading-relaxed text-muted-foreground">
        {{ practiceDoneBody }}
      </p>

      <!-- The session summary: what actually happened, measured. Competence
           signals only — no points, no streak theatre. [R14], docs/UX.md §14.5 -->
      <dl
        v-if="store.session.answered"
        class="mt-1 grid grid-cols-2 gap-x-10 gap-y-2 rounded-lg border border-border-subtle px-6 py-4 text-center sm:grid-cols-4"
      >
        <div>
          <dt class="text-[11px] uppercase tracking-wide text-muted-foreground">
            {{ $t('review.summary.answered') }}
          </dt>
          <dd class="font-mono text-xl tabular-nums">{{ store.session.answered }}</dd>
        </div>
        <div>
          <dt class="text-[11px] uppercase tracking-wide text-muted-foreground">
            {{ $t('review.summary.accuracy') }}
          </dt>
          <dd class="font-mono text-xl tabular-nums">{{ sessionAccuracy }}%</dd>
        </div>
        <div>
          <dt class="text-[11px] uppercase tracking-wide text-muted-foreground">
            {{ $t('review.summary.newWords') }}
          </dt>
          <dd class="font-mono text-xl tabular-nums">{{ store.session.newSeen }}</dd>
        </div>
        <div>
          <dt class="text-[11px] uppercase tracking-wide text-muted-foreground">
            {{ $t('review.summary.minutes') }}
          </dt>
          <dd class="font-mono text-xl tabular-nums">{{ sessionMinutes }}</dd>
        </div>
      </dl>

      <!-- The words missed this sitting, with their answers: the one thing a
           miss gets here besides its next scheduled turn. Read, not re-asked —
           nothing on this screen tests anything. -->
      <div
        v-if="store.session.missed.length"
        class="mt-1 w-full max-w-md rounded-lg border border-border-subtle px-5 py-3 text-left"
      >
        <p class="text-[11px] tracking-wide text-muted-foreground uppercase">
          {{ $t('review.summary.missed', { n: store.session.missed.length }) }}
        </p>
        <MissedList :items="store.session.missed" class="mt-2" />
        <p class="mt-2 text-xs leading-relaxed text-muted-foreground">
          {{
            store.mode === 'practice'
              ? $t('review.summary.missedBodyPractice')
              : $t('review.summary.missedBody')
          }}
        </p>
      </div>

      <!-- Learning steps maturing today: the cards behind «обновил страницу и
           появилось ещё 5». Named, counted down, and reloaded automatically —
           never denied. -->
      <div
        v-if="store.mode === 'review' && store.counts.dueLaterToday > 0"
        class="mt-1 flex max-w-md flex-col items-center gap-1 rounded-lg border border-border-subtle px-6 py-3"
      >
        <p class="flex items-center gap-2 text-sm">
          <Clock class="size-4 text-learning" aria-hidden="true" />
          {{ $t('review.dueSoon', { n: store.counts.dueLaterToday }) }}
        </p>
        <p class="font-mono text-lg tabular-nums" aria-live="polite">{{ nextDueLabel }}</p>
        <p class="text-xs text-muted-foreground">{{ $t('review.dueSoonAuto') }}</p>
      </div>

      <!-- Where the rest of the «26» went: due, hidden until tomorrow, and said
           so instead of being silently counted. -->
      <p
        v-if="store.mode === 'review' && (currentBreakdown?.buriedToday ?? 0) > 0"
        class="flex max-w-md items-center gap-2 text-xs text-muted-foreground"
      >
        <MoonStar class="size-3.5 shrink-0" aria-hidden="true" />
        {{ $t('review.buriedToday', { n: currentBreakdown?.buriedToday }) }}
      </p>
      <p
        v-if="store.mode === 'review' && store.counts.newDeferredNotes > 0"
        class="flex max-w-md items-center gap-2 text-xs text-muted-foreground"
      >
        <MoonStar class="size-3.5 shrink-0" aria-hidden="true" />
        {{ $t('review.newDeferred', { n: store.counts.newDeferredNotes }) }}
      </p>

      <!-- The whole point of the mode: there is always another round, and it
           is reachable from the moment the queue runs out. From a review it
           is a navigation over the pile just finished — the intent travels in
           the query, so the URL stays honest (ADR-028). -->
      <Button
        v-if="store.mode === 'practice' && store.practicePool > 0"
        variant="outline"
        class="mt-1"
        @click="store.loadPractice()"
      >
        {{ $t('review.practiceAgain') }}
      </Button>
      <RouterLink
        v-else-if="store.mode === 'review'"
        :to="practiceTo"
        :class="buttonVariants({ variant: 'outline' })"
        class="mt-1"
      >
        {{ $t('review.practice') }}
      </RouterLink>

      <Note v-if="pacedOut" variant="limit" class="mt-2 max-w-md text-left">
        <!-- The numbers first, in WORDS, then the mechanism in cards, then why
             the cap exists. A limit without its arithmetic reads as a fault. -->
        <p>
          {{
            $t('review.paceWaiting', {
              words: store.counts.newBacklogNotes,
              cards: store.counts.new,
            })
          }}
        </p>
        <p v-if="daysToClear" class="mt-1.5">
          {{
            $t('review.paceDays', {
              days: daysToClear,
              perDay: store.counts.newPerDay,
              today: store.counts.introducedToday,
            })
          }}
        </p>
        <p class="mt-1.5 text-xs">{{ $t('review.paceWhy') }}</p>
      </Note>

      <!-- What the other piles hold, in one sentence: the tabs that stood
           here said it with numbers, and emptying one pile must still not take
           a reload to discover the other has work (ADR-016, ADR-028). -->
      <p v-if="otherPiles" class="max-w-md text-xs leading-relaxed text-muted-foreground">
        {{ $t('review.start.elsewhere', otherPiles) }}
      </p>

      <div class="mt-2 flex flex-wrap justify-center gap-2">
        <RouterLink to="/review" :class="buttonVariants({ variant: 'outline' })">
          {{ $t('review.start.choose') }}
        </RouterLink>
        <RouterLink to="/" :class="buttonVariants({ variant: 'ghost' })">
          {{ $t('common.backHome') }}
        </RouterLink>
        <!-- The pace is a setting, not a verdict. Saying "come back tomorrow"
             without showing where the dial is makes it look like one. -->
        <RouterLink v-if="pacedOut" to="/settings" :class="buttonVariants({ variant: 'ghost' })">
          {{ $t('review.pacePace') }}
        </RouterLink>
      </div>
    </div>

    <!-- Between sets: what this set did, measured, and the way to the next. -->
    <SetSummary
      v-else-if="store.betweenSets"
      :stats="store.setStats"
      :set="{ n: store.setsDone, of: store.setsDone + store.setCount }"
      :left="store.remaining"
      @next="nextSet"
      @stop="router.push('/')"
    />

    <template v-else-if="store.current">
      <!-- Practice schedules nothing, and the learner is told so rather than
           left to infer it from a due date that never moves. The app's
           credibility rests on not overclaiming, and this is the one screen
           where the same card means two different things. -->
      <Note v-if="store.mode === 'practice'" variant="info" class="text-xs">
        {{ $t('review.practiceNotice') }}
      </Note>

      <!-- The word just set aside, and the way back. No confirmation dialog
           precedes this — trashing is reversible, and a dialog on a reversible
           action taxes the common case to guard the rare one. What it owes
           instead is an undo that is visible without hunting for a screen.

           After a restore the wording changes rather than the banner vanishing:
           the card does not reappear in THIS session (the queue was built
           before it left), and a banner that simply disappeared would look
           like the restore had failed. -->
      <Note v-if="trashed" variant="info" class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs">
        <span class="flex-1">
          {{
            trashed.restored
              ? $t('review.trash.restored', { word: trashed.label })
              : $t('review.trash.done', { word: trashed.label })
          }}
        </span>
        <Button
          v-if="!trashed.restored"
          variant="ghost"
          size="sm"
          class="h-7 shrink-0 gap-1.5 px-2 text-xs"
          @click="untrash"
        >
          <Undo2 class="size-3.5" aria-hidden="true" />
          {{ $t('review.trash.undo') }}
        </Button>
        <RouterLink
          v-else
          to="/trash"
          :class="buttonVariants({ variant: 'ghost', size: 'sm' })"
          class="h-7 shrink-0 px-2 text-xs"
        >
          {{ $t('review.trash.open') }}
        </RouterLink>
      </Note>

      <!-- Progress -->
      <div class="flex flex-col gap-2">
        <!-- Wraps: «ВОСПРОИЗВЕДЕНИЕ» is a long chip and the content column is
             narrow on a phone, where it used to run off the right edge. -->
        <div
          class="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 text-xs text-muted-foreground"
        >
          <span class="flex flex-wrap items-center gap-x-2 gap-y-0.5">
            <!-- The pile, now that the tabs are on the start screen (ADR-028).
                 A practice round also prints the criterion really applied:
                 «слабые» only when the server served it that way, and the
                 fallback echo when it did not (ADR-024). -->
            <span>{{ pileLabel }}</span>
            <span v-if="store.mode === 'practice' && store.practiceServedMode === 'weak'">
              · {{ $t('review.practiceWeak') }}
            </span>
            <span
              v-else-if="store.mode === 'practice' && store.practiceMode === 'weak'"
              class="text-uncertain"
            >
              · {{ $t('review.practiceWeakFallback') }}
            </span>
            <span v-if="setLabel" class="font-medium text-foreground">· {{ setLabel }}</span>
            <span class="font-mono tabular-nums">
              {{ $t('review.set.progress', { done: store.setProgress.done, total: store.setProgress.total }) }}
            </span>
          </span>
          <!--
            The direction, as a LABEL. It used to be a bordered pill sitting in
            the top-right corner where a control belongs, and `пропуск` — the
            name of the direction — reads in Russian as the imperative "skip".
            It was clicked as a skip button and did nothing, which is the worst
            outcome an affordance can have. No border, no box, no hit target —
            `pointer-events-none` keeps it that way now that it is two chips.

            Two chips, because it was two words: `УЗНАВАНИЕ НОВАЯ`, uppercased
            by CSS, run together with a space between them and nothing saying
            that the first is what the card asks of you and the second is that
            you have not seen it before. Read as one unparseable label.
          -->
          <span class="flex min-w-0 items-center gap-2">
          <span class="pointer-events-none flex min-w-0 flex-wrap items-center gap-1.5">
            <span class="hidden text-[10px] text-muted-foreground sm:inline">
              {{ $t('review.cardType') }}
            </span>
            <span
              class="rounded border border-border-subtle bg-muted px-1.5 py-0.5 text-[10px] tracking-wide uppercase"
            >
              {{ $t(`review.directions.${store.current.direction}`) }}
            </span>
            <span
              v-if="store.current.isNew"
              class="rounded border border-new/40 bg-new/10 px-1.5 py-0.5 text-[10px] tracking-wide text-new uppercase"
            >
              {{ $t('review.newCard') }}
            </span>
          </span>
          <!-- The tutor, top right, in both phases (ADR-034). A sibling of the
               chips' span, which is pointer-events-none on purpose. -->
          <Button
            variant="ghost"
            size="sm"
            class="shrink-0 gap-1.5 text-xs text-muted-foreground hover:text-foreground"
            :class="tutorOpen ? 'bg-accent text-foreground' : ''"
            :aria-pressed="tutorOpen"
            :title="$t('review.tutor.hint')"
            @click="toggleTutor"
          >
            <GraduationCap class="size-3.5" aria-hidden="true" />
            <span class="hidden sm:inline">{{ $t('review.tutor.open') }}</span>
            <span class="font-mono text-[10px] opacity-60">a</span>
          </Button>
          </span>
        </div>
        <!-- The set's own bar: cards answered, over the set. A finish line the
             learner can see the end of. [R18 §2] -->
        <div class="h-1 w-full overflow-hidden rounded-full bg-muted">
          <div
            class="h-full rounded-full bg-primary transition-[width] duration-200"
            :style="{ width: `${setPct}%` }"
          />
        </div>
        <!-- What this direction actually asks of you. The five directions were
             named and never explained, and the name alone does not tell a
             learner whether to type, to recall or to listen. docs/UX.md §1.4. -->
        <p class="text-[11px] leading-relaxed text-muted-foreground">
          {{ $t(`review.directionHint.${store.current.direction}`) }}
        </p>
        <!-- What this card knows about itself: stage, the last outcomes — and
             after the answer, where the interval went. Measured figures only;
             there is no score here. [R18], ADR-026 -->
        <MemoryMeter
          v-if="store.current.memory"
          :memory="store.current.memory"
          :after="store.result?.memory ?? null"
          :scheduled="store.mode === 'review'"
          class="mt-1"
        />
      </div>

      <!--
        The card. Keyed on the card id so each one arrives with `card-in` rather
        than the text swapping in place — 160ms, and interactive on the frame it
        mounts, so nothing waits for it.
      -->
      <div
        :key="store.current.cardId"
        class="animate-card-in rounded-xl border bg-card p-8 shadow-raised transition-colors"
        :class="cardTone"
      >
        <!--
          An audio card withholds the text in BOTH phases: the sound is the
          question, and the sentence is printed once, below the divider, by the
          reveal block. Printing it up here as well would show the answer while
          the card is still asking.

          A device with no Hungarian voice says so instead of presenting a
          silent card. The card is still answerable — the sentence appears on
          reveal — but the learner is told why there is nothing to hear rather
          than left tapping a dead button. ADR-005.
        -->
        <div v-if="isAudioDirection" class="flex flex-col items-center gap-3 py-4">
          <SpeakButton
            :text="store.current.hu"
            size="sm"
            :label="$t('speech.replay')"
            :rate="store.current.direction === 'spell' ? 0.85 : 1"
          />
          <p
            v-if="!speech.usable"
            class="max-w-sm text-center text-xs leading-relaxed text-uncertain"
          >
            {{ speech.supported ? $t('speech.noVoiceHint') : $t('speech.unsupportedHint') }}
          </p>
        </div>

        <!--
          The Hungarian, with every word clickable.

          The split comes from the server (`srs.TargetSplit`), so this never
          locates the target itself: those offsets are UTF-8 bytes and
          JavaScript counts UTF-16 — slicing with them put the cloze gap inside
          the following word on every accented sentence.

          `produce` is the exception: its prompt is the L1 gloss, because the
          whole card is "say this in Hungarian" and showing the Hungarian would
          be showing the answer.
        -->
        <p
          v-else-if="store.current.direction === 'produce'"
          class="text-center text-2xl leading-relaxed"
        >
          {{ gloss.text }}
        </p>
        <p v-else class="text-center text-2xl leading-relaxed">
          <HungarianText
            :text="store.current.hu"
            :before="store.target?.before"
            :after="store.target?.after"
            :blank="store.current.direction === 'cloze' && store.phase === 'question'"
            :highlight="store.current.direction !== 'cloze' || store.phase === 'answer'"
            :selected="inspected"
            @pick="inspect"
          />
          <SpeakButton
            v-if="store.phase === 'answer'"
            :text="store.current.hu"
            class="ml-1 align-middle"
          />
        </p>

        <!--
          The cloze cue, at question time. Without it the card names no word.
        -->
        <p
          v-if="clozeCue && store.phase === 'question'"
          class="mt-3 flex flex-wrap items-baseline justify-center gap-x-2 text-center text-sm"
        >
          <span class="text-muted-foreground">{{ $t('review.putInForm') }}</span>
          <span v-if="clozeCue.word" class="text-lg font-medium" lang="hu">{{
            clozeCue.word
          }}</span>
          <span v-if="clozeCue.meaning" class="text-muted-foreground"
            >— {{ clozeCue.meaning }}</span
          >
        </p>

        <!--
          What the SENTENCE means, from the pack's own translation.

          Distinct from the word gloss below it, and the confusion between the
          two is what made this card unreadable: `Jó hírt hoztunk.` was shown
          under «хороший; добрый» — the dictionary entry for `jó` — with nothing
          saying that the gloss belonged to one of the four words, let alone
          which. The target word is now underlined above and the sentence gets
          its own line.

          Withheld until the answer on every direction that is asking about
          comprehension, which is all of them except `recognize` — there the
          sentence IS the question and its translation is the answer.
        -->
        <SentenceCrib
          v-if="showsSentenceTranslation"
          :hu="store.current.hu"
          :key="store.current.cardId"
          :text="sentenceTranslation.text"
          :is-fallback="sentenceTranslation.isFallback"
          :model="store.current.sentenceModel"
          class="mt-3"
        />

        <!--
          The TARGET WORD's dictionary entry, labelled as such and attributed to
          the word it belongs to. Unlabelled and floating under the sentence, it
          read as a bad translation of the whole thing.
        -->
        <p
          v-if="store.current.direction !== 'produce' && gloss.text && store.phase === 'answer'"
          class="mt-3 text-center text-sm leading-relaxed text-muted-foreground"
        >
          <span v-if="store.current.targetForm" class="font-medium text-foreground" lang="hu">
            {{ store.current.targetForm }}
          </span>
          <span class="mx-1">—</span>{{ gloss.text }}
          <span v-if="gloss.isFallback" class="ml-1 text-xs text-uncertain">
            {{ $t('review.glossFallback') }}
          </span>
        </p>

        <!--
          Typed answer. The box is the question phase's; once an answer is in,
          the verdict takes its place — «вы / нужно» lands exactly where the
          eye already was, instead of below a repeated sentence and a divider,
          which on a long card meant scrolling to find out what was wrong.
        -->
        <form
          v-if="store.isProductive && store.phase === 'question'"
          class="mt-6 flex flex-col gap-3"
          @submit.prevent="submitTyped"
        >
          <Input
            v-model="store.typed"
            data-answer-input
            lang="hu"
            autocapitalize="off"
            autocomplete="off"
            spellcheck="false"
            class="text-center text-lg"
            :placeholder="typePlaceholder"
          />
          <div class="flex justify-center gap-2">
            <Button type="submit" :disabled="store.submitting">{{ $t('review.check') }}</Button>
            <Button type="button" variant="ghost" size="sm" @click="store.hint()">
              {{ $t('review.hint') }}
            </Button>
          </div>
          <p
            v-if="previewLabel(effective('good'))"
            class="text-center text-[11px] text-muted-foreground"
            :title="$t('review.memory.previewHint')"
          >
            {{ $t('review.memory.ifRight', { t: previewLabel(effective('good')) }) }}
          </p>
        </form>

        <!-- Reveal for receptive cards -->
        <div v-else-if="store.phase === 'question'" class="mt-6 flex justify-center">
          <Button @click="store.reveal()">{{ $t('review.show') }}</Button>
        </div>

        <!--
          Result. Keyed on the feedback counter so the movement replays for
          every verdict — CSS will not restart an animation on an element that
          did not change. `scroll-mb` keeps it clear of the fixed key strip when
          `showVerdict` has to scroll at all.

          Order: the verdict first, then what the card additionally graded,
          then the sentence as context — for the directions whose prompt did
          not already show it. A `recognize` card has nothing to add here: its
          crib and gloss are printed with the sentence above.
        -->
        <div
          v-else-if="store.isProductive || showsFullSentenceOnReveal"
          :key="flashKey"
          data-verdict
          class="mt-6 flex scroll-mb-28 flex-col gap-4"
          :class="flashClass"
        >
          <template v-if="store.isProductive">
            <MorphemeDiff
              v-if="verdictKind === 'diff' && store.result?.diff"
              :diff="store.result.diff"
            />

            <!-- A hit: the verdict and the form it confirmed, on one line. -->
            <div
              v-else-if="verdictKind === 'correct'"
              class="flex flex-wrap items-baseline justify-center gap-x-3 gap-y-1"
            >
              <span class="flex items-center gap-1.5 text-sm font-medium text-known">
                <Check class="size-4" aria-hidden="true" />
                {{ $t('review.right') }}
              </span>
              <span class="font-mono text-2xl leading-relaxed text-known" lang="hu">
                {{ expectedAnswer }}
              </span>
            </div>

            <!--
              The answer, plainly — the diff's well, without a diff.

              «Подсказка» used to reveal NOTHING on a typed card: it sets the
              phase to `answer` without submitting, so there was no diff and no
              server verdict, and the one thing a hint exists to show was the
              one thing missing. Worse since `produce` began grading the
              dictionary form — the sentence below contains the inflected one,
              so the answer was nowhere on the screen at all.
            -->
            <div
              v-else
              class="rounded-lg border border-border-subtle bg-background p-3 font-mono text-lg leading-relaxed"
            >
              <div v-if="store.typed" class="flex items-baseline gap-2">
                <span class="w-16 shrink-0 text-xs text-muted-foreground">{{ $t('review.youWrote') }}</span>
                <span class="text-error">{{ store.typed }}</span>
              </div>
              <div class="flex items-baseline gap-2">
                <span class="w-16 shrink-0 text-xs text-muted-foreground">{{ $t('review.correct') }}</span>
                <span class="text-known" lang="hu">{{ expectedAnswer }}</span>
              </div>
            </div>

            <!--
              A tolerated pass says so, in the uncertainty colour, with the
              real spelling. Green alone here would be a silent pass — and
              hat/hát being different words is the one fact this line exists
              to keep in view.
            -->
            <p
              v-if="store.result?.acceptedWithAccents"
              class="text-center text-xs leading-relaxed text-uncertain"
            >
              {{ $t('review.acceptedAccents', { expected: store.result?.expected || store.current.targetForm }) }}
            </p>

            <FeatureChips v-if="store.result" :features="store.result.gradedFeatures" />
          </template>

          <!--
            The sentence, where the prompt did not already show it: the L1
            gloss was the `produce` prompt, and the audio directions withheld
            the text because the sound was the question. The crib goes WITH
            the sentence it translates, and only here — see
            `showsSentenceTranslation`.
          -->
          <div
            v-if="showsFullSentenceOnReveal"
            class="flex flex-col gap-2"
            :class="store.isProductive ? 'border-t border-border-subtle pt-5' : ''"
          >
            <p class="text-center text-lg leading-relaxed">
              <HungarianText
                :text="store.current.hu"
                :before="store.target?.before"
                :after="store.target?.after"
                highlight
                :selected="inspected"
                @pick="inspect"
              />
              <SpeakButton :text="store.current.hu" class="ml-1 align-middle" />
            </p>
            <SentenceCrib
              :hu="store.current.hu"
              :key="store.current.cardId"
              :text="sentenceTranslation.text"
              :is-fallback="sentenceTranslation.isFallback"
              :model="store.current.sentenceModel"
            />
          </div>
        </div>
      </div>

      <!-- Grading -->
      <div v-if="store.phase === 'answer'" class="flex flex-col gap-2">
        <div v-if="!store.isProductive" class="grid grid-cols-4 gap-2">
          <Button
            v-for="r in ratingKeys"
            :key="r.n"
            :variant="r.variant"
            class="h-auto flex-col gap-0.5 py-4"
            :title="$t('review.memory.previewHint')"
            @click="grade(r.n)"
          >
            <span>{{ $t(`review.ratings.${r.key}`) }}</span>
            <!-- After a hint — «Подсказка» or the tutor before the answer —
                 the server caps the grade at Hard (ADR-034); the button says so. -->
            <span v-if="capped(r.key)" class="text-[10px] leading-tight text-uncertain">
              {{ $t('review.ratings.cappedHard') }}
            </span>
            <!-- What this grade schedules, from the scheduler's own preview.
                 Approximate, and absent in practice. -->
            <span v-if="previewLabel(effective(r.key))" class="font-mono text-[11px] tabular-nums opacity-80">
              ~{{ previewLabel(effective(r.key)) }}
            </span>
            <span class="font-mono text-[10px] opacity-60">{{ r.n }}</span>
          </Button>
        </div>
        <Button v-else class="w-full" @click="submitTyped">
          {{ $t('review.next') }}
          <span class="font-mono text-[10px] opacity-60">↵</span>
        </Button>
      </div>

      <Note v-if="store.error" variant="error">{{ store.error }}</Note>

      <!--
        «В корзину». Set below the ratings in weight and away from them in
        position: it is the rare action on this screen and must never be within
        a slip of the four buttons pressed two hundred times a day.

        Offered in BOTH phases on purpose. A card can be judged unusable from
        the question alone — a sentence the miner cut badly, a word three months
        out of reach — and making the learner reveal an answer they do not want
        first would be the app insisting on an interaction to accept a refusal.
      -->
      <div class="flex flex-wrap justify-center gap-2">
        <Button
          variant="ghost"
          size="sm"
          class="gap-1.5 text-xs text-muted-foreground hover:text-foreground"
          :disabled="store.trashing"
          :title="$t('review.trash.hint')"
          @click="trashCurrent"
        >
          <Trash2 class="size-3.5" aria-hidden="true" />
          {{ $t('review.trash.action') }}
          <span class="font-mono text-[10px] opacity-60">t</span>
        </Button>
      </div>

      <p class="text-center text-xs text-muted-foreground">{{ $t('review.keysInspect') }}</p>
      <KeyBar :items="keyHints" storage-key="tanulom.keybar.review" />
    </template>
    </div>

    <!-- The side column: any word, at any time — what it means, what shape it
         is in, and three sentences it really appears in — and the tutor,
         after an answer, with the question about this card already written. -->
    <div
      v-if="inspected || tutorOpen"
      class="flex w-full flex-col gap-4 lg:sticky lg:top-2 lg:max-h-[calc(100dvh-6rem)] lg:shrink-0 lg:overflow-y-auto"
      :class="tutorOpen ? 'lg:w-96' : 'lg:w-80'"
    >
      <WordInspector v-if="inspected" :key="inspected" :word="inspected" @close="inspected = ''" />
      <TutorAside
        v-if="tutorOpen && store.current"
        :chips="tutorChips"
        :card="tutorCard"
        :note="store.current.noteId"
        :phase="store.phase"
        :thread-id="tutorThreadId"
        @asked="onTutorAsked"
        @created="onTutorCreated"
        @deleted="tutorThread = null"
        @close="tutorOpen = false"
      />
    </div>
  </div>
</template>
