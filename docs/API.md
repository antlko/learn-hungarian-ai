# API

REST over JSON. Cookie sessions. All routes under `/api`.

> **Status legend:** ✅ implemented · 🔨 planned this phase · ⏳ later phase.
> Endpoints are documented **when they are built**, in the same change. An endpoint that appears
> here without a status marker is a documentation bug.

## Conventions

- **Errors** — every failure renders as `{"error": "message"}` with an appropriate status, via
  the central Fiber `ErrorHandler`. Handlers return `fiber.NewError(code, msg)`; they never
  format error bodies themselves.
- **Success** — `c.JSON(dto)` or `c.Status(201).JSON(dto)`.
- **Casing** — DTOs are camelCase. `web-ui/src/types/` mirrors
  `backend/internal/api/dto.go`; keep them in sync when a shape changes.
- **Auth** — `session` cookie, `HttpOnly`, `SameSite=Lax`, `Secure` when served over TLS.
  Route guards in the SPA are UX only; **the API enforces roles independently**.
- **Rate limiting** — auth routes are limited per IP.
- **Time** — RFC 3339 UTC everywhere.

---

## Setup & auth

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/bootstrap` | 🔨 | `{needsSetup, packPresent, packVersion, locales}` — unauthenticated; drives the wizard |
| `POST` | `/api/setup` | 🔨 | first admin. `{email, password, uiLocale, glossLang}`. 409 once a user exists |
| `POST` | `/api/login` | 🔨 | `{email, password}` → sets cookie |
| `POST` | `/api/logout` | 🔨 | clears cookie |
| `GET` | `/api/me` | 🔨 | current user + settings |

## Settings & capabilities

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/settings` | 🔨 | secrets stripped; presence exposed as `secrets: {openaiKey: true}` |
| `PUT` | `/api/settings` | 🔨 | partial; **blank/omitted secret preserves the stored one** |
| `GET` | `/api/capabilities` | 🔨 | `[{name, configured, reachable, latencyMs, detail}]`, cached 30 s |
| `POST` | `/api/capabilities/:name/test` | ⏳ | round-trip test against a provider |

## Language & morphology

| Method | Path | Status | Notes |
|---|---|---|---|
| `POST` | `/api/analyze` | 🔨 P2 | `{text}` → tokens with `lemma`, `upos`, `feats`, `segments`, `certainty` |
| `GET` | `/api/lemma/:id` | 🔨 P2 | lemma, glosses (ru/en + `glossFallback` flag), IPA, frequency, CEFR |
| `GET` | `/api/paradigm/:lemmaId` | 🔨 P2 | full declension/conjugation table |
| `GET` | `/api/features` | 🔨 P2 | the `morph_feature` inventory with labels and teach order |
| `POST` | `/api/morph/correction` | ⏳ P4 | learner-reported wrong analysis → overlay |

`POST /api/analyze` response shape:

```json
{ "tokens": [ {
    "idx": 2, "form": "házaimban", "lemma": "ház", "lemmaId": 4211, "upos": "NOUN",
    "feats": { "Case": "Ine", "Number[psor]": "Plur", "Person[psor]": "1", "Number": "Sing" },
    "segments": [
      {"form":"ház","kind":"stem"},
      {"form":"a","kind":"linking"},
      {"form":"im","kind":"suffix","featureId":"Number[psor]=Plur"},
      {"form":"ban","kind":"suffix","featureId":"Case=Ine","allomorphRule":"back"}
    ],
    "certainty": "certain"
} ], "sentences": [{"start":0,"end":5}] }
```

`certainty` ∈ `certain | uncertain | derived` — surfaced in the UI, never hidden.

## Review (SRS)

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/review/queue` | ⏳ P3 | today's queue; `?limit=` |
| `POST` | `/api/review/answer` | ⏳ P3 | `{cardId, rating, typed?, durationMs}` → next card + graded features |
| `POST` | `/api/review/undo` | ⏳ P3 | reverts the last answer including feature reviews |
| `GET` | `/api/review/forecast` | ⏳ P3 | 90-day projection for the governor |
| `GET/POST/PUT/DELETE` | `/api/notes[/:id]` | ⏳ P3 | notes and their card directions |

## Texts & mining

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET/POST/DELETE` | `/api/texts[/:id]` | ⏳ P4 | ingest paste/URL/`.srt`/`.epub` |
| `GET` | `/api/texts/:id/tokens` | ⏳ P4 | pre-analysed tokens + per-token knowledge state |
| `GET` | `/api/texts/:id/coverage` | ⏳ P4 | `{lemmaCoverage, elementCoverage, unknownLemmas, unknownFeatures}` |
| `POST` | `/api/knowledge/lemma` | ⏳ P4 | mark known / learning / ignored |
| `GET` | `/api/mining/next` | ⏳ P4 | the `n+1` rail |

## Curriculum & drills

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/curriculum` | ⏳ P5 | topic DAG with per-topic phase and prerequisites |
| `GET` | `/api/curriculum/:code` | ⏳ P5 | rule card + progress |
| `GET` | `/api/drill/next` | ⏳ P5 | items honouring the blocked→interleaved gate |
| `POST` | `/api/drill/answer` | ⏳ P5 | grades the item **and** emits feature reviews |
| `POST` | `/api/placement` | ⏳ P5 | placement test → initial state |

## HVPT & listening

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/hvpt/session` | ⏳ P6 | trial set; **fails if fewer than `min_talkers` are available** |
| `POST` | `/api/hvpt/answer` | ⏳ P6 | `{trialId, chosen, rtMs}` |
| `GET` | `/api/hvpt/contrasts` | ⏳ P6 | per-contrast state and accuracy |
| `GET/POST` | `/api/listen/*` | ⏳ P6 | material, pass state, dictation grading |

## Progress & export

| Method | Path | Status | Notes |
|---|---|---|---|
| `GET` | `/api/stats/overview` | ⏳ P7 | lemma count, streak, adherence |
| `GET` | `/api/stats/heatmap` | ⏳ P7 | per-feature mastery |
| `GET` | `/api/stats/forecast` | ⏳ P7 | review load projection |
| `GET/POST` | `/api/checkpoints` | ⏳ P7 | assessments; failure blocks curriculum advance |
| `GET` | `/api/export/apkg` | ⏳ P3 | `.apkg` download |
| `GET` | `/api/export/backup` | ⏳ P7 | full zip (config + db + media; pack excluded — it is reproducible) |
| `POST` | `/api/import/backup` | ⏳ P7 | validate → snapshot to `backups/` → apply |

## Speech & LLM *(v1.1)*

| Method | Path | Status | Notes |
|---|---|---|---|
| `POST` | `/api/speech/tts` | ⏳ P8 | text → audio; content-addressed cache |
| `POST` | `/api/speech/asr` | ⏳ P8 | audio → transcript + confidence |
| `POST` | `/api/speech/score` | ⏳ P8 | target + audio → **explicit segmental feedback** + enqueued HVPT contrasts |
| `POST` | `/api/llm/generate` | ⏳ P9 | comprehensible input; returns `attested` per span |
| `POST` | `/api/llm/correct` | ⏳ P9 | staged correction — **server releases stage 3 only after an attempt** |
| `POST` | `/api/llm/roleplay` | ⏳ P9 | scenario turn, optionally voice in/out |

## Static

| Path | Serves |
|---|---|
| `/media/*` | cached TTS, HVPT clips, learner recordings |
| `/*` | SPA catch-all from the embedded FS |
