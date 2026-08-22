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
  /** Presence only — a secret's value never reaches the client. */
  secrets: Record<string, boolean>
}

export type ServiceProvider = 'off' | 'local' | 'openai'

export interface Service {
  provider: ServiceProvider
  endpoint: string
  model?: string
  voices?: string[]
}

export interface UpdateSettingsRequest {
  locale?: Settings['locale']
  review?: Settings['review']
  mining?: Settings['mining']
  drill?: Settings['drill']
  hvpt?: Settings['hvpt']
  curriculum?: Settings['curriculum']
  services?: Record<string, Service>
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

export type CapabilityName = 'tts' | 'asr' | 'nlp' | 'llm'

export interface Capability {
  name: CapabilityName
  provider: ServiceProvider
  configured: boolean
  reachable: boolean
  latencyMs?: number
  detail?: string
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

export interface QueueItem {
  cardId: number
  noteId: number
  direction: DirectionName
  isNew: boolean
  due: string
  hu: string
  targetForm: string
  targetStart: number
  targetEnd: number
  targetFeats: string[]
  glossRu: string
  glossEn: string
  lemmaId?: number
}

export interface QueueCounts {
  due: number
  new: number
  learning: number
  total: number
  /** New cards held back by the daily limit — shown so the cap is visible. */
  newCapped: number
}

export interface QueueResponse {
  items: QueueItem[]
  counts: QueueCounts
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
  words: CourseWord[]
}

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
  /** The unit's words with no card yet — named, so the UI never infers them. */
  notInPlay: string[]
}

export interface CoursePlan {
  units: UnitState[]
  /** The unit to work on now. Empty once the course has handed over. */
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

export interface CourseResponse {
  plan: CoursePlan
  enrolled: Record<string, CourseProgress>
}

/** What taking a unit into work actually produced — including what it did not. */
export interface EnrolResult {
  unitCode: string
  notesMade: number
  duplicate: number
  /** Words the pack has no attested sentence for, so no card was made. */
  skipped: string[]
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
