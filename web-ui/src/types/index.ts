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
