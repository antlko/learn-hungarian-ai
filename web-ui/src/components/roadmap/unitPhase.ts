import type { CourseCounts, CourseTier, UnitState } from '@/types'

/**
 * What a unit's label says. Derived, never stored: «выучен» is a fact about the
 * learner's words (learned === total), not about the deck merely containing them.
 *
 *   learned   every word learned
 *   learning  the unit is in work: enrolled, or all its words are already in the deck
 *   next      the app's ADVICE on where to continue
 *   ahead     nothing started; nothing locked (ADR-013)
 */
export type UnitPhase = 'learned' | 'learning' | 'next' | 'ahead'

export const TIERS: CourseTier[] = ['basic', 'intermediate', 'advanced']

export const learnedOf = (u: UnitState) => u.learned ?? 0
export const settledOf = (u: UnitState) => u.settled ?? 0
export const tierOf = (u: UnitState): CourseTier => u.unit.tier ?? 'basic'
export const isSideUnit = (u: UnitState) => !!u.unit.track && u.unit.track !== 'core'

export function unitPhase(u: UnitState): UnitPhase {
  if (u.wordsTotal > 0 && learnedOf(u) >= u.wordsTotal) return 'learned'
  if (u.enrolled || u.status === 'done') return 'learning'
  if (u.status === 'current') return 'next'
  return 'ahead'
}

/** Sums over core units; the server's own totals win when it sends them. */
export function sumCounts(units: UnitState[]): CourseCounts {
  const out = { words: 0, inDeck: 0, learned: 0, settled: 0 }
  for (const u of units) {
    out.words += u.wordsTotal
    out.inDeck += u.wordsInPlay
    out.learned += learnedOf(u)
    out.settled += settledOf(u)
  }
  return out
}

/** The soft gate's default (ADR-058); the server's `unlock.share` wins when sent. */
export const UNLOCK_SHARE = 0.7

/** Settled words over total; 0 for an empty unit (never NaN). */
export const settledShare = (u: UnitState) => (u.wordsTotal > 0 ? settledOf(u) / u.wordsTotal : 0)

/** Is the unit drawn as started: opened, or some of its words already in the deck. */
export const isStarted = (u: UnitState) => u.enrolled || u.wordsInPlay > 0

export interface GateEntry {
  share: number
  /** share >= the unlock share. */
  passed: boolean
  /** Presentation only: after the current unit and never opened. Tapping still works. */
  closed: boolean
  /** The core unit before this one, when there is one. */
  prev: UnitState | null
}
export interface Gate {
  /** First core unit not passed; '' when every core unit is. */
  current: string
  byCode: Map<string, GateEntry>
}

/**
 * The soft gate, computed from the plan alone (ADR-058, ADR-013): core units in
 * `ord` order; `current` is the first not passed; units after it are «closed»
 * unless the learner already opened them. Themes are never gated. Nothing here
 * refuses anything — it only decides how a node is DRAWN and what the unit page
 * says.
 */
export function computeGate(units: UnitState[], unlockShare: number = UNLOCK_SHARE): Gate {
  const core = units.filter((u) => !isSideUnit(u)).sort((a, b) => a.unit.ord - b.unit.ord)
  const byCode = new Map<string, GateEntry>()
  let current = ''
  core.forEach((u, i) => {
    const share = settledShare(u)
    const passed = share >= unlockShare
    if (!passed && !current) current = u.unit.code
    byCode.set(u.unit.code, {
      share,
      passed,
      closed: !!current && u.unit.code !== current && !u.enrolled && !passed,
      prev: i > 0 ? core[i - 1]! : null,
    })
  })
  for (const u of units) {
    if (isSideUnit(u)) byCode.set(u.unit.code, { share: settledShare(u), passed: settledShare(u) >= unlockShare, closed: false, prev: null })
  }
  return { current, byCode }
}
