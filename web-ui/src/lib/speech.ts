/**
 * Hungarian speech from the device, through the Web Speech API.
 *
 * This exists because the app made no sound at all. Every audio feature — the
 * 🔊 in the reader, the `listen` and `spell` review directions, hearing a form
 * you just got wrong — was gated behind a TTS sidecar, and `services/tts/` in
 * this repository is a README. A fresh install therefore had four services
 * reading "off" and no way to hear a single Hungarian word.
 *
 * `speechSynthesis` is on every browser we target and every desktop OS ships at
 * least one Hungarian voice, so this is the default source of speech. It is
 * genuinely optional and genuinely checkable, which is the important part: the
 * app asks the device what it has and reports the answer, rather than assuming.
 *
 * Two browser behaviours this has to survive:
 *
 *   1. `getVoices()` is empty on first call in Chrome and fills in later, on a
 *      `voiceschanged` event that may fire more than once. Anything that reads
 *      the list once at import time concludes there are no voices, forever.
 *   2. Speech needs a user gesture on some engines. Nothing here autoplays on
 *      page load; every call is downstream of a click or a keypress, and a
 *      rejected utterance is reported rather than swallowed.
 */

/** Whether the browser has the API at all. */
export const speechSupported = (): boolean =>
  typeof window !== 'undefined' && 'speechSynthesis' in window

let cachedVoices: SpeechSynthesisVoice[] = []
const listeners = new Set<() => void>()

function refresh() {
  if (!speechSupported()) return
  cachedVoices = window.speechSynthesis.getVoices()
  for (const fn of listeners) fn()
}

if (speechSupported()) {
  refresh()
  // Fires when the list arrives, and again when the OS voice set changes.
  window.speechSynthesis.addEventListener('voiceschanged', refresh)
}

/** Subscribe to voice-list changes. Returns the unsubscribe function. */
export function onVoicesChanged(fn: () => void): () => void {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

/**
 * The Hungarian voices this device has.
 *
 * Matched on the BCP-47 prefix rather than the exact tag: engines report
 * `hu-HU`, `hu_HU` and bare `hu` depending on the platform.
 */
export function hungarianVoices(): SpeechSynthesisVoice[] {
  return cachedVoices.filter((v) => /^hu\b/i.test(v.lang.replace('_', '-')))
}

/**
 * Voices whose name marks them as a higher-quality build.
 *
 * Every desktop OS ships a small "compact" voice per language and offers a much
 * better one as a download — macOS calls them Enhanced and Premium, Windows
 * calls them Natural. They are the same voice by name, so `Tünde` and
 * `Tünde (Enhanced)` sit side by side in the list and the compact one usually
 * comes first. Picking by list order therefore picks the WORST available voice,
 * which is what "the speech sounds robotic" turned out to be.
 */
const QUALITY = /\((enhanced|premium|natural|neural)\)|\b(enhanced|premium|natural|neural)\b/i

/** True when this voice is one of the downloadable high-quality builds. */
export const isHighQuality = (v: SpeechSynthesisVoice): boolean => QUALITY.test(v.name)

/**
 * The voice to speak with, or null when the device has no Hungarian one.
 *
 * Order of preference: the name the learner chose, then the best-sounding voice
 * installed, then anything. Quality beats locality — a network voice that
 * sounds like a person is worth more here than an on-device one that does not,
 * and the offline case is already covered by there always being a local voice
 * to fall back to.
 */
export function pickVoice(preferredName?: string): SpeechSynthesisVoice | null {
  const voices = hungarianVoices()
  if (!voices.length) return null
  if (preferredName) {
    const named = voices.find((v) => v.name === preferredName)
    if (named) return named
  }
  return voices.find(isHighQuality) ?? voices.find((v) => v.localService) ?? voices[0]
}

/**
 * What is actually handed to the engine.
 *
 * A single capital letter is ANNOUNCED rather than read: the voice says
 * «nagy A» — "capital A" — which is what the reader's most common one-letter
 * word, the definite article `A` at the head of a sentence, came out as. The
 * lower-case letter is the same sound in Hungarian and is read as the word,
 * so the case is dropped for a one-letter utterance and for nothing else.
 */
export function spokenForm(text: string): string {
  return /^\p{Lu}[.,!?;:…]?$/u.test(text) ? text.toLocaleLowerCase('hu') : text
}

export interface SpeakOptions {
  /** 0.5–1.5. Slower is for hearing a suffix, not a different reading speed. */
  rate?: number
  voiceName?: string
  onEnd?: () => void
  onError?: () => void
}

/**
 * The rate a compact voice is most intelligible at.
 *
 * A concatenative voice degrades fastest at the joins between units, and the
 * joins are what a learner needs: Hungarian vowel length is phonemic, and `hat`
 * against `hát` is exactly the distinction a rushed compact voice smears. 0.9
 * is enough to hear it and not slow enough to sound like a warning message.
 * Applied only where the device has nothing better installed — a good voice at
 * 0.9 sounds artificially careful.
 */
export const COMPACT_RATE = 0.9

/**
 * Speak Hungarian. Returns false when the device cannot.
 *
 * Any utterance in flight is cancelled first: two overlapping voices reading
 * two different words is worse than no audio, and the reader fires this on
 * every arrow key.
 */
export function speak(text: string, opts: SpeakOptions = {}): boolean {
  const trimmed = text.trim()
  if (!trimmed || !speechSupported()) return false
  const voice = pickVoice(opts.voiceName)
  if (!voice) return false

  window.speechSynthesis.cancel()
  const utter = new SpeechSynthesisUtterance(spokenForm(trimmed))
  utter.voice = voice
  utter.lang = voice.lang
  utter.rate = Math.min(1.5, Math.max(0.5, opts.rate ?? 1))
  if (opts.onEnd) utter.addEventListener('end', opts.onEnd)
  if (opts.onError) utter.addEventListener('error', opts.onError)
  window.speechSynthesis.speak(utter)
  return true
}

export function cancelSpeech(): void {
  if (speechSupported()) window.speechSynthesis.cancel()
}
