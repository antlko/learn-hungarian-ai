import { http, ApiError } from './http'
import type {
  WritingCheck,
  WritingCreateRequest,
  WritingExternalRequest,
  WritingListItem,
  WritingPiece,
  WritingProgress,
  WritingPromptResponse,
  WritingTopic,
} from '@/types'

/**
 * «Письмо» (ADR-085). Nothing here makes a card. `checkLocal` streams like the
 * tutor does (server-sent events over a POST, read with fetch + a reader): the
 * promise resolves with the stored check from `done` and rejects with an
 * ApiError carrying the i18n key from an `error` event or a refusal.
 */
export const writingApi = {
  topics: () => http.get<{ topics: WritingTopic[] }>('/api/writing/topics').then((r) => r.topics),
  list: () => http.get<{ pieces: WritingListItem[] }>('/api/writing').then((r) => r.pieces),
  create: (body: WritingCreateRequest) => http.post<WritingPiece>('/api/writing', body),
  get: (id: number) => http.get<WritingPiece>(`/api/writing/${id}`),
  save: (id: number, title: string, body: string) =>
    http.put<WritingPiece>(`/api/writing/${id}`, { title, body }),
  remove: (id: number) => http.del<void>(`/api/writing/${id}`),
  prompt: (id: number, lang: string) =>
    http.post<WritingPromptResponse>(`/api/writing/${id}/prompt`, { lang }),
  external: (id: number, body: WritingExternalRequest) =>
    http.post<WritingCheck>(`/api/writing/${id}/checks/external`, body),
  check: (id: number, checkId: number) => http.get<WritingCheck>(`/api/writing/${id}/checks/${checkId}`),
  removeCheck: (id: number, checkId: number) =>
    http.del<void>(`/api/writing/${id}/checks/${checkId}`),
  /** Ends the running local check where it is; what is complete is kept as interrupted. Idempotent. */
  stop: () => http.post<void>('/api/writing/checks/stop'),

  async checkLocal(
    id: number,
    lang: string,
    onProgress: (p: WritingProgress) => void,
  ): Promise<WritingCheck> {
    let res: Response
    try {
      res = await fetch(`/api/writing/${id}/checks/local`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
        body: JSON.stringify({ lang }),
      })
    } catch {
      throw new ApiError(0, 'network-unreachable')
    }
    if (!res.ok) {
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
    for (;;) {
      const { value, done } = await reader.read()
      buffer += decoder.decode(value ?? new Uint8Array(), { stream: !done })
      let cut: number
      // Events are blank-line separated; a chunk may split one, so the buffer keeps the tail.
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
        let ev: { chars?: number; errors?: number; check?: WritingCheck; error?: string }
        try {
          ev = JSON.parse(data)
        } catch {
          continue
        }
        if (event === 'progress') onProgress({ chars: ev.chars ?? 0, errors: ev.errors ?? 0 })
        else if (event === 'done' && ev.check) return ev.check
        else if (event === 'error') throw new ApiError(502, ev.error || 'errors.writingCheckFailed')
      }
      if (done) break
    }
    throw new ApiError(0, 'errors.writingCheckFailed')
  },
}
