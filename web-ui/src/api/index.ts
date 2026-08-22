import { http } from './http'
import type {
  Bootstrap,
  Capability,
  Me,
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
}

export { ApiError } from './http'
