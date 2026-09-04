import { http, ApiError } from './http'
import type { TutorCardRef, TutorEvent, TutorMessage, TutorSummary, TutorThread } from '@/types'

/**
 * The grammar tutor's calls (ADR-027, ADR-029). A thread is a conversation:
 * listed, opened, asked, deleted; pinned to a topic or free, in which case
 * the server finds the lesson per question and names it on the answer.
 *
 * `ask` is the one endpoint in the app that streams: the answer arrives as
 * server-sent events over a POST, because a local model on a CPU writes a
 * sentence at a time and an answer that appears all at once after twenty
 * seconds reads as a hang. `fetch` + a reader rather than `EventSource`, which
 * can only GET. Each `delta` event is handed to `onDelta`; the promise resolves
 * with the stored message from the `done` event, or rejects with an ApiError
 * carrying the i18n key from an `error` event. Any text that arrived comes
 * back as `done` with the stored message, marked `interrupted` when the answer
 * was cut — by the budget, the timeout, or `stop` (ADR-031); `error` means
 * nothing arrived. `replace` edits a question of the learner's own: it and
 * everything after it are removed before this one is asked.
 */
export const tutorApi = {
  /** The learner's chats, most recently active first. */
  list: () => http.get<TutorSummary[]>('/api/tutor'),

  /** Opens a chat — pinned when `topic` is non-blank. `kind: 'review'` asks
   *  for the chat about `note` instead — one per word per day (ADR-034):
   *  found when it exists, else created and named by the word. */
  create: (topic: string, lang: string, kind?: 'review', note?: number) =>
    http.post<TutorThread>(`/api/tutor?lang=${lang}`, { topic, kind, note }),

  /** The day's chat about one word, or `thread: null` — never created by
   *  looking. The panel opens on it without minting an empty conversation for
   *  a learner who asks nothing. */
  reviewNote: (lang: string, note: number) =>
    http.get<TutorThread>(`/api/tutor/review?lang=${lang}&note=${note}`),

  thread: (id: number, lang: string) => http.get<TutorThread>(`/api/tutor/${id}?lang=${lang}`),

  /** A chat not yet started: availability and sources, no thread. */
  fresh: (lang: string) => http.get<TutorThread>(`/api/tutor/new?lang=${lang}`),

  /** The latest chat pinned to a lesson, or none yet with the pin to use. */
  forTopic: (code: string, lang: string) =>
    http.get<TutorThread>(`/api/tutor/topic/${encodeURIComponent(code)}?lang=${lang}`),

  remove: (id: number) => http.del<void>(`/api/tutor/${id}`),

  /** Ends the answer in flight where it is; the stream then delivers the
   *  partial as `done`, marked interrupted. Idempotent. */
  stop: (id: number) => http.post<void>(`/api/tutor/${id}/stop`),

  async ask(
    id: number,
    question: string,
    lang: string,
    onDelta: (text: string) => void,
    replace?: number,
    onAsked?: (questionId: number, question?: string) => void,
    card?: TutorCardRef,
  ): Promise<TutorMessage> {
    let res: Response
    try {
      res = await fetch(`/api/tutor/${id}`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
        body: JSON.stringify({ question, lang, replace: replace || undefined, card }),
      })
    } catch {
      throw new ApiError(0, 'network-unreachable')
    }
    if (!res.ok) {
      // A refusal is plain JSON in the API's one error shape.
      let message = `request-failed-${res.status}`
      try {
        const payload = (await res.json()) as { error?: unknown }
        if (payload?.error) message = String(payload.error)
      } catch {
        // keep the status message
      }
      throw new ApiError(res.status, message)
    }
    if (!res.body) throw new ApiError(res.status, 'unexpected-response')

    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let partial: TutorMessage | undefined

    const handle = (event: string, data: string): TutorMessage | undefined => {
      let ev: TutorEvent
      try {
        ev = JSON.parse(data) as TutorEvent
      } catch {
        return undefined
      }
      if (event === 'asked' && ev.id) onAsked?.(ev.id, ev.question)
      if (event === 'delta' && ev.text) onDelta(ev.text)
      if (event === 'done' && ev.message) return ev.message
      if (event === 'error') {
        partial = ev.message
        throw new TutorError(ev.error || 'errors.tutorFailed', ev.message)
      }
      return undefined
    }

    // Events are blank-line separated; a chunk may split one in half, so the
    // buffer keeps the tail.
    for (;;) {
      const { value, done } = await reader.read()
      buffer += decoder.decode(value ?? new Uint8Array(), { stream: !done })
      let cut: number
      while ((cut = buffer.indexOf('\n\n')) >= 0) {
        const block = buffer.slice(0, cut)
        buffer = buffer.slice(cut + 2)
        let event = ''
        let data = ''
        for (const line of block.split('\n')) {
          if (line.startsWith('event: ')) event = line.slice(7)
          else if (line.startsWith('data: ')) data += line.slice(6)
        }
        if (!event) continue
        const final = handle(event, data)
        if (final) return final
      }
      if (done) break
    }
    if (partial) return partial
    throw new ApiError(0, 'errors.tutorFailed')
  },
}

/** A stream that ended in an error event; `partial` is what was said first. */
export class TutorError extends ApiError {
  constructor(
    message: string,
    public readonly partial?: TutorMessage,
  ) {
    super(502, message)
    this.name = 'TutorError'
  }
}
