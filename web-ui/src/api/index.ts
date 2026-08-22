import { http, ApiError } from './http'
import type {
  ChangePasswordRequest,
  CreateUserRequest,
  UpdateUserRequest,
  User,
  AnalyzeResponse,
  Breakdown,
  CheckpointVerdict,
  Overview,
  CoverageVerdict,
  HvptAnswer,
  HvptContrast,
  HvptSession,
  ListenState,
  UndoResult,
  CourseResponse,
  EnrolResult,
  Resource,
  CurriculumResponse,
  DrillAnswerRequest,
  DrillAnswerResponse,
  DrillNextResponse,
  PlacementResponse,
  TopicDetail,
  AnswerRequest,
  AnswerResponse,
  Bootstrap,
  Capability,
  Coverage,
  LemmaDetail,
  LemmaStatus,
  ScoredSentence,
  TextDetail,
  TextSummary,
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

/**
 * PATCH is spelled out here rather than added to the shared http helper: the
 * account edit is the only partial update in the whole API, and http.ts carries
 * the verbs every other endpoint uses. The unwrapping is the same because the
 * error shape is the same — one central Go handler renders {"error": "..."}.
 */
async function patch<T>(path: string, body: unknown): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, {
      method: 'PATCH',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
  } catch {
    throw new ApiError(0, 'network-unreachable')
  }
  const text = await res.text()
  let payload: unknown
  try {
    payload = text ? JSON.parse(text) : undefined
  } catch {
    throw new ApiError(res.status, 'unexpected-response')
  }
  if (!res.ok) {
    const message =
      typeof payload === 'object' && payload !== null && 'error' in payload
        ? String((payload as { error: unknown }).error)
        : `request-failed-${res.status}`
    throw new ApiError(res.status, message)
  }
  return payload as T
}

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

  // ── accounts ──────────────────────────────────────────────────────────
  users: () => http.get<{ users: User[] }>('/api/users'),

  createUser: (body: CreateUserRequest) => http.post<User>('/api/users', body),

  updateUser: (id: number, body: UpdateUserRequest) => patch<User>(`/api/users/${id}`, body),

  deleteUser: (id: number) => http.del<void>(`/api/users/${id}`),

  changeOwnPassword: (body: ChangePasswordRequest) => http.post<void>('/api/me/password', body),

  // ── language ──────────────────────────────────────────────────────────
  analyze: (text: string) => http.post<AnalyzeResponse>('/api/analyze', { text }),

  breakdown: (form: string) =>
    http.get<Breakdown>(`/api/breakdown?form=${encodeURIComponent(form)}`),

  reportAnalysis: (body: { form: string; lemma?: string; feats?: string; note?: string }) =>
    http.post<{ ok: boolean }>('/api/morph/correction', body),

  retractAnalysisReport: (form: string) =>
    http.del<{ ok: boolean }>(`/api/morph/correction?form=${encodeURIComponent(form)}`),

  features: () => http.get<MorphFeature[]>('/api/features'),

  // ── review ────────────────────────────────────────────────────────────
  queue: () => http.get<QueueResponse>('/api/review/queue'),

  answer: (body: AnswerRequest) => http.post<AnswerResponse>('/api/review/answer', body),

  undo: () => http.post<UndoResult>('/api/review/undo'),

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
    http.post<{
      noteId: number
      directions: string[]
      targetFeats: string[]
      duplicate: boolean
    }>('/api/notes', body),

  mastery: () => http.get<{ features: FeatureMastery[] }>('/api/stats/mastery'),

  overview: () => http.get<Overview>('/api/stats/overview'),

  takeCheckpoint: (body: { code: string; attested: string[]; notes?: string }) =>
    http.post<CheckpointVerdict>('/api/checkpoints', body),

  attestHuman: (happened: boolean) =>
    http.post<{ ok: boolean }>('/api/today/human', { happened }),

  // ── the guided course ─────────────────────────────────────────────────
  course: () => http.get<CourseResponse>('/api/course'),

  enrolUnit: (code: string) =>
    http.post<EnrolResult>(`/api/course/${encodeURIComponent(code)}/enrol`),

  resources: () => http.get<{ resources: Resource[] }>('/api/resources'),

  // ── curriculum & drills ───────────────────────────────────────────────
  curriculum: () => http.get<CurriculumResponse>('/api/curriculum'),

  topic: (code: string) => http.get<TopicDetail>(`/api/curriculum/${code}`),

  drillNext: (topic?: string, count = 10) =>
    http.get<DrillNextResponse>(
      `/api/drill/next?count=${count}` + (topic ? `&topic=${encodeURIComponent(topic)}` : ''),
    ),

  drillAnswer: (body: DrillAnswerRequest) =>
    http.post<DrillAnswerResponse>('/api/drill/answer', body),

  placement: () => http.get<PlacementResponse>('/api/placement'),

  placementAction: (action: 'apply' | 'reset') =>
    http.post<{ applied: number; action: string }>('/api/placement', { action }),

  // ── reading ───────────────────────────────────────────────────────────
  texts: () => http.get<{ texts: TextSummary[] }>('/api/texts'),

  createText: (body: { title?: string; body: string; kind?: string; sourceUrl?: string }) =>
    http.post<{ id: number; title: string; coverage: Coverage }>('/api/texts', body),

  text: (id: number) => http.get<TextDetail>(`/api/texts/${id}`),

  deleteText: (id: number) => http.del<void>(`/api/texts/${id}`),

  nextSentences: (id: number, limit = 10) =>
    http.get<{ sentences: ScoredSentence[] }>(`/api/texts/${id}/next?limit=${limit}`),

  lemma: (id: number) => http.get<LemmaDetail>(`/api/lemma/${id}`),

  // ── ear trainer ───────────────────────────────────────────────────────
  hvptSession: () => http.get<HvptSession>('/api/hvpt/session'),

  hvptAnswer: (body: { trialId: string; chosen: 'a' | 'b'; rtMs: number }) =>
    http.post<HvptAnswer>('/api/hvpt/answer', body),

  hvptContrasts: () =>
    http.get<{ contrasts: HvptContrast[]; talkers: number }>('/api/hvpt/contrasts'),

  // ── listening ─────────────────────────────────────────────────────────
  listenUpload: (form: FormData) =>
    http.postForm<{ id: number; title: string; audioPath: string; coverage: Coverage; verdict: CoverageVerdict }>(
      '/api/listen/material',
      form,
    ),

  listenState: (id: number) => http.get<ListenState>(`/api/listen/${id}/state`),

  listenProgress: (id: number, body: { pass: number; positionS: number; secondsHeard: number }) =>
    http.post<{ ok: boolean; totalHours: number }>(`/api/listen/${id}/progress`, body),

  setKnown: (lemmaId: number, status: LemmaStatus) =>
    http.post<void>('/api/knowledge/lemma', { lemmaId, status }),
}

export { ApiError } from './http'
