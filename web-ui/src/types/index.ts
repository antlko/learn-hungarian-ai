/**
 * Wire types. These mirror backend/internal/api/dto.go exactly (camelCase).
 * When a DTO shape changes, change both files and docs/API.md in the same commit.
 */

export interface Bootstrap {
  needsSetup: boolean
  packPresent: boolean
  packVersion?: string
  /** "starter" (embedded in the binary) or "full". */
  packEdition?: string
  locales: string[]
  version: string
  /** The password floor the server enforces (auth.MinPasswordLength). */
  minPassword: number
}

export interface User {
  id: number
  email: string
  role: 'admin' | 'learner'
  createdAt: string
}

export interface Me {
  user: User
  settings: Settings
}

export type Role = User['role']

export interface CreateUserRequest {
  email: string
  password: string
  role: Role
}

/**
 * A partial edit: an omitted field is left alone, which is how a role changes
 * without resending a password the UI has never seen.
 */
export interface UpdateUserRequest {
  role?: Role
  password?: string
}

export interface ChangePasswordRequest {
  currentPassword: string
  newPassword: string
}

export type UILocale = 'ru' | 'en'
export type GlossLang = 'ru' | 'en' | 'both'
export type Goal = 'reading' | 'conversation' | 'exam'

/** The five review directions. Each carries independent FSRS state per note. */
export type DirectionName = 'recognize' | 'produce' | 'listen' | 'cloze' | 'spell'

export interface Direction {
  enabled: boolean
  weight: number
}

export interface Settings {
  locale: { ui: UILocale; gloss: GlossLang; glossFallback: boolean }
  review: {
    desiredRetention: number
    newPerDay: number
    maxReviewsPerDay: number
    dailyMinutes: number
    goal: Goal
    directions: Record<string, Direction>
    burySiblings: boolean
    /** Accept typed answers missing only diacritics. Default true; rated Hard. */
    accentTolerance: boolean
  }
  mining: {
    knownThreshold: number
    featureWeight: number
    targetUnknown: number
    unknownSigma: number
    minCoverage: number
    targetLength: [number, number]
  }
  /** The blocked -> interleaved gate. Exposed on purpose: see docs/PEDAGOGY.md §2.1. */
  drill: {
    gateThreshold: number
    gateWindow: number
    masteryThreshold: number
    masteryWindow: number
    hysteresis: number
    definiteObjectPct: number
    demonstrativePct: number
    harmonyLatencyMs: number
  }
  hvpt: { minTalkers: number; trialsPerSession: number; rateJitter: number }
  curriculum: { focusUnlockListeningHours: number; examDate: string }
  services: Record<string, Service>
  /** Where the optional full language pack downloads from; empty sha256 = not published. */
  langpack: { url: string; sha256: string }
  /** Presence only — a secret's value never reaches the client. */
  secrets: Record<string, boolean>
}

/**
 * `device` is TTS spoken by this browser through the Web Speech API. It is
 * valid for `tts` only, takes no endpoint, and is the default — see
 * `config.ProviderDevice` and `lib/speech.ts`.
 */
export type ServiceProvider = 'off' | 'device' | 'local' | 'openai' | 'managed'

export interface Service {
  provider: ServiceProvider
  endpoint: string
  model?: string
  voices?: string[]
  /** Output only: set through `api.setServicePaused`, never through the settings PUT. */
  paused?: boolean
}

export interface UpdateSettingsRequest {
  locale?: Settings['locale']
  review?: Settings['review']
  mining?: Settings['mining']
  drill?: Settings['drill']
  hvpt?: Settings['hvpt']
  curriculum?: Settings['curriculum']
  services?: Record<string, Service>
  langpack?: Settings['langpack']
  /** Blank value keeps the stored secret; a new value replaces it. */
  secrets?: Record<string, string>
}

export interface Clamp {
  field: string
  from: number
  to: number
}

export interface UpdateSettingsResponse {
  settings: Settings
  clamped?: Clamp[]
}

export type CapabilityName = 'tts' | 'asr' | 'llm' | 'mt'

/**
 * One machine translation.
 *
 * `model` is not decoration and is not optional in practice: this is the only
 * text in the app that no person and no corpus stands behind, so the screen
 * names who wrote it. See ADR-014. `cached` explains how an answer arrived
 * with the translator switched off.
 */
export interface Translation {
  source: string
  text: string
  model?: string
  cached: boolean
}

/** What the last press of «Проверить» found, per service. */
export interface CheckResult {
  at: Date
  reachable: boolean
  reason?: string
}

export interface Capability {
  name: CapabilityName
  provider: ServiceProvider
  configured: boolean
  reachable: boolean
  latencyMs?: number
  detail?: string
  /** Why a configured service could not be reached (a code; the client words it). */
  reason?: string
  /** What an llm endpoint says it serves (`GET /v1/models`) — the picker's list. */
  models?: string[]
  /** The admin switched this service off (ADR-068); provider and endpoint are kept. */
  paused?: boolean
}

// ─── Review ────────────────────────────────────────────────────────────────

export type Certainty = 'certain' | 'uncertain' | 'derived' | 'unknown' | 'corrected'

export interface Analysis {
  form: string
  lemma: string
  lemmaId: number
  upos: string
  feats: Record<string, string>
  featureIds: string[]
  source: string
  certainty: Certainty
  freqRank?: number
  alternatives: number
}

export interface AnalyzedToken {
  index: number
  text: string
  start: number
  end: number
  isWord: boolean
  analysis: Analysis
}

export interface AnalyzeResponse {
  tokens: AnalyzedToken[]
  sentences: { start: number; end: number }[]
  coverage: number
  wordCount: number
}

/**
 * The sentence either side of the target word, split by the server.
 *
 * The offsets behind it are byte offsets into UTF-8 and JavaScript indexes
 * UTF-16, so the split has to happen where the bytes are. Never reconstruct it
 * from `hu` on this side.
 */
export interface TargetSplit {
  before: string
  after: string
}

/**
 * Where a card is in its life, in the words Anki users already read. `young`
 * and `mature` split at a 21-day interval — a reporting convention, not a
 * memory threshold, and the UI says so. ADR-026.
 */
export type MemoryStage = 'new' | 'learning' | 'young' | 'mature' | 'relearning'

/** What each grade would schedule, as instants. Approximate: fuzz is seeded at answer time. */
export interface MemoryPreview {
  /** When the figures were computed; an interval is `due − at`, never `due − now`. */
  at: string
  again: string
  hard: string
  good: string
  easy: string
}

/**
 * What a card knows about itself, for the learner to see while answering.
 * Every figure is measured or computed by the scheduler's own library — the
 * chance of recall, the interval, the tallies from the review log. There is
 * no score in here. [R18], ADR-026.
 */
export interface Memory {
  stage: MemoryStage
  /** 0–1; the same "chance of recalling it now" the words browser shows. */
  retrievability: number
  stabilityDays: number
  intervalDays: number
  reps: number
  lapses: number
  /** This card's answers in the review log. */
  correct: number
  total: number
  /** Every card of the word. */
  wordCorrect: number
  wordTotal: number
  /** The last outcomes, oldest first, at most eight. */
  history: boolean[]
  /** Absent in free practice, where nothing is scheduled. */
  preview?: MemoryPreview
  /**
   * Where the card stands on FSRS's minute-scale learning steps ({1, 10}
   * minutes by default, {10} when relearning). Both 0 once it is scheduled in
   * days. `stepsLeft` is the scheduler's own `RemainingSteps`, not a count the
   * app keeps.
   */
  stepsLeft: number
  stepsTotal: number
  /** 0–100: where the word is now (ADR-070). Falls on a mistake; 100 is about half a year between showings. */
  progress: number
  /**
   * Whether a «Хорошо» from here would take the card off the minute steps and
   * onto days — asked of the scheduler, not guessed from the interval. False
   * in free practice, where nothing is scheduled at all.
   */
  graduatesOnGood: boolean
}

export interface QueueItem {
  cardId: number
  noteId: number
  /** Course unit code when the note came from enrolling a unit; absent otherwise. Display-only. */
  unit?: string
  /** A course card whose sentence a model wrote and the learner accepted (ADR-073); the card says so. */
  aiWritten?: boolean
  direction: DirectionName
  isNew: boolean
  due: string
  /** The card's memory figures. Absent only from an older server. */
  memory?: Memory
  hu: string
  targetForm: string
  /** Absent when the note has no located target span. */
  target?: TargetSplit
  targetFeats: string[]
  /** The TARGET WORD's dictionary gloss. */
  glossRu: string
  glossEn: string
  /**
   * What the WHOLE SENTENCE means, where the pack has a translation.
   * Absent is normal — the pack translates 19,641 of its sentences, not all.
   */
  sentenceRu?: string
  sentenceEn?: string
  /**
   * The MACHINE that wrote sentenceRu/sentenceEn, and empty for a pack
   * translation — those were written by people and there is no model to name.
   *
   * A word mined from a text has no pack sentence to borrow from, so its crib
   * comes from the MT sidecar. ADR-014 makes this label the entire defence: a
   * wrong translation is perfectly well-formed, so it has to say who wrote it,
   * beside the text and never in a tooltip.
   */
  sentenceModel?: string
  lemmaId?: number
  /** Dictionary form of the target word — the cue a cloze cannot be answered without. */
  lemma?: string
  /**
   * The string this card is graded against, as the SERVER computes it.
   *
   * Here so «Подсказка» can reveal the answer without grading the card. Never
   * re-derive it in the client: `produce` grades the dictionary form and
   * `cloze` the form in the sentence, and a second copy of that rule is how the
   * hint comes to show one word while the grader wants another. See ADR-017.
   */
  expected?: string
  /** 3-5 shuffled candidates including `expected`, for tap-to-answer; absent means type. ADR-060. */
  options?: string[]
}

/**
 * What is waiting, in BOTH units.
 *
 * A note is a word to learn; a card is one direction of it, and a note spawns
 * three to five. "59 new cards waiting" was accurate and useless to a learner
 * who had taken up 27 words — there was no way to turn one number into the
 * other. Lead with words, explain in cards.
 */
export interface QueueCounts {
  /** The served scope's dueNow: what THIS session serves, post-burial. */
  due: number
  /** New CARDS in the deck (raw pool). */
  new: number
  total: number
  /**
   * Every new word this build considered, on one basis: shown now · deferred
   * because a sibling was already due or answered today · held back by the
   * daily budget alone. Deferrals are NOT the limit's doing — the old screen
   * blamed Settings for them.
   */
  newShownNotes: number
  newDeferredNotes: number
  newBacklogNotes: number
  /** The global daily budget: started today, the cap (0 = unlimited), left. */
  introducedToday: number
  newPerDay: number
  newBudgetLeft: number
  /** Learning steps maturing before local midnight, and the earliest of them. */
  dueLaterToday: number
  nextDueAt?: string
  /** Due items maxReviewsPerDay trimmed from this build. */
  reviewsCapped?: number
}

/** Which pile of the deck a session is drawn from. */
export type ReviewScope = 'all' | 'course' | 'reader'

/**
 * A queue request's scope: one of the three piles, the words mined from one
 * text — a SUB-slice of the reader pile, never a fourth one (ADR-022) — or the
 * trash.
 *
 * `trash` is accepted by FREE PRACTICE only. The review queue refuses it, and
 * the server refuses it too: a trash the scheduler drew from would be a pile
 * cards come back from, which is the opposite of what the button promised.
 * ADR-025.
 */
export type QueueScope = ReviewScope | 'trash' | `text:${number}`

/**
 * One pile's numbers, in the single vocabulary every screen shares (ADR-021):
 * post-burial and day-granular, so a number on screen is a number the queue
 * will honour. dueNow is the session; dueLaterToday are learning steps that
 * return today; buriedToday is where due-but-hidden cards are counted instead
 * of inside a bigger figure — «26 к повторению» over a 10-item session was
 * exactly that.
 */
export interface Breakdown {
  scope: ReviewScope | string
  dueNow: number
  dueLaterToday: number
  nextDueAt?: string
  buriedToday: number
  newCards: number
  newNotes: number
}

/**
 * Free practice: retrieval without scheduling.
 *
 * A separate mode, not a second queue. Nothing here moves a due date, so the
 * response deliberately carries no schedule — see ADR-019.
 */
export interface PracticeQueueResponse {
  items: QueueItem[]
  scope: QueueScope
  /**
   * What was actually served: weak falls back to random on a deck with nothing
   * reviewed yet, and the screen prints the criterion really applied (ADR-024).
   */
  mode: 'random' | 'weak'
  /** How many words the pile could offer at all, so an empty one can say so. */
  pool: number
}

export interface PracticeAnswerResponse {
  correct: boolean
  diff?: Diff
  expected?: string
  acceptedWithAccents?: boolean
  /** False for «Знаю» words and new cards: the answer was logged but moved nothing (ADR-064). */
  counted: boolean
  reason?: 'known' | 'new'
  rating?: number
  nextDue?: string
  intervalDays?: number
  /** Absent = none. */
  gradedFeatures?: FeatureGrade[]
  memory?: Memory
}

/** Practice volume AND accuracy: a count alone only ever goes up. */
export interface PracticeStats {
  today: number
  todayRight: number
  total: number
  totalRight: number
}

/** One local day of practice, for the Progress series. */
export interface PracticeDay {
  day: string
  total: number
  right: number
}

/** One direction's all-time practice tally. */
export interface PracticeDirection {
  direction: DirectionName
  total: number
  right: number
}

/** One word of the deck, as the words browser lists it. */
export interface WordRow {
  lemmaId: number
  lemma: string
  glossRu: string
  glossEn: string
  status: LemmaStatus
  firstSeen?: string
  /** Chance of recalling it right now — the strongest card's figure. */
  retrievability: number
  nextDue?: string
  lapses: number
  cards: number
  /**
   * Where the word is in its life, in the review card's vocabulary. `known` is
   * the learner's «Знаю»; otherwise the strongest card's stage (young/mature
   * split at a 21-day interval). Empty only for an ignored word with no cards.
   */
  stage: WordStage | ''
  /** The strongest card's figures, from srs.Memory. */
  intervalDays: number
  reps: number
  stepsLeft: number
  stepsTotal: number
  /** 0–100; omitted for «Знаю» words (ADR-070). */
  progress?: number
}

export type WordStage = 'learning' | 'relearning' | 'young' | 'mature' | 'known'

export interface WordsResponse {
  words: WordRow[]
  total: number
  page: number
  pageSize: number
}

/** One word the learner set aside, as the trash screen lists it. */
export interface TrashedNote {
  noteId: number
  hu: string
  targetForm: string
  glossRu: string
  glossEn: string
  lemmaId?: number
  /** The dictionary form — a sentence is not a word's name. */
  lemma?: string
  /** `course:<code>` or `manual`. Never printed raw: a unit code is not a name. */
  source: string
  trashedAt: string
  /** Directions that come back with a restore — what it costs the day. */
  cards: number
}

export interface TrashListResponse {
  notes: TrashedNote[]
  total: number
}

/**
 * The result of trashing or restoring one word.
 *
 * `scopes` is fresh Breakdowns, so the tab strip updates from the server rather
 * than by subtracting one from a number it is holding — two numbers on one
 * screen disagreeing is the failure ADR-021 exists to prevent.
 */
export interface TrashResponse {
  noteId: number
  cards: number
  /** False when the word was already where the call would put it. */
  changed: boolean
  trashed: number
  scopes: Breakdown[]
}

/** How the served queue is arranged: overdue first, or a day-stable shuffle. ADR-026. */
export type ReviewOrder = 'due' | 'random'

export interface QueueResponse {
  items: QueueItem[]
  counts: QueueCounts
  /** What was actually served — echoed, never assumed from the request. */
  scope: ReviewScope
  /** Every pile, so the control can label the tab you are not standing on. */
  scopes: Breakdown[]
  /** The order actually applied, echoed for the same reason as `scope`. */
  order?: ReviewOrder
}

export type ErrorClass =
  | ''
  | 'harmony'
  | 'vowel_length'
  | 'vowel_quality'
  | 'case'
  | 'definiteness'
  | 'possessor'
  | 'stem'
  | 'spelling'
  | 'unknown'

/** The morpheme-level comparison of a typed answer against the target. */
export interface Diff {
  typed: string
  expected: string
  match: boolean
  nearMatch: boolean
  class: ErrorClass
  wrongFeature?: string
  commonPrefix: number
  commonSuffix: number
  /** An i18n key, not prose — the backend never picks a language. */
  hint?: string
  hintArgs?: Record<string, string>
}

export interface FeatureGrade {
  FeatureID: string
  Rating: number
  Correct: boolean
}

export interface AnswerResponse {
  correct: boolean
  rating: number
  nextDue: string
  intervalDays: number
  gradedFeatures: FeatureGrade[]
  diff?: Diff
  expected?: string
  /** Correct by tolerance: the right word minus its diacritics. Never silent. */
  acceptedWithAccents?: boolean
  /** The card's memory AFTER this answer, so the interval change can be shown. */
  memory?: Memory
  /** Practice only: false when the answer moved nothing (ADR-064). */
  counted?: boolean
  reason?: 'known' | 'new'
}

export interface UndoResult {
  cardId: number
  noteId: number
  reviewTime: string
  features: string[]
}

/** One piece of a segmented form. Boundaries are verified, never guessed. */
export interface Segment {
  form: string
  kind: 'stem' | 'linking' | 'suffix' | 'ending'
  featureId?: string
  featureIds?: string[]
  allomorphRule?: string
  /** A stem alternation such as "a→á". */
  note?: string
}

export interface ExplainLine {
  surface: string
  kind: Segment['kind']
  featureId?: string
  labelRu?: string
  labelEn?: string
  ruleRu?: string
  ruleEn?: string
}

/**
 * One attested sentence using a lemma, for the word inspector.
 *
 * `form` is the surface shape the lemma takes in THIS sentence, which is the
 * part worth pointing at: seeing `nagyon` among `nagy`'s examples is the lesson.
 */
export interface Example {
  hu: string
  ru?: string
  en?: string
  form?: string
}

/** One attested pack sentence shown on a course unit (display-only; never a note). */
export interface UnitPhrase {
  id: number
  hu: string
  ru: string
  en?: string
}

/** The full decomposition of one form — [R4]'s explicit instruction in miniature. */
export interface Breakdown {
  form: string
  analysis: Analysis
  segments: Segment[]
  explain: ExplainLine[]
}

export interface AnswerRequest {
  cardId: number
  rating?: number
  typed?: string
  usedHint?: boolean
  /** How the answer was entered; a chosen option is sent as `typed`. ADR-060. */
  via?: 'typed' | 'choice'
  durationMs: number
}

export interface MorphFeature {
  id: string
  key: string
  value: string
  labelRu: string
  labelEn: string
  allomorphs: Record<string, string>
  explainRu: string
  explainEn: string
  teachOrder: number
  grid?: string
}

export interface FeatureMastery {
  featureId: string
  retrievability: number
  accuracy: number
  reps: number
  observations: number
  medianMs: number
  /** Correct AND fast. Matters for motor skills like vowel harmony. */
  automated: boolean
  /** `accuracy` rests on enough answers to report — `drill.gateWindow` of them.
   *  Below this the figure is an estimate off three lucky answers, and the
   *  progress screen says so instead of drawing a full bar. */
  ready: boolean
  /** Ready AND under `drill.masteryThreshold`: a measured weakness. */
  weak: boolean
  /** Codes of the grammar topics that teach this feature; `[]` if none. */
  topics: string[]
}

/** One CEFR band's standing in the level estimate (GET /api/stats/level). */
export interface LevelBand {
  band: 'A1' | 'A2' | 'B1' | 'B2' | 'C1'
  counted: number
  total: number
  /** counted / total */
  share: number
  /** The share at which the band's vocabulary counts as reached (0.8). */
  target: number
  /** At least half the band's drillable topics are interleaved or mastered. */
  grammarReady: boolean
  grammarDrillable: number
  reached: boolean
}

/**
 * A MEASURED estimate with a stated method (ADR-047), never a score: it can go
 * down. `coverage` is the share of everyday running text made of counted words.
 */
export interface LevelEstimate {
  /** "A2.2"; "C1+" once every measured band is reached. */
  label: string
  band: string
  sublevel: number
  confidence: 'low' | 'estimate'
  coverage: number
  countedLemmas: number
  bands: LevelBand[]
  /** Checkpoint code ("m3") holding the working band shut, or "". */
  cappedBy: string
  /** i18n key of the fixed method sentence. */
  methodKey: string
}

export interface BandWord {
  lemmaId: number
  lemma: string
  glossRu: string
  glossEn: string
  freqRank: number
}

export interface BandWordsResponse {
  words: BandWord[]
}

/**
 * The translation sidecar's own account of itself.
 *
 * `configured` is a settings question, `reachable` a deployment one, and they
 * send the learner to different remedies — which is why they are two fields and
 * not one. `state` is what the service says once it answers at all.
 */
export interface MTStatus {
  configured: boolean
  reachable: boolean
  /** 'loading' | 'ready' | 'failed'. Absent when we could not ask. */
  state?: string
  /** The service's own failure reason, passed through verbatim. */
  detail?: string
  model?: string
  /** Zero means the service could not measure it — draw an indeterminate bar,
   *  never "0%". */
  downloadedBytes?: number
  totalBytes?: number
}

/**
 * A meaning the learner keeps for a word the pack does not have.
 *
 * `source` survives storage on purpose: a meaning a translation model produced
 * stays labelled as the model's for as long as it exists, and never quietly
 * becomes something a person appears to stand behind. [ADR-014]
 */
export interface UserGloss {
  form: string
  /** Empty when nothing has been saved for this word. */
  gloss: string
  source: 'manual' | 'machine' | ''
  model?: string
}

export interface ForecastDay {
  date: string
  reviews: number
  newCards: number
  minutes: number
}

export interface Forecast {
  days: ForecastDay[]
  peakReviews: number
  peakMinutes: number
  steadyStateReviews: number
  steadyStateMinutes: number
  budgetMinutes: number
  sustainable: boolean
  suggestedNewPerDay: number
}

export interface GovernorVerdict {
  allowed: boolean
  requested: number
  suggested: number
  reason: string
  forecast: Forecast
}

// ─── Reading ───────────────────────────────────────────────────────────────

export type LemmaStatus = 'new' | 'learning' | 'known' | 'ignored'

export type CEFRLevel = 'A1' | 'A2' | 'B1' | 'B2' | 'C1' | 'C2'

/**
 * A ready-made reading set (GET /api/library/sets): «Фразы» (real sentences a
 * person wrote) or «Диалог» (`track: 'dialogue'`, written by an AI, `authored`).
 */
export interface LibrarySet {
  code: string
  level: 'A1' | 'A2' | 'B1'
  track: 'core' | 'theme' | 'dialogue'
  title: string
  titleRu: string
  titleEn: string
  theme: string
  sentences: number
  /** Written by an AI for the app, not checked by a native speaker: always labelled. */
  authored: boolean
  imported: boolean
  textId?: number
}

/** One line of a set in the preview; either translation may be missing. */
export interface LibrarySetLine {
  hu: string
  ru?: string
  en?: string
}

/** GET /api/library/sets/:code — the set as it would be imported, in order. */
export interface LibrarySetDetail {
  code: string
  level: 'A1' | 'A2' | 'B1'
  track: 'core' | 'theme' | 'dialogue'
  title: string
  titleRu: string
  titleEn: string
  theme: string
  authored: boolean
  sentences: LibrarySetLine[]
}

export interface TextSummary {
  id: number
  title: string
  sourceUrl: string
  kind: 'paste' | 'url' | 'srt' | 'epub' | 'generated'
  wordCount: number
  lemmaCoverage: number
  elementCoverage: number
  addedAt: string
  /** Set when this text is a listening material. */
  audioPath: string
  /**
   * Estimated CEFR band, '' while unestimated or when the estimator abstained.
   * `levelSource` says WHO estimated — heuristic | llm | manual — and the UI
   * always shows it: a machine estimate must never look like a fact (ADR-023).
   */
  level: CEFRLevel | ''
  levelSource: 'heuristic' | 'llm' | 'manual' | ''
  levelDetail?: string
  /** How many of the learner's deck words were mined from this text. */
  noteCount: number
}

export interface StoredToken {
  index: number
  sentIdx: number
  form: string
  start: number
  end: number
  isWord: boolean
  lemmaId: number
  feats: string[] | null
  certainty: Certainty
}

/**
 * Two coverage numbers, deliberately.
 *
 * `lemmaCoverage` is comparable to the published 95%/98% figures — which were
 * derived from English word families. `elementCoverage` additionally requires
 * every suffix on the token to be known, and is the honest number for an
 * agglutinative language: you can know `ház` perfectly and still be stopped by
 * `házaimban`.
 */
export interface Coverage {
  words: number
  knownLemmas: number
  knownElements: number
  unresolved: number
  lemmaCoverage: number
  elementCoverage: number
  unknownLemmaIds: number[]
  unknownFeatures: string[]
}

export type CoverageVerdict = 'too_hard' | 'with_help' | 'comfortable'

export interface TextDetail {
  text: TextSummary & { body: string }
  tokens: StoredToken[]
  coverage: Coverage
  verdict: CoverageVerdict
  known: Record<string, LemmaStatus>
  thresholds: { minimal: number; unassisted: number; listening: number }
}

export interface ScoredSentence {
  sentIdx: number
  text: string
  score: number
  unknownLemmas: number[]
  unknownFeatures: string[]
  unknownWeighted: number
  coverage: number
}

export interface LemmaDetail {
  id: number
  lemma: string
  upos: string
  ipa?: string
  freqRank?: number
  cefr?: string
  glosses: Record<string, string[]>
  glossFallback: boolean
  paradigm?: Record<string, string>
  /** Absent when the word is not in the deck (ADR-070). */
  memory?: LemmaMemory
}

/** The word's memory on GET /api/lemma/:id: its best card's figures plus each card. */
export interface LemmaMemory {
  progress: number
  stage: MemoryStage
  nextDue?: string
  intervalDays: number
  reps: number
  lapses: number
  retrievability: number
  /** Newest last, at most eight. */
  history: boolean[]
  cards: { direction: DirectionName; progress: number; stage: MemoryStage }[]
}

// ─── Curriculum & grammar drills ───────────────────────────────────────────

/**
 * The blocked → interleaved gate, as five phases.
 *
 * `explained` is the honest one: some topics have a real rule card and no drill
 * generator behind them (the alphabet, politeness registers, preverb
 * detachment). They teach by explanation and deliberately do not gate what comes
 * after — see [R5] and backend/internal/drill/schedule.go.
 */
export type Phase = 'locked' | 'explained' | 'blocked' | 'interleaved' | 'mastered'

/** The exercise types. Each has its own renderer in components/grammar. */
export type DrillKind = 'allomorph' | 'case' | 'conjugation' | 'cloze' | 'transform' | 'mcq'

/**
 * The gate's verdict, with everything needed to explain it.
 *
 * `reason` is an i18n key and `args` are its substitutions — the backend never
 * picks a language. A phase badge without a reason is worse than no badge: a
 * learner who does not know why interleaving is being held back assumes the app
 * is broken.
 */
export interface GateDecision {
  phase: Phase
  from: Phase
  reason: string
  args?: Record<string, string>
  current: number
  target: number
  observations: number
  window: number
  /** False while the estimate is too thin to act on. */
  ready: boolean
  demoted: boolean
  /**
   * What is worth finishing first. A RECOMMENDATION — the topic is practised
   * either way. Prerequisites stopped being locks because a floor nobody can
   * cross is a wall: see `drill.Decision.Advice`.
   */
  advice?: string
  adviceArgs?: Record<string, string>
}

export interface TopicProgress {
  blockedAccuracy: number
  blockedTotal: number
  interleavedAccuracy: number
  interleavedTotal: number
  medianMs: number
  attempts: number
  correct: number
  fallbacks: number
}

export interface DrillAvailability {
  available: boolean
  kinds: DrillKind[]
  /** An i18n key explaining an unavailable topic. Never an empty screen. */
  reason?: string
}

export interface Topic {
  code: string
  level: 'A1' | 'A2' | 'B1' | 'B2'
  titleRu: string
  titleEn: string
  prereqCodes: string[]
  unlocks: string[]
  featureIds: string[]
  teachOrder: number
  /** Mastered on latency, not accuracy alone. [R15] */
  motorSkill: boolean
  phase: Phase
  decision: GateDecision
  progress: TopicProgress
  drills: DrillAvailability
}

export interface GateConfig {
  threshold: number
  window: number
  masteryThreshold: number
  masteryWindow: number
  hysteresis: number
  harmonyLatencyMs: number
}

export interface CurriculumResponse {
  topics: Topic[]
  current?: string
  placementTaken: boolean
  gate: GateConfig
}

export interface TopicFeature {
  id: string
  labelRu: string
  labelEn: string
  explainRu: string
  explainEn: string
  allomorphs: Record<string, string>
  grid?: string
  /** False when never graded — the UI shows a dash, not 0%. */
  seen: boolean
  retrievability: number
  accuracy: number
  observations: number
  medianMs: number
  automated: boolean
}

export interface TopicDetail {
  topic: Topic
  /** Markdown, both locales, always. The rule card comes first. [R4] */
  explainRu: string
  explainEn: string
  features: TopicFeature[]
  blocking?: string[]
}

export interface DrillOption {
  text: string
  /** False means this string is not a Hungarian word at all. */
  attested: boolean
}

/** One exercise. The expected answer is never included — the server holds it. */
export interface DrillItem {
  id: string
  topicCode: string
  kind: DrillKind
  phase: Phase
  promptKey: string
  promptArgs?: Record<string, string>
  /** The Hungarian frame with `___` where the answer goes. */
  frame?: string
  lemma?: string
  lemmaId?: number
  glossRu?: string
  glossEn?: string
  /** Empty means the learner types the answer. */
  options?: DrillOption[]
  /** The features this item requires — the only ones a wrong answer charges. */
  featureIds: string[]
  headline?: string
  attested: boolean
  sentenceId?: number
}

/** What a mistake says to practise next — including "a drill cannot fix this". */
export type Remedy =
  | 'drill.remedy.volume'
  | 'drill.remedy.rule'
  | 'drill.remedy.ear'
  | 'drill.remedy.vocabulary'
  | 'drill.remedy.none'

export interface DrillTarget {
  featureId: string
  topicCode: string
  kinds: DrillKind[]
  remedy: Remedy
  misses: number
}

export interface DrillNextResponse {
  topic: Topic
  items: DrillItem[]
  followUp?: DrillTarget[]
  /** An i18n key explaining an empty batch. */
  unavailable?: string
}

export interface DrillAnswerRequest {
  itemId: string
  answer: string
  usedHint?: boolean
  durationMs: number
}

export interface DrillAnswerResponse {
  correct: boolean
  /** Correct by tolerance: the right form minus its diacritics. */
  acceptedWithAccents?: boolean
  expected: string
  rating: number
  diff?: Diff
  gradedFeatures: FeatureGrade[]
  progress: TopicProgress
  decision: GateDecision
  followUp?: DrillTarget[]
  topic: Topic
  /** Post-answer state of the features this item graded, so the numbers on
   *  screen move together instead of drifting apart. */
  featureState: FeatureMastery[]
}

export interface PlacementResult {
  code: string
  answered: number
  correct: number
}

export interface PlacementSeed {
  code: string
  phase: Phase
  accuracy: number
  observations: number
}

export interface PlacementResponse {
  done: boolean
  topic?: Topic
  items: DrillItem[]
  index: number
  total: number
  results: PlacementResult[]
  seeds?: PlacementSeed[]
}

// ─── Ear trainer (HVPT) ────────────────────────────────────────────────────

export interface HvptPair {
  id: number
  a: string
  b: string
  contrast: string
  priority: number
  glossARu: string
  glossBRu: string
  glossAEn: string
  glossBEn: string
}

/** One 2AFC trial. The played member is NOT here — the server holds it. */
export interface HvptTrial {
  id: string
  pair: HvptPair
  audioUrl: string
  talkerId: string
  rate: number
}

export interface HvptSession {
  trials: HvptTrial[]
  talkers: number
  today: { trials: number; correct: number; talkers: number }
  /** The honest refusal: fewer distinct talkers than the floor requires. */
  refused?: { have: number; need: number }
}

export interface HvptAnswer {
  correct: boolean
  played: 'a' | 'b'
  accuracy: number
  total: number
  /** Both members' clips from the trial's talker, for after-answer replay. */
  audioA?: string
  audioB?: string
}

export interface HvptContrast {
  contrast: string
  seen: boolean
  accuracy: number
  total: number
  due: boolean
}

// ─── Listening ─────────────────────────────────────────────────────────────

export interface ListenState {
  pass: 1 | 2 | 3
  positionS: number
  listenedS: number
}

// ─── Progress ──────────────────────────────────────────────────────────────

export interface DailyLedger {
  day: string
  reviews: number
  drills: number
  trials: number
  listenS: number
  /**
   * Seconds spent in the reader. Added by migration 00020 — before it, reading
   * reported no time at all and an hour with a text left the day's minutes
   * unmoved. Optional on the type so a response from an older server still
   * parses. ADR-044.
   */
  readS?: number
  activeMs: number
  dueLeft: number
  human: boolean
}

export interface CheckpointSpec {
  code: string
  lemmaTarget: number
  gatesLevel: string
  attested: string[]
}

export interface CheckpointEvidence {
  lemmas: number
  lemmaTarget: number
  missedDays30: number
  avgCoverage30: number
  humanDays30: number
  listeningHours: number
  attested?: string[]
}

export interface Checkpoint {
  id: number
  code: string
  status: 'passed' | 'failed'
  evidence: CheckpointEvidence
  notes: string
  decidedAt: string
}

export interface Overview {
  lemmas: {
    known: number
    learning: number
    curve: { day: string; count: number }[] | null
    targets: Record<string, number>
  }
  streak: { current: number; best: number; todayMet: boolean }
  today: DailyLedger
  /**
   * Free practice, reported SEPARATELY from reviews because it means something
   * different: it schedules nothing, so one number must not be mistaken for
   * progress through the deck. See ADR-019.
   */
  practice: PracticeStats
  /** Practice series for the Progress screen — still schedules nothing. */
  practiceDays: PracticeDay[]
  practiceByDirection: PracticeDirection[]
  /** The recent daily ledger, newest first — the Today load sparkline. */
  days: DailyLedger[]
  /**
   * The whole deck's Breakdown — the same numbers, in the same coordinate
   * system, that the review tabs show (ADR-021). dueNow duplicates
   * queue.dueNow for older callers.
   */
  queue: Breakdown
  dueNow: number
  dailyMinutes: number
  listeningHours: number
  checkpoints: Checkpoint[]
  nextCheckpoint?: { spec: CheckpointSpec; evidence: CheckpointEvidence }
  blockedLevels: string[]
  examDate?: string
}

export interface CheckpointVerdict {
  checkpoint: Checkpoint
  diagnostic?: { findings: { key: string; value: number }[] }
}

// ─── The guided course ──────────────────────────────────────────────────────

export interface CourseWord {
  lemma: string
  upos: string
  glossRu: string
  glossEn: string
  /** Set only where something will trip you: an irregular stem, a register trap. */
  note: string
}

export interface CourseUnit {
  code: string
  month: number
  ord: number
  titleRu: string
  titleEn: string
  goalRu: string
  goalEn: string
  situationRu: string
  situationEn: string
  /** Grammar topics this unit leans on — codes into the curriculum DAG. */
  topicCodes: string[]
  /** 'core' (or absent) for the main road; 'theme' for a side unit (later batch). */
  track?: string
  /** Difficulty tier the road groups by. Absent (older server) means 'basic'. */
  tier?: CourseTier
  words: CourseWord[]
}

export type CourseTier = 'basic' | 'intermediate' | 'advanced'

/**
 * Where a unit stands. `ahead` is deliberately not `locked`: nothing in the API
 * stops a learner enrolling on unit 20 in week one, so the UI must not draw a
 * padlock it cannot back up. See internal/course/course.go.
 */
export type UnitStatus = 'done' | 'current' | 'ahead'

export interface UnitState {
  unit: CourseUnit
  status: UnitStatus
  enrolled: boolean
  wordsTotal: number
  /**
   * How many of the unit's words are in the review deck. A COUNT, not a per-word
   * flag — which is why the word list only marks state where it can prove it.
   */
  wordsInPlay: number
  /**
   * The subset of `wordsInPlay` whose notes THIS unit created
   * (`note.source = 'course:<code>'`).
   *
   * `wordsInPlay` counts a lemma wherever it came from, which is right — but
   * it was the only progress a plan row showed, so a word added from the
   * reader raised a later month's fraction and the screen read as "you have
   * started month 2". The difference is what the row now explains.
   */
  fromHere?: number
  /** Unit words actually learned (graduated or known). Absent means 0. */
  learned?: number
  /** Unit words settled: `known`, or the strongest live card is mature (>= unlock.settledDays). Absent means 0. ADR-058. */
  settled?: number
  /** The unit's words with no card yet — named, so the UI never infers them. */
  notInPlay: string[]
  /** Not-in-deck words of an ENROLLED unit that the pack has no sentence to illustrate (derived per request; ADR-072). */
  noExample: string[]
}

export interface CoursePlan {
  units: UnitState[]
  /** The unit to take NEXT (ADR-048: follows enrolment — first enrolled and
   *  unfinished, else first untouched). Empty once the course has handed over. */
  current: string
  handedOver: boolean
  completed: number
  total: number
}

export interface CourseProgress {
  unitCode: string
  enrolled: boolean
  notesMade: number
  /** How many words were skipped, as a count. Which ones is in EnrolResult. */
  skipped: number
}

export interface CourseCounts {
  words: number
  inDeck: number
  learned: number
  /** Settled words (ADR-058). Absent on an older server. */
  settled?: number
}

export interface CourseTotals extends CourseCounts {
  perTier: Record<CourseTier, CourseCounts>
}

export interface CourseResponse {
  plan: CoursePlan
  /** Absent on an older server: the client then sums the units. */
  totals?: CourseTotals
  /** The soft gate's rule (ADR-058); absent on an older server. Advice only. */
  unlock?: { share: number; settledDays: number }
  enrolled: Record<string, CourseProgress>
}

/** What taking a unit into work actually produced — including what it did not. */
/**
 * A course word that got no card — WITH its meaning.
 *
 * The bare Hungarian string was all this used to carry, so the panel named
 * three words to the learner and told them nothing about any of them.
 */
export interface SkippedWord {
  lemma: string
  glossRu: string
  glossEn: string
  /** Left out because the learner marked the word «Знаю», not for lack of an example. */
  known?: boolean
}

export interface CourseProposal {
  hu: string
  tr: string
}

/** A model's proposed sentences for one unit word. Nothing here is stored until accepted. */
export interface CourseProposeResult {
  lemma: string
  model: string
  lang: string
  proposals: CourseProposal[]
}

export interface EnrolResult {
  unitCode: string
  notesMade: number
  duplicate: number
  /** Words the pack has no attested sentence for, so no card was made. */
  skipped: SkippedWord[]
}

// ─── The hand-off catalogue ─────────────────────────────────────────────────

export type ResourceKind = 'news' | 'reading' | 'listening' | 'video' | 'reference' | 'practice'

export interface Resource {
  kind: ResourceKind
  title: string
  url: string
  /** The CEFR level this becomes usable at, as the pack states it. */
  level: string
  blurbRu: string
  blurbEn: string
  free: boolean
}

// ── the grammar tutor (ADR-027) ────────────────────────────────────────────

/** One turn of a tutor thread. An assistant turn names the model that wrote it. */
export interface TutorMessage {
  id: number
  role: 'user' | 'assistant'
  content: string
  lang?: string
  model?: string
  promptVersion?: number
  /**
   * The lesson an assistant turn was grounded in — pinned or inferred (ADR-029)
   * — with its name in the answer language; absent when the answer came from
   * the reference alone. Printed under the answer: a choice the learner can
   * see is a choice they can correct by pinning.
   */
  topic?: string
  topicTitle?: string
  /** Backticked Hungarian words the pack's dictionary does not carry. Never
   *  null from the server since ADR-029's fix; the panel still guards it. */
  unattested: string[]
  /** The stream was cut off — a timeout, a stop, a lost connection or the
   *  answer budget; the text is partial and the panel says so. */
  interrupted: boolean
  /** Why, when interrupted: budget, timeout, stop or connection; absent on
   *  older rows, which the panel labels without a cause. */
  cutReason?: 'budget' | 'timeout' | 'stop' | 'connection' | 'echo'
  /** The review card a question was about, its phase and the prepared
   *  template it came from (ADR-034); absent on an ordinary question. */
  cardId?: number
  cardPhase?: 'question' | 'answer'
  template?: string
  /**
   * The context meter (ADR-031): what the server reported spending on this
   * answer, in its own tokens, and the window the model ran with. Absent when
   * not reported — the meter says so instead of printing 0.
   */
  promptTokens?: number
  answerTokens?: number
  contextTokens?: number
  createdAt: string
}

export interface TutorSource {
  source: string
  url?: string
  license: string
}

/**
 * A review card a question is about (ADR-034). The server composes a prepared
 * question from it — the client never assembles the facts — and refuses a
 * template not offered on that card in that phase.
 */
export interface TutorCardRef {
  cardId: number
  phase: 'question' | 'answer'
  template?: string
  typed?: string
}

/** One prepared question the review card offers; `id` is the server's template id. */
export interface TutorChip {
  id: string
  label: string
}

/** One conversation's header. `topic` is its pin, blank for a free chat;
 *  `kind` is 'review' for a day's review chat, titled by the client from its date. */
export interface TutorThreadHeader {
  id: number
  /** The first question — or, on a review chat, the WORD it is about. */
  title: string
  topic?: string
  kind?: 'review'
  /** The note a review chat is about: one chat per word per day (ADR-034). */
  noteId?: number
  titleRu?: string
  titleEn?: string
  createdAt: string
  updatedAt: string
}

export interface TutorThread {
  available: boolean
  /** An i18n key saying why the tutor is unavailable. */
  reason?: string
  model?: string
  lang: string
  /** Null before the first question — a new chat, or a lesson panel opened
   *  before anything was asked there; `topic` is then the pin to create it with. */
  thread: TutorThreadHeader | null
  topic?: string
  messages: TutorMessage[]
  sources: TutorSource[]
}

/** One server-sent event of a streamed answer. */
export interface TutorEvent {
  /** On `asked`: the stored question's id, and its text when the server composed it. */
  id?: number
  question?: string
  text?: string
  message?: TutorMessage
  error?: string
}

/**
 * One row of the learner's chat list, most recently active first (ADR-029).
 * `topic` is the pin with its names; `lastTopic` is the lesson the latest
 * answer was grounded in, for a free chat's subtitle.
 */
/**
 * Which tab a chat belongs to.
 *
 * `chat` is the ordinary kind — free or pinned to a grammar topic. Its stored
 * value is the empty string, which a query string cannot carry, so the wire
 * name is `chat`. `review` is one per word per local day from the review card;
 * `word` is the same idea from the word inspector.
 */
export type TutorKind = 'chat' | 'review' | 'word'

/**
 * A dictionary entry written by the language model, for a word the pack does
 * not have. Mirrors `api.AIEntryResponse`. ADR-042.
 *
 * Everything here is DISPLAYED and labelled with `model`. It reaches
 * `user_gloss` only on an explicit save (as `machine`), and a card's back only
 * after «Принять как своё» re-saves it as `manual` — ADR-014/ADR-035.
 * `examples` are illustration, never material: they are not attested and never
 * become notes.
 */
export interface AIEntry {
  form: string
  model: string
  /** The model's own admission. An honest dead end, passed through. */
  unknown?: boolean
  lemma?: string
  pos?: string
  meanings?: string[]
  morphemes?: string[]
  examples?: { hu: string; tr: string }[]
  /** The model's text when it did not come back in the expected shape. */
  raw?: string
  /** Hungarian in this entry that the pack's dictionary has never seen. */
  unattested: string[]
}

/** Whether an AI entry can be offered at all, and by which model. */
export interface AIEntryStatus {
  available: boolean
  reason?: string
  model?: string
}

/** The chat list plus per-kind counts, for the tab strip. */
export interface TutorListResponse {
  threads: TutorSummary[]
  counts: Partial<Record<TutorKind, number>>
}

export interface TutorSummary {
  id: number
  /** The first question — or, on a review chat, the WORD it is about. */
  title: string
  topic?: string
  kind?: 'review'
  noteId?: number
  titleRu?: string
  titleEn?: string
  lastTopic?: string
  lastTopicRu?: string
  lastTopicEn?: string
  messages: number
  lastRole: 'user' | 'assistant' | ''
  lastContent: string
  updatedAt: string
}

/**
 * What a profile import is about to replace, counted in the learner's own
 * units rather than in table names. Mirrors `api.ProfileSizeResponse`.
 */
export interface ProfileSize {
  notes: number
  cards: number
  reviews: number
  texts: number
  words: number
  threads: number
  days: number
}

/**
 * What an import actually did. Mirrors `api.ImportReport`.
 *
 * `replayOk` is the integrity check no interchange format could make: every
 * card's review log was replayed through the scheduler and produced the state
 * that arrived with it. `replayWarning` is the one expected way for that to
 * come out unequal — a profile that crossed between CPU architectures, where
 * FSRS's interval fuzz redraws. `skippedColumns` names anything the file
 * carried that this schema has no home for; it is shown, never swallowed.
 */
export interface ImportReport {
  tables: Record<string, number>
  totalRows: number
  mediaFiles: number
  skippedColumns?: string[]
  replayOk: boolean
  replayChecked: number
  replayWarning?: string
  secretsOmitted: boolean
}

// ── Admin: pack install, restart, components (ADR-049) ────────────────────

export type PackInstallState = 'idle' | 'downloading' | 'verifying' | 'ready' | 'failed'

/** GET /api/admin/pack/install. `errorKey` is an i18n key (errors.*), '' when none. */
export interface PackInstallStatus {
  state: PackInstallState
  bytes: number
  total: number
  errorKey: string
  restartRequired: boolean
  /** The binary carries a pinned checksum, so the install needs no address from the admin. */
  published: boolean
  /** Size of the pack asset in bytes; 0 = unknown. */
  sizeBytes: number
  url: string
}

/** POST /api/admin/restart. false = nothing restarts it; tell the admin to start it again. */
export interface RestartResponse {
  supervised: boolean
}

export type ComponentId = 'pack' | 'mt' | 'tts' | 'asr' | 'llm'

/** GET /api/admin/components. `state` is a word the screen maps to copy. */
export interface ComponentStatus {
  id: ComponentId
  configured: boolean
  reachable: boolean
  /** pack: starter | full | installing | restartRequired | missing. Others: off | device | unreachable | ready, or mt's own loading | ready | failed. */
  state: string
  /** 0..1; 0 when nothing is in flight. */
  progress: number
  model: string
  ramHint: string
  composeProfile: string
  /** false = this repository does not ship the sidecar yet. */
  built: boolean
  /** mt only; absent from an older server — the screen then offers Docker and own-server tabs only. */
  managed?: ManagedStatus
}

export type ManagedState =
  | 'absent'
  | 'installing'
  | 'installed'
  | 'starting'
  | 'running'
  | 'failed'
  | 'stopped'

/** The service the app downloads and runs itself (ADR-050). Every field but `available`/`state` may be missing. */
export interface ManagedStatus {
  available: boolean
  /** i18n key (errors.componentNoPython | errors.componentReadOnly) when `available` is false. */
  unavailableKey?: string
  pythonVersion?: string
  state: ManagedState
  /** i18n key: component.step.venv | pip | model | start. */
  step?: string
  /** 0..1 over the whole install; never goes back within a job. */
  progress?: number
  /** 127.0.0.1 only. */
  port?: number
  errorKey?: string
  /** A short tail of the failing command; no secrets. */
  errorDetail?: string
  /** The last ≤40 lines of the install/run log. */
  log?: string[]
  /** Human estimates from the server's constants, not measurements. */
  diskNeed?: string
  ramNeed?: string
}

/** POST /api/library/sets/:code/import. `created` is false when the learner already had it. */
export interface ImportSetResult {
  textId: number
  created: boolean
}

/** GET /api/stats/deck: WORDS by stage (the Words list's rule), never a due count. */
export interface DeckStages {
  total: number
  stages: { learning: number; relearning: number; young: number; mature: number; known: number }
  /** In the deck, no reviewed card yet; not also counted in `stages.learning`. */
  new: number
  /** Ten decile counts (0–9, 10–19 … 90–100) of words with a reviewed card (ADR-070). */
  progress: number[]
}
