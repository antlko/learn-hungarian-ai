/**
 * Wire types. These mirror backend/internal/api/dto.go exactly (camelCase).
 * When a DTO shape changes, change both files and docs/API.md in the same commit.
 */

export interface Bootstrap {
  needsSetup: boolean
  packPresent: boolean
  packVersion?: string
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
  curriculum: { focusUnlockListeningHours: number }
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

export type Certainty = 'certain' | 'uncertain' | 'derived' | 'unknown'

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
