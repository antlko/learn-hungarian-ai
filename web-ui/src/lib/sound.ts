/**
 * Feedback sounds, synthesised.
 *
 * Every sound is built from oscillators and gain envelopes at runtime. No audio
 * files ship: nothing to licence, nothing to download, nothing to embed in the
 * single static binary, and no CDN for a self-hosted install to depend on.
 *
 * [R1] The testing effect grows when feedback follows the retrieval attempt —
 * see .claude/research/01-retrieval-practice.md. That is what these are: a
 * sound is feedback, not a reward. There is no point economy behind it and
 * nothing is unlocked by hearing one — [R16] .claude/research/16-debunked.md §3.
 *
 * Design constraints, all of them consequences of repetition. A learner doing
 * 200 cards a day hears `correct` 200 times:
 *  · Under 250ms, so the sound is over before the next card is read.
 *  · Sine and triangle only. Sawtooth and square carry odd harmonics that turn
 *    abrasive by the fiftieth repetition.
 *  · Quiet — peak gain 0.07 of full scale — and `wrong` is quieter than
 *    `correct`, not louder. The learner is wrong by design a fifth of the time;
 *    a buzzer on a fifth of answers teaches avoidance, not Hungarian.
 *  · Notes come from one D-major-ish set (D5/A5/D6, plus G4→E♭4 for `wrong`) so
 *    that two sounds landing close together are consonant rather than a clash.
 *
 * Reduced motion is deliberately NOT consulted: sound is a separate channel with
 * its own preference. The toggle lives in the UI store and Settings.
 */

const STORAGE_KEY = 'tanulom.sound'

export type SoundName = 'correct' | 'wrong' | 'milestone' | 'sessionComplete'

interface Voice {
  /** Hertz. */
  freq: number
  type: OscillatorType
  /** Seconds after the trigger. */
  at: number
  /** Seconds; includes the decay tail. */
  dur: number
  /** Peak gain, 0–1, before the master trim. */
  gain: number
  /** Optional glide target in hertz, reached at the end of `dur`. */
  glideTo?: number
}

/**
 * Recipes. `dur` is the whole audible life of the voice, so the longest
 * `at + dur` in a recipe is the length of the sound.
 */
const RECIPES: Record<SoundName, Voice[]> = {
  // D5 with a soft octave on top: one short, bright, unremarkable blip.
  correct: [
    { freq: 587.33, type: 'sine', at: 0, dur: 0.11, gain: 0.05 },
    { freq: 1174.66, type: 'sine', at: 0.005, dur: 0.07, gain: 0.014 },
  ],
  // G4 sagging to E♭4 — a minor sixth down, quieter than `correct`. It reads as
  // "no" without reading as a penalty.
  wrong: [
    { freq: 392.0, type: 'triangle', at: 0, dur: 0.16, gain: 0.038, glideTo: 311.13 },
    { freq: 196.0, type: 'sine', at: 0, dur: 0.14, gain: 0.016, glideTo: 155.56 },
  ],
  // A rising D–A–D arpeggio. The only sound in the app allowed to feel like an
  // event, because the only thing that fires it is a real one.
  milestone: [
    { freq: 587.33, type: 'sine', at: 0, dur: 0.1, gain: 0.05 },
    { freq: 880.0, type: 'sine', at: 0.06, dur: 0.1, gain: 0.05 },
    { freq: 1174.66, type: 'sine', at: 0.12, dur: 0.12, gain: 0.06 },
    { freq: 293.66, type: 'triangle', at: 0.12, dur: 0.12, gain: 0.02 },
  ],
  // E5 falling to A4: a descending fifth is a full stop. The queue is empty; go
  // and do something else.
  sessionComplete: [
    { freq: 659.25, type: 'sine', at: 0, dur: 0.12, gain: 0.045 },
    { freq: 440.0, type: 'sine', at: 0.09, dur: 0.15, gain: 0.05 },
    { freq: 220.0, type: 'triangle', at: 0.09, dur: 0.15, gain: 0.018 },
  ],
}

/** Master trim, so every recipe can be balanced against the others in one place. */
const MASTER = 0.9

/** The floor for `exponentialRampToValueAtTime`, which rejects a target of 0. */
const SILENCE = 0.0001

type AudioContextCtor = typeof AudioContext

let ctx: AudioContext | null = null
let master: GainNode | null = null
let enabled = readEnabled()

function readEnabled(): boolean {
  try {
    // Default ON: a learner who wants silence turns it off once, and the
    // absence of feedback is worse for the majority than a sound they mute.
    return localStorage.getItem(STORAGE_KEY) !== 'off'
  } catch {
    return true
  }
}

function ensureContext(): AudioContext | null {
  if (!enabled || typeof window === 'undefined') return null
  if (!ctx) {
    const w = window as Window & { webkitAudioContext?: AudioContextCtor }
    const Ctor = window.AudioContext ?? w.webkitAudioContext
    if (!Ctor) return null
    ctx = new Ctor()
    master = ctx.createGain()
    master.gain.value = MASTER
    master.connect(ctx.destination)
  }
  // Browsers create the context suspended until a user gesture, and suspend it
  // again on some platforms when the tab is backgrounded. `resume()` is a
  // no-op when it is already running, and rejects harmlessly when there has
  // been no gesture yet — in which case the next real gesture will fix it.
  if (ctx.state === 'suspended') void ctx.resume()
  return ctx
}

const GESTURES = ['pointerdown', 'keydown', 'touchstart'] as const

/**
 * The context can only be created inside a user gesture, so we take the first
 * one the app sees. The listener stays attached while sound is disabled — the
 * learner may enable it later, and `setSoundEnabled` primes it directly then.
 */
function onFirstGesture() {
  if (!enabled) return
  if (ensureContext()) removeGestureHooks()
}

function removeGestureHooks() {
  for (const g of GESTURES) window.removeEventListener(g, onFirstGesture, true)
}

if (typeof window !== 'undefined') {
  for (const g of GESTURES) {
    window.addEventListener(g, onFirstGesture, { capture: true, passive: true })
  }
}

function voice(ac: AudioContext, out: GainNode, v: Voice, t0: number) {
  const osc = ac.createOscillator()
  const env = ac.createGain()
  osc.type = v.type
  osc.frequency.setValueAtTime(v.freq, t0 + v.at)
  if (v.glideTo !== undefined) {
    osc.frequency.exponentialRampToValueAtTime(v.glideTo, t0 + v.at + v.dur)
  }

  // 6ms attack: fast enough to feel simultaneous with the keystroke, slow
  // enough that the onset is not a click.
  const start = t0 + v.at
  env.gain.setValueAtTime(SILENCE, start)
  env.gain.exponentialRampToValueAtTime(v.gain, start + 0.006)
  env.gain.exponentialRampToValueAtTime(SILENCE, start + v.dur)

  osc.connect(env)
  env.connect(out)
  osc.start(start)
  osc.stop(start + v.dur + 0.02)
}

/**
 * Play a feedback sound. Cheap and safe to call from a hot path: it returns
 * immediately when sound is off or the platform has no WebAudio, and never
 * throws into the caller.
 */
export function play(name: SoundName): void {
  const ac = ensureContext()
  if (!ac || !master) return
  try {
    const t0 = ac.currentTime
    for (const v of RECIPES[name]) voice(ac, master, v, t0)
  } catch {
    // A failed sound must never break an answer. Nothing to report: the
    // learner can see the feedback on screen, which is the channel that counts.
  }
}

export function isSoundEnabled(): boolean {
  return enabled
}

/**
 * Persist the preference. Turning sound ON is itself a user gesture, so this is
 * also the moment to build the context — otherwise the first sound after
 * enabling would be swallowed.
 */
export function setSoundEnabled(next: boolean): void {
  enabled = next
  try {
    localStorage.setItem(STORAGE_KEY, next ? 'on' : 'off')
  } catch {
    // Private-mode storage refusals leave the flag in memory for this session.
  }
  if (next) ensureContext()
}
