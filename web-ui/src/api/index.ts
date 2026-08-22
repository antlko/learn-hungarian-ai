import { http } from './http'
import type {
  AnalyzeResponse,
  AnswerRequest,
  AnswerResponse,
  Bootstrap,
  Capability,
  FeatureMastery,
  Forecast,
  GovernorVerdict,
  Me,
  MorphFeature,
  QueueResponse,
  Settings,
  UpdateSettingsRequest,
  UpdateSettingsResponse,
} from '@/types'

/** Typed API surface. One function per endpoint in docs/API.md. */
export const api = {
  bootstrap: () => http.get<Bootstrap>('/api/bootstrap'),

  setup: (body: { email: string; password: string; uiLocale: string; glossLang: string }) =>
    http.post<Me>('/api/setup', body),

  login: (body: { email: string; password: string }) => http.post<Me>('/api/login', body),

  logout: () => http.post<void>('/api/logout'),

  me: () => http.get<Me>('/api/me'),

  settings: () => http.get<Settings>('/api/settings'),

  updateSettings: (body: UpdateSettingsRequest) =>
    http.put<UpdateSettingsResponse>('/api/settings', body),

  capabilities: () => http.get<Capability[]>('/api/capabilities'),

  // ── language ──────────────────────────────────────────────────────────
  analyze: (text: string) => http.post<AnalyzeResponse>('/api/analyze', { text }),

  features: () => http.get<MorphFeature[]>('/api/features'),

  // ── review ────────────────────────────────────────────────────────────
  queue: () => http.get<QueueResponse>('/api/review/queue'),

  answer: (body: AnswerRequest) => http.post<AnswerResponse>('/api/review/answer', body),

  forecast: (newPerDay?: number) =>
    http.get<{ forecast: Forecast; verdict?: GovernorVerdict }>(
      newPerDay === undefined
        ? '/api/review/forecast'
        : `/api/review/forecast?newPerDay=${newPerDay}`,
    ),

  createNote: (body: {
    hu: string
    targetForm?: string
    glossRu?: string
    glossEn?: string
    lemmaId?: number
  }) =>
    http.post<{ noteId: number; directions: string[]; targetFeats: string[] }>('/api/notes', body),

  mastery: () => http.get<{ features: FeatureMastery[] }>('/api/stats/mastery'),
}

export { ApiError } from './http'
