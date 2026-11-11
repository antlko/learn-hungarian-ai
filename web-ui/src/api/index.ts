import { http, ApiError } from './http'
import type {
  Example,
  CapabilityName,
  ChangePasswordRequest,
  CreateUserRequest,
  UpdateUserRequest,
  User,
  AnalyzeResponse,
  Breakdown,
  QueueScope,
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
  UnitPhrase,
  CourseProposal,
  CourseProposeResult,
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
  PracticeQueueResponse,
  WordsResponse,
  PracticeAnswerResponse,
  ScoredSentence,
  TextDetail,
  TextSummary,
  LibrarySet,
  LibrarySetDetail,
  TrashListResponse,
  TrashResponse,
  MTStatus,
  ComponentStatus,
  PackInstallStatus,
  RestartResponse,
  AIEntry,
  AIEntryStatus,
  ImportReport,
  ProfileSize,
  UserGloss,
  Translation,
  FeatureMastery,
  LevelEstimate,
  BandWordsResponse,
  DeckStages,
  Forecast,
  GovernorVerdict,
  Me,
  MorphFeature,
  QueueResponse,
  ReviewOrder,
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

/** `?scope=…`, or nothing at all — never `?scope=undefined`. */
function scopeQuery(scope?: QueueScope) {
  return scope ? `?scope=${encodeURIComponent(scope)}` : ''
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
  /** Admin: switch one optional service off/on without losing its settings (ADR-068). Returns the fresh capability list. */
  setServicePaused: (name: CapabilityName, paused: boolean) =>
    http.post<Capability[]>(`/api/admin/services/${name}/paused`, { paused }),

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
  queue: (scope: QueueScope = 'all', order: ReviewOrder = 'due') =>
    http.get<QueueResponse>(`/api/review/queue?scope=${scope}&order=${order}`),

  answer: (body: AnswerRequest) => http.post<AnswerResponse>('/api/review/answer', body),

  practiceQueue: (scope: QueueScope = 'all', size = 20, mode: 'random' | 'weak' = 'random') =>
    http.get<PracticeQueueResponse>(`/api/practice/queue?scope=${scope}&size=${size}&mode=${mode}`),

  words: (params: {
    status?: string
    stage?: string
    q?: string
    sort?: string
    page?: number
    pageSize?: number
  }) => {
    const qs = new URLSearchParams()
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== '') qs.set(k, String(v))
    }
    return http.get<WordsResponse>(`/api/words?${qs.toString()}`)
  },

  practiceAnswer: (body: AnswerRequest) =>
    http.post<PracticeAnswerResponse>('/api/practice/answer', body),

  undo: () => http.post<UndoResult>('/api/review/undo'),

  // ── «Знаю» ────────────────────────────────────────────────────────────
  /**
   * Set a word aside. Not a delete: the note, its cards and their FSRS state
   * survive, so a restore is a restore. It leaves the SCHEDULER — free practice
   * still reaches it through `scope: 'trash'`. ADR-025.
   */
  /**
   * `scope` is the pile the caller is looking at. It changes nothing about what
   * is trashed — it decides which breakdowns come back, so a text-scoped
   * session gets its own chip's number and not just the fixed three.
   */
  knowNote: (noteId: number, scope?: QueueScope) =>
    http.post<TrashResponse>(`/api/notes/${noteId}/know${scopeQuery(scope)}`),

  restoreNote: (noteId: number, scope?: QueueScope) =>
    http.post<TrashResponse>(`/api/notes/${noteId}/restore${scopeQuery(scope)}`),

  known: () => http.get<TrashListResponse>('/api/trash'),

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
    /** The library text this word was mined from — what makes «повторить
     *  слова из этого текста» findable later (note_origin, ADR-022). */
    textId?: number
  }) =>
    http.post<{
      noteId: number
      directions: string[]
      targetFeats: string[]
      duplicate: boolean
      /**
       * The lemma's status AFTER the call. The reader recolours the word from
       * this rather than guessing: `known` and `ignored` are declarations that
       * minting a card must never override, and that rule lives on the server.
       */
      lemmaStatus?: LemmaStatus
    }>('/api/notes', body),

  /** The estimated Hungarian level (ADR-047): a measurement with a stated method. */
  level: () => http.get<LevelEstimate>('/api/stats/level'),

  /** Words by stage (the Words list's rule). A word count, not a due count. */
  deckStages: () => http.get<DeckStages>('/api/stats/deck'),

  /** The highest-frequency words of one band the learner does not yet count. Read-only. */
  levelBand: (band: string, limit = 50) =>
    http.get<BandWordsResponse>(`/api/stats/level/band/${band}?limit=${limit}`),

  mastery: () => http.get<{ features: FeatureMastery[]; window: number }>('/api/stats/mastery'),

  /**
   * The whole Today/Progress payload. `days` is the window the activity chart
   * asked for — 30, 90 or 365; the server clamps anything else to 30. The
   * STREAK is always computed over the full ledger, never over this window.
   */
  overview: (days?: number) =>
    http.get<Overview>(`/api/stats/overview${days && days !== 30 ? `?days=${days}` : ''}`),

  takeCheckpoint: (body: { code: string; attested: string[]; notes?: string }) =>
    http.post<CheckpointVerdict>('/api/checkpoints', body),

  attestHuman: (happened: boolean) =>
    http.post<{ ok: boolean }>('/api/today/human', { happened }),

  // ── the guided course ─────────────────────────────────────────────────
  course: () => http.get<CourseResponse>('/api/course'),

  /** Up to four attested pack sentences that show the unit's words in use. */
  unitPhrases: (code: string) =>
    http.get<{ phrases: UnitPhrase[] }>(`/api/course/units/${encodeURIComponent(code)}/phrases`),

  enrolUnit: (code: string) =>
    http.post<EnrolResult>(`/api/course/${encodeURIComponent(code)}/enrol`),

  /** Ask the model for example sentences for a word the pack cannot illustrate. Stores nothing. */
  proposeUnitWord: (code: string, lemma: string, lang: string) =>
    http.post<CourseProposeResult>(
      `/api/course/units/${encodeURIComponent(code)}/words/${encodeURIComponent(lemma)}/propose?lang=${lang}`,
    ),

  /** Accept one proposed sentence: it becomes an AI-labelled card of the unit. */
  acceptUnitWord: (code: string, lemma: string, body: CourseProposal) =>
    http.post<{ lemma: string; duplicate: boolean }>(
      `/api/course/units/${encodeURIComponent(code)}/words/${encodeURIComponent(lemma)}/accept`,
      body,
    ),

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

  librarySets: () => http.get<LibrarySet[]>('/api/library/sets'),

  /** Preview of one set, without importing it. */
  librarySet: (code: string) =>
    http.get<LibrarySetDetail>(`/api/library/sets/${encodeURIComponent(code)}`),

  importLibrarySet: (code: string) =>
    http.post<{ textId: number; created: boolean }>(
      `/api/library/sets/${encodeURIComponent(code)}/import`,
      {},
    ),

  createText: (body: { title?: string; body: string; kind?: string; sourceUrl?: string }) =>
    http.post<{ id: number; title: string; coverage: Coverage }>('/api/texts', body),

  text: (id: number) => http.get<TextDetail>(`/api/texts/${id}`),

  deleteText: (id: number) => http.del<void>(`/api/texts/${id}`),

  nextSentences: (id: number, limit = 10) =>
    http.get<{ sentences: ScoredSentence[] }>(`/api/texts/${id}/next?limit=${limit}`),

  lemma: (id: number) => http.get<LemmaDetail>(`/api/lemma/${id}`),

  /** Attested sentences using this lemma, with whatever translations exist. */
  lemmaExamples: (id: number, limit = 3) =>
    http.get<{ examples: Example[] }>(`/api/lemma/${id}/examples?limit=${limit}`),

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

  /**
   * Machine-translate Hungarian sentences. 503 when `services.mt` is off or the
   * sidecar is not answering — a genuine failure path, which is why callers
   * check the capability before offering the control at all.
   */
  translate: (texts: string[], target: string) =>
    http.post<{ translations: Translation[] }>('/api/translate', { texts, target }),

  /**
   * What the translation sidecar is doing, in its own words.
   *
   * Distinct from `/api/capabilities`, which flattens every 5xx to
   * `reachable: false` — and a translator downloading its model answers 503 by
   * design, so through the probe "downloading" and "not deployed" are the same
   * grey row. This one carries `state`, the failure `detail` and the byte
   * counts, so Settings can tell them apart.
   */
  /**
   * The learner's own dictionary entry for a surface form, or an empty one.
   *
   * A miss is an empty entry rather than a 404: almost every word is a miss,
   * and a 404 would put a special case in every caller.
   */
  gloss: (form: string) => http.get<UserGloss>(`/api/glosses/${encodeURIComponent(form)}`),

  saveGloss: (form: string, body: { gloss: string; source: 'manual' | 'machine'; model?: string }) =>
    http.put<UserGloss>(`/api/glosses/${encodeURIComponent(form)}`, body),

  deleteGloss: (form: string) => http.del(`/api/glosses/${encodeURIComponent(form)}`),

  /**
   * What an import is about to replace, in the learner's own units.
   *
   * The confirm dialog must name what it is erasing, and "24 tables" is not a
   * thing anybody has. See ADR-040.
   */
  profileSize: () => http.get<ProfileSize>('/api/profile/size'),

  /**
   * Download the whole profile.
   *
   * Deliberately NOT through `http`: the response is a zip, not JSON, and it
   * can be tens of megabytes. The browser's own download machinery handles it
   * better than anything we would write, so the link is simply followed.
   */
  profileExportUrl: () => '/api/profile/export',

  profileImport: (file: File) => {
    const form = new FormData()
    form.append('profile', file)
    return http.postForm<ImportReport>('/api/profile/import', form)
  },

  /**
   * Time spent with a text, for the daily ledger.
   *
   * The reader was the one loop that reported nothing, so an hour with a text
   * left «21 / 90 мин» unmoved and the day unmet. The server clamps each
   * report to 60 s and echoes what it actually credited. ADR-044.
   */
  readingProgress: (seconds: number) =>
    http.post<{ seconds: number; activeMs: number }>('/api/reading/progress', { seconds }),

  /**
   * Whether a model can write a dictionary entry, and which one. Asked BEFORE
   * the button is offered: offering it and then failing is the pattern ADR-001
   * exists to avoid.
   */
  aiEntryStatus: () => http.get<AIEntryStatus>('/api/words/ai-entry/status'),

  /**
   * A dictionary entry from the model, for a word the pack does not have.
   *
   * Displayed and labelled. It writes nothing — saving is a separate, explicit
   * `saveGloss` with `source: 'machine'`. ADR-042.
   */
  aiEntry: (form: string, lang: string) =>
    http.post<AIEntry>(`/api/words/${encodeURIComponent(form)}/entry?lang=${lang}`, {}),

  mtStatus: () => http.get<MTStatus>('/api/mt/status'),

  /** Fetch the model, or retry after a failed download. Admin-only. */
  mtWarmup: () => http.post<MTStatus>('/api/mt/warmup', {}),

  /** Admin: the pack and every optional sidecar in one list (ADR-049). */
  adminComponents: () => http.get<ComponentStatus[]>('/api/admin/components'),
  /** Admin: install | start | stop | uninstall the app-run copy of mt. 202 = started; 409 = a job is running. */
  componentAction: (id: 'mt', action: 'install' | 'start' | 'stop' | 'uninstall') =>
    http.post<unknown>(`/api/admin/components/${id}/${action}`, {}),
  /** Admin: start the full-pack download. 422 errors.packNotPublished, 409 busy. */
  packInstallStart: () => http.post<PackInstallStatus>('/api/admin/pack/install', {}),
  packInstallStatus: () => http.get<PackInstallStatus>('/api/admin/pack/install'),
  /** Admin: stop the process; whether anything brings it back is `supervised`. */
  adminRestart: () => http.post<RestartResponse>('/api/admin/restart', {}),
}

export { ApiError } from './http'
export { tutorApi, TutorError } from './tutor'
