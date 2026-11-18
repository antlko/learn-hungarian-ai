/**
 * Local Piper speech through the server, with the device voice as the floor.
 *
 * The server renders a sentence to a WAV once and caches it by content; the
 * browser only asks and plays (ADR-079). Nothing here ever blocks the reader on
 * the engine: when `/api/speech/voices` says `available: false`, or a render
 * comes back 503/504, the caller speaks the same text with the device voice —
 * silently, because "the engine is off" is a normal state and not an error
 * (ADR-012). A 400 is a bug in this client and is reported.
 */
import { ApiError } from '@/api/http'
import { api } from '@/api'
import type { TtsVoiceId } from '@/types'

/** Whether the reader offers the «Аудирование» mode toggle (components/reader/ListenMode.vue). */
export const LISTEN_MODE_READY = true

export const VOICE_KEY = 'tanulom.tts.voice'
export const RATE_KEY = 'tanulom.tts.rate'

/** The sample every persona card plays. Fixed Hungarian, so a voice is judged on the same words. */
export const SAMPLE_TEXT = 'Szia! Ma együtt olvasunk.'

/** The personas, in card order. Initials are chrome, not an avatar. */
export const PERSONAS: { id: TtsVoiceId; initial: string; gender: 'female' | 'male' }[] = [
  { id: 'anna', initial: 'A', gender: 'female' },
  { id: 'berta', initial: 'B', gender: 'female' },
  { id: 'imre', initial: 'I', gender: 'male' },
]

export const isPersona = (v: unknown): v is TtsVoiceId => PERSONAS.some((p) => p.id === v)

/** localStorage can throw (private mode, quota, disabled): a preference is never worth a crash. */
export function loadStoredVoice(): TtsVoiceId | '' {
  try {
    const v = localStorage.getItem(VOICE_KEY)
    return isPersona(v) ? v : ''
  } catch {
    return ''
  }
}
export function storeVoice(id: TtsVoiceId | ''): void {
  try {
    if (id) localStorage.setItem(VOICE_KEY, id)
    else localStorage.removeItem(VOICE_KEY)
  } catch {
    // The choice holds for this session only.
  }
}

export const RATES = [0.5, 0.75, 1, 1.25, 1.5]

export function loadStoredRate(): number {
  try {
    const raw = localStorage.getItem(RATE_KEY)
    const n = Number(raw)
    return raw !== null && n >= 0.5 && n <= 1.5 ? n : 1
  } catch {
    return 1
  }
}
export function storeRate(n: number): void {
  try {
    localStorage.setItem(RATE_KEY, String(n))
  } catch {
    // Session only.
  }
}

/** The server's cap on one request, in UTF-8 bytes. */
const MAX_BYTES = 1000

const byteLen = (s: string) => new TextEncoder().encode(s).length

/**
 * One sentence as pieces the server accepts. A sentence under the cap is one
 * piece; a longer one is cut at the last comma/semicolon/space that fits, so a
 * pause lands where a person would take one.
 */
export function chunkForTts(text: string, limit = MAX_BYTES - 100): string[] {
  const out: string[] = []
  let rest = text.trim()
  while (rest && byteLen(rest) > limit) {
    let cut = rest.length
    while (cut > 0 && byteLen(rest.slice(0, cut)) > limit) cut = Math.floor(cut * 0.9)
    const head = rest.slice(0, cut)
    const at = Math.max(head.lastIndexOf(', '), head.lastIndexOf('; '), head.lastIndexOf(' '))
    const end = at > 20 ? at + 1 : cut
    out.push(rest.slice(0, end).trim())
    rest = rest.slice(end).trim()
  }
  if (rest) out.push(rest)
  return out
}

/** True when the engine is off or not answering: fall back to the device voice, quietly. */
export function shouldFallBack(e: unknown): boolean {
  return e instanceof ApiError && (e.status === 503 || e.status === 504 || e.status === 0)
}

/**
 * A render cache with in-flight de-duplication and a ceiling on parallel
 * requests. The server caches too, but a round trip to learn that is the delay
 * this exists to hide, and re-requesting what is already here is waste.
 */
export class RenderCache {
  private done = new Map<string, { url: string; durationS: number }>()
  private flying = new Map<string, Promise<{ url: string; durationS: number }>>()
  private waiting: (() => void)[] = []
  private active = 0

  constructor(private readonly maxInFlight = 2) {}

  private key(voice: TtsVoiceId, text: string) {
    return `${voice}\u0000${text}`
  }

  has(voice: TtsVoiceId, text: string): boolean {
    return this.done.has(this.key(voice, text))
  }

  get(voice: TtsVoiceId, text: string) {
    const k = this.key(voice, text)
    const hit = this.done.get(k)
    if (hit) return Promise.resolve(hit)
    const fly = this.flying.get(k)
    if (fly) return fly
    const p = this.slot().then(() =>
      api
        .speechTts({ text, voice, style: 'sentence' })
        .then((r) => {
          this.done.set(k, r)
          return r
        })
        .finally(() => {
          this.flying.delete(k)
          this.release()
        }),
    )
    this.flying.set(k, p)
    return p
  }

  private slot(): Promise<void> {
    if (this.active < this.maxInFlight) {
      this.active++
      return Promise.resolve()
    }
    return new Promise((res) => this.waiting.push(res))
  }
  private release() {
    const next = this.waiting.shift()
    if (next) next()
    else this.active--
  }

  /** Forget one render (its file is gone from the server cache) so the next `get` asks again. */
  drop(voice: TtsVoiceId, text: string) {
    this.done.delete(this.key(voice, text))
  }

  clear() {
    this.done.clear()
  }
}
