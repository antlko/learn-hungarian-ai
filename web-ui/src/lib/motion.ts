/**
 * The motion system's durations, in TypeScript.
 *
 * These mirror the `--animate-*` tokens in `assets/tailwind.css` one for one.
 * The CSS is the implementation; this file exists because some things have to
 * be timed in script — when to unmount a leaving element, how long to hold a
 * transient overlay — and hard-coding 320 in a component is how a design system
 * drifts. If you change a duration, change it in both places.
 *
 * Nothing here starts an animation. Motion is CSS; this is only the clock.
 */
export const MOTION = {
  answerCorrect: 180,
  answerWrong: 300,
  cardIn: 160,
  cardOut: 120,
  tickUp: 240,
  phaseChange: 260,
  milestone: 320,
} as const

/**
 * The OS setting, read live. The global rule in `assets/tailwind.css` already
 * disables every animation; this is for the script-side consequence — skipping
 * an exit delay rather than waiting out an animation that is not playing.
 */
export function prefersReducedMotion(): boolean {
  if (typeof window === 'undefined' || !window.matchMedia) return false
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/**
 * `ms` normally, 0 when the learner asked for less motion. Use it for timers
 * that wait for an animation to finish, never for timers that control how long
 * information stays readable — a reduced-motion learner still needs the time to
 * read it.
 */
export function motionDuration(ms: number): number {
  return prefersReducedMotion() ? 0 : ms
}
