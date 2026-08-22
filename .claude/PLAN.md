# tanulom — implementation plan

> **Status:** Phase 0 in progress · **Last updated:** 2026-08-22
> This is the living plan. It is granular on purpose. Update it as phases complete — a phase
> is not done until its checklist is ticked *and* its documentation is written.

## Quick links

| Want to know | Read |
|---|---|
| Why any pedagogical decision was made | [`research/README.md`](research/README.md) → the numbered note |
| The original brief, verbatim | [`research/00-source-plan.md`](research/00-source-plan.md) |
| Architectural choices and their alternatives | [`DECISIONS.md`](DECISIONS.md) |
| How the system behaves | [`../docs/ARCHITECTURE.md`](../docs/ARCHITECTURE.md) |
| Pedagogy → citation map | [`../docs/PEDAGOGY.md`](../docs/PEDAGOGY.md) |
| Language pack schema & pipeline | [`../docs/LANGPACK.md`](../docs/LANGPACK.md) |
| Screens, keyboard, a11y | [`../docs/UX.md`](../docs/UX.md) |
| Endpoints | [`../docs/API.md`](../docs/API.md) |
| Running it locally | [`../docs/DEVELOPMENT.md`](../docs/DEVELOPMENT.md) |
| LLM prompt templates | [`prompts/`](prompts/) |

---

## 1. What this is

A self-hosted Hungarian trainer that tracks **two independent knowledge state spaces** —
lemmas and morphological features — instead of treating a wordform as an atom.

The thesis comes from [`research/00-source-plan.md`](research/00-source-plan.md) §7.2:

> `házaimban` is not one unit — it is `ház` + `-a-` + `-im` + `-ban`: one lemma plus three
> grammatical facts, each learned, forgotten and re-learned on its own schedule.

Everything distinctive about this app follows from taking that literally. See
[`../docs/ARCHITECTURE.md` §2](../docs/ARCHITECTURE.md).

### Fixed decisions

| Decision | Choice | ADR |
|---|---|---|
| Backend | Go core, single static binary, optional sidecars | [ADR-001](DECISIONS.md#adr-001) |
| Runtime morphology | Precomputed language pack, no Python at runtime | [ADR-002](DECISIONS.md#adr-002) |
| Scheduler | FSRS-6 via `go-fsrs/v4`, over two state spaces | [ADR-003](DECISIONS.md#adr-003) |
| Locales | RU + EN, both complete, switchable | [ADR-004](DECISIONS.md#adr-004) |
| Speech/LLM | Pluggable; local default, OpenAI-compatible alternative | [ADR-005](DECISIONS.md#adr-005) |
| v1.0 scope | Core with zero external dependencies | [ADR-006](DECISIONS.md#adr-006) |
| Deployment | upmonitor-style: one `docker run`, one volume | [ADR-007](DECISIONS.md#adr-007) |

---

## 2. Phase 0 — Foundation & documentation

**Goal:** every claim has a note; every note says what it changes in the code; the repo is
scaffolded and CI-ready. No app code.

- [x] Repository scaffold (`backend/`, `web-ui/`, `services/`, `langpack/`, `docs/`, `.claude/`)
- [x] `.claude/research/` — 18 notes (`00` source plan + `01`–`17`)
- [x] `.claude/research/pdf/` — 8 open-access PDFs + provenance ledger
- [x] `.claude/PLAN.md` (this file)
- [x] `.claude/DECISIONS.md` — 7 ADRs
- [x] `.claude/prompts/` — prompt templates with rationale
- [x] `docs/PEDAGOGY.md` — feature → citation map, divergences from the source plan
- [x] `docs/LANGPACK.md` — pack schema, pipeline, licences
- [x] `docs/UX.md` — screens, keyboard map, colour semantics, a11y
- [x] `docs/ARCHITECTURE.md` — package graph, data ownership, where-to-change map
- [x] `docs/API.md`, `docs/CONFIGURATION.md`, `docs/DEVELOPMENT.md`, `docs/SELFHOSTING.md`
- [x] `CLAUDE.md`, `README.md`, `LICENSE`, `.gitignore`, `.dockerignore`
- [x] `.github/workflows/ci.yml`
- [x] `git init` + initial commit

**Exit criterion:** `grep -rn '\[R[0-9]' docs/ .claude/PLAN.md` — every referenced note exists
and every note contains a "What this changes in our code" section.

---

## 3. Phase 1 — Skeleton that runs

**Goal:** `docker run` → setup wizard → empty dashboard. Nothing pedagogical yet.

### Backend

- [x] `go mod init tanulom`; pin Go 1.26, Fiber v3, `modernc.org/sqlite`, `pressly/goose/v3`,
      `go-fsrs/v4`, `golang.org/x/crypto/bcrypt`, `gopkg.in/yaml.v3`
- [x] `internal/logger` — slog JSON, level from `TANULOM_LOG_LEVEL`
- [x] `internal/state` — `state.json` outside the data dir, records the active data dir
- [x] `internal/config` — `Config` struct, `Load`/`Save`/`Validate`, atomic write via
      temp+rename, `Clone()` for copy-on-write; `normalize()` fills only zero fields
      (never rewrites a user's existing values)
- [x] `internal/db` — `Open(path)` with the WAL/busy_timeout/foreign_keys DSN,
      `SetMaxOpenConns(1)`, embedded goose migrations run inside `Open`
- [x] `migrations/00001_init.sql` — `user`, `session`, `settings`
- [x] `internal/auth` — bcrypt hash/verify, session token mint/verify, expiry
- [x] `internal/web` — `//go:embed all:dist`, `FS()`
- [x] `internal/api` — `Server` owning config+db; Fiber v3 app; central `ErrorHandler` →
      `{"error": msg}`; `auth`/`admin` middleware; rate limiter on auth routes; SPA catch-all
- [x] `handlers_auth.go` — `POST /api/setup`, `POST /api/login`, `POST /api/logout`,
      `GET /api/me`
- [x] `handlers_settings.go` — `GET/PUT /api/settings`
- [x] `GET /api/capabilities` — probes optional services, returns status + latency
- [x] `cmd/tanulom/main.go` — flags (`--data-dir`, `--addr`), env fallback, graceful shutdown
- [x] Tests: config round-trip, atomic save, auth hash/verify, migration idempotency

### Frontend

- [x] `npm create vue@latest` scaffold; TS, Pinia, vue-router, ESLint+oxlint, Prettier
- [x] Tailwind v4 via `@tailwindcss/vite`; dark-first token set in `assets/tailwind.css`
- [x] shadcn-vue primitives — **built so far:** button, input, label, card (+header/title/
      description/content/footer), select; plus `vue-sonner` for toasts.
      **Deferred until a screen needs them:** dialog, switch, tabs, table, tooltip, separator,
      dropdown-menu, badge, avatar. Adding a primitive nothing renders is dead code.
- [x] `vue-i18n` with `locales/ru.json` + `locales/en.json` — **both keys added together, always**
- [x] `src/api/http.ts` — fetch wrapper, `{error}` unwrapping, 401 → login redirect
- [x] `src/stores/` — `auth`, `settings`, `ui`, `capabilities`
- [x] `AppShell` / `AppSidebar` / `AppTopbar`; router with setup/login/role guards
- [x] Views: `SetupView`, `LoginView`, `TodayView` (empty state), `SettingsView`, `NotFoundView`
- [x] `ThemeToggle`, `LocaleToggle`, `CapabilityDots`

### Deployment

- [x] `Dockerfile` — node build → `--platform=$BUILDPLATFORM golang` cross-compile
      (`CGO_ENABLED=0`) → `gcr.io/distroless/static`
- [x] `docker-compose.yml` — core service + `nlp`/`speech`/`llm` profiles (stubs)
- [x] `.github/workflows/docker-publish.yml` — buildx amd64+arm64 → GHCR on tag
- [x] `.dockerignore`

**Exit criteria**
- [x] `cd backend && go vet ./... && go test ./...` clean
- [x] `cd web-ui && npm run type-check && npm run lint && npm run build` clean
- [x] Single binary serves the SPA + API; setup → Today → Settings verified in a browser
- [x] Clamped values are reported to the user (toast), not applied silently
- [x] Both locales render; capability dots show all four services correctly degraded
- [x] `docker compose config` valid; `docker compose up -d` resolves to the core service
      alone (the `nlp`/`speech`/`llm` services are correctly excluded without a profile)
- [ ] **`docker build` NOT verified** — the Docker daemon in this environment cannot reach
      any registry (`docker pull hello-world` hangs), so the base images
      (`node:24-alpine`, `golang:1.26`, `gcr.io/distroless/static`) never download. This is
      an environment limitation, not a known defect in the Dockerfile: the same three-stage
      pattern is proven in `antlko/upmonitor`, and every step it runs
      (`npm ci`, `npm run build-only`, `go mod download`, `CGO_ENABLED=0 go build`) has been
      run and verified natively here. **Run `docker build -t tanulom .` on a machine with
      registry access before tagging a release.**
- [ ] Multi-arch build succeeds for `linux/arm64` (same blocker; CI will exercise it on tag)

**Known gaps carried into Phase 2**
- `services/{nlp,tts,asr}` are documented placeholders; the `nlp` and `speech` compose
  profiles will not start until Phase 8–9. Stated in `services/README.md` and in the
  compose file itself rather than left to be discovered.
- No PWA/offline yet (`vite-plugin-pwa` lands with the review queue in Phase 3 — there is
  nothing worth precaching until then).

---

## 4. Phase 2 — Language pack v1

**Goal:** `POST /api/analyze {"text":"a házaimban"}` returns a correct lemma + feature
decomposition for every token, **offline, with no sidecar running**.

Full spec: [`../docs/LANGPACK.md`](../docs/LANGPACK.md).

### Pipeline (`langpack/`, Python — build-time only, never shipped)

- [ ] `pipeline/fetch.py` — pinned URLs + SHA256 into `cache/`; refuses to proceed on mismatch
- [ ] `pipeline/wiktionary.py` — wiktextract JSONL → `lemma`, `lemma_gloss` (en), `paradigm`, IPA
- [ ] `pipeline/wiktionary_ru.py` — RU edition → `lemma_gloss` (ru); EN fallback where absent
- [ ] `pipeline/frequency.py` — `hu_50k` + Webcorpus → `freq_rank`, `freq_pm`
- [ ] `pipeline/analyse.py` — HuSpaCy `hu_core_news_lg` over Tatoeba + Webcorpus sample →
      `sentence_token`; expand `wordform` from corpus forms ∪ Wiktionary paradigms
- [ ] `pipeline/features.py` — derive `morph_feature` rows incl. allomorph tables from harmony
      class (`-ban/-ben`, `-hoz/-hez/-höz`, …); RU+EN labels; `teach_order`; `prereq_ids`
- [ ] `pipeline/sentences.py` — Tatoeba HU + EN/RU links; **filter audio by per-clip licence**;
      difficulty score
- [ ] `pipeline/minimal_pairs.py` — build the contrast inventory ([R7](research/07-hvpt.md));
      render Piper × 3 voices; select Common Voice HU clips per contrast
- [ ] `pipeline/curriculum.py` — `grammar_topic` DAG (~60 topics), ordering from
      [R15](research/15-hungarian-difficulty.md)
- [ ] `pipeline/ngram.py` — 2/3/4-gram counts for the attestation guardrail
- [ ] `pipeline/emit.py` — write SQLite, create indexes, `VACUUM`, checksum, `manifest.json`
- [ ] `LICENSES.md` — per-source attribution ledger

### Backend

- [ ] `internal/lang` — locate/verify/`ATTACH` the pack; version + schema compatibility check;
      refuse to start on a major mismatch with a clear message
- [ ] `internal/morph/tokenize.go` — Hungarian-aware tokeniser (digraphs `cs gy ly ny sz ty zs`,
      `dzs`; clitics; hyphenation; sentence split)
- [ ] `internal/morph/lookup.go` — `form → []Analysis`; frequency prior for ambiguity;
      `Uncertain` flag when >1 analysis and no sidecar
- [ ] `internal/morph/fallback.go` — suffix-stripper from `morph_feature` allomorphs for
      unknown forms
- [ ] `internal/morph/segment.go` — morpheme segmentation (stem + ordered affixes)
- [ ] `internal/morph/diff.go` — align two segmentations, classify the error
      (harmony / case / definiteness / possessor / length / spelling)
- [ ] `internal/morph/explain.go` — human-readable RU+EN explanation of a form
- [ ] `handlers_analyze.go` — `POST /api/analyze`, `GET /api/lemma/:id`, `GET /api/paradigm/:id`
- [ ] Pack download-on-first-run with progress; `--build-arg BAKE_LANGPACK=1` alternative
- [ ] `.github/workflows/langpack.yml`

### Tests

- [ ] **Golden test** over a held-out UD Hungarian sample: lemma accuracy ≥ 95%, feature-set
      exact match ≥ 90%. This is the quality gate for the whole product — if it fails, `n+1`
      and the heatmap are wrong.
- [ ] Tokeniser: digraph, clitic, hyphenation, sentence-boundary fixtures
- [ ] `diff.go`: table-driven over every error class
- [ ] Fallback analyser on deliberately out-of-vocabulary forms

---

## 5. Phase 3 — SRS core (T1)

**Goal:** a full day's review cycle runs; replaying a log reproduces state exactly; `.apkg`
imports into real Anki.

- [ ] `migrations/00002_srs.sql` — `note`, `card`, `review_log`, `lemma_state`, `morph_state`
- [ ] `internal/srs/scheduler.go` — `go-fsrs/v4` wrapper; FSRS-6 defaults; desired retention
      from settings, clamped 0.80–0.95 ([R3](research/03-fsrs.md))
- [ ] `internal/srs/directions.go` — the five directions; goal-based weighting
      ([R12](research/12-retrieval-direction.md))
- [ ] `internal/srs/emit.go` — **dual-space emission**: one answer → 1 card review + N feature
      reviews; rating from correctness × latency; only features *required by the item* graded
- [ ] `internal/srs/queue.go` — daily queue assembly; new/review interleave; sibling burying
- [ ] `internal/srs/forecast.go` — 90-day workload simulation
- [ ] `internal/srs/governor.go` — refuse to raise the new-card limit past sustainable load;
      always show the curve first
- [ ] `internal/export/apkg.go` — `.apkg` writer (zip + `collection.anki21` SQLite + `media`)
- [ ] `internal/export/csv.go`, `internal/export/backup.go` (full zip export/import)
- [ ] `handlers_review.go` — `GET /api/review/queue`, `POST /api/review/answer`,
      `POST /api/review/undo`, `GET /api/review/forecast`
- [ ] `ReviewView.vue` — chrome-free, keyboard-only (`space`/`1-4`/`e`/`s`/`u`), audio autoplay,
      morpheme diff on answer, feature chips showing what was graded
- [ ] `stores/review.ts` — optimistic answer, undo buffer, prefetch next card
- [ ] **Property test:** replay `review_log` through FSRS → stored state reproduced exactly
- [ ] Round-trip test: export `.apkg` → re-import → note/card counts and fields preserved

---

## 6. Phase 4 — Reader + mining (T2)

**Goal:** read a Telex article end to end with lookups and one-key card creation.

- [ ] `migrations/00003_texts.sql` — `text`, `text_token`
- [ ] `internal/mining/ingest.go` — paste, URL (readability extraction), `.srt`, `.epub`, `.txt`
- [ ] `internal/mining/score.go` — the `n+1` scorer ([`../docs/ARCHITECTURE.md` §5.1](../docs/ARCHITECTURE.md));
      weights from `config.yaml`
- [ ] `internal/mining/dedup.go` — near-duplicate suppression
- [ ] `internal/stats/coverage.go` — **both** lemma coverage and element coverage
      ([R6](research/06-lexical-coverage.md))
- [ ] `handlers_texts.go` — CRUD, tokenised fetch, mark-known/ignore, `n+1` rail
- [ ] `ReaderView.vue` — two-pane; knowledge colouring; pinned coverage meter with the
      95/98% lines labelled *"English-derived, indicative only"*
- [ ] `WordDetailRail.vue` — lemma, **breakdown of this exact form**, gloss (RU/EN), IPA, audio,
      full paradigm table, frequency, Tatoeba examples
- [ ] Keyboard: `j/k` word, `n/p` sentence, `a` add card, `k` known, `i` ignore, `/` search
- [ ] "This analysis is wrong" report → `morph_correction` table overriding the pack

---

## 7. Phase 5 — Grammar + curriculum (T6)

**Goal:** the blocked→interleaved gate works, is visible, and is configurable.

- [ ] `migrations/00004_curriculum.sql` — `topic_progress`, `drill_attempt`
- [ ] `internal/curriculum/dag.go` — prerequisite resolution, unlocking, placement test
- [ ] `internal/drill/schedule.go` — **the gate** ([R5](research/05-interleaving.md)):
      `locked → blocked → interleaved → mastered`, with hysteresis on fallback
- [ ] `internal/drill/generate.go` — item generators: allomorph selection, case selection,
      conjugation choice, cloze, transformation, MCQ-with-distractors
- [ ] `internal/drill/harmony.go` — vowel harmony as a **motor skill**: mastery measured by
      latency, not accuracy alone ([R15](research/15-hungarian-difficulty.md))
- [ ] `internal/drill/definiteness.go` — weighted 55% definite-object / 25% demonstrative-object
      to match the documented error distribution
- [ ] `internal/drill/from_error.go` — error → underlying feature → targeted follow-up items
- [ ] `GrammarView.vue` — rule card first ([R4](research/04-explicit-instruction.md)), then
      drills; phase badge with a plain-language explanation of why you're in it
- [ ] `TopicMap.vue` — the DAG, rendered, with locks and prerequisites visible

---

## 8. Phase 6 — Ear + Listen (T5, T3)

- [ ] `migrations/00005_hvpt.sql` — `hvpt_trial`, `hvpt_contrast_state`
- [ ] `internal/hvpt/trial.go` — 2AFC builder; **refuses a session below `min_talkers` (5)**
- [ ] `internal/hvpt/state.go` — FSRS maintenance per contrast; **no "completed" state**
      ([R7](research/07-hvpt.md))
- [ ] `internal/hvpt/adaptive.go` — contrast selection from personal error history
- [ ] `EarView.vue` — two big buttons, immediate green/red, talker badge, running accuracy,
      distinct-talker count for the session
- [ ] `ListenView.vue` — **three-pass protocol as UI state** ([R13](research/13-shadowing.md));
      A/B loop; 0.5–1.5× with `preservesPitch`; dictation with char diff
- [ ] Coverage pre-check before playback; warn below ~90% element coverage

---

## 9. Phase 7 — Progress, checkpoints → **v1.0**

- [ ] `migrations/00006_progress.sql` — `checkpoint`, `daily_log`
- [ ] `internal/stats/heatmap.go` — per-feature mastery aggregation
- [ ] `internal/stats/streak.go` — streak = **obligations met**, not activity
      ([R14](research/14-motivation.md))
- [ ] `internal/stats/checkpoint.go` — M1/M3/M6/M12/M18 assessments; failure blocks advance and
      opens a diagnostic against the three failure modes
- [ ] `ProgressView.vue` — lemma curve vs A1/A2/B1/B2 targets; **morph heatmap**;
      `LocativeGrid.vue` (the 3×3 table coloured by mastery); forecast; checkpoint history
- [ ] `TodayView.vue` — four loop cards; **Loop 4 un-completable by the app**, with the reason
      shown
- [ ] Backup/restore zip; `docs/SELFHOSTING.md`; README screenshots; tag `v1.0.0`

---

## 10. Phase 8–10 — v1.1 onward

**Phase 8 · Speech (T4).** `services/tts` (Piper), `services/asr` (faster-whisper);
`internal/speech` provider abstraction + content-addressed cache; recording UI;
**explicit segmental feedback** naming the phoneme and enqueuing the HVPT contrast
([R8](research/08-capt-asr.md)); confidence gating because Hungarian WER ≈ 11%.

**Phase 9 · LLM tutor (T4 roleplay, T7).** `services/nlp` (HuSpaCy); `internal/llm`;
`internal/corpus/attest.go` — **the attestation guardrail**; writing correction in
prompt-then-reveal style ([R9](research/09-corrective-feedback.md)); voice roleplay
([R10](research/10-llm-tutors.md)).

**Phase 10 · v1.2 / v1.3.** Subtitle mining at scale, EPUB library, FSRS optimizer sidecar,
ECL mock exams, multi-user polish.

---

## 11. Standing rules

1. **Behaviour change → doc change in the same commit.** The `docs/` tree is the contract.
2. **Pedagogical code cites its note.** `// [R5] ... see .claude/research/05-interleaving.md`
3. **Both locales, always.** A PR adding an `en` key without `ru` fails review.
4. **Never edit an applied migration.** Add a new numbered file.
5. **`config.yaml` is mutated only via `Server.updateConfig`** (clone → validate → atomic save
   → swap). Secrets go in SQLite, never in YAML.
6. **The app must remain fully usable with every sidecar stopped.** Verified at each release.
7. **No feature ships that we cannot justify.** If the evidence is weak, say so in the UI.
