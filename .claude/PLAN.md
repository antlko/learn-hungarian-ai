# tanulom — implementation plan

> **Status:** Phases 0–7 complete, plus the post-v1.0 work in [§10](#10-after-the-plan--what-v10-turned-out-to-need)
> (accounts, the guided course, the embedded starter pack, the Makefile, the removal of
> export/backup, review sets with per-card memory, and the grammar tutor) · **Not shipped:** the `v1.0.0` tag and a verified
> `docker build` · **Last updated:** 2026-09-04
> This is the living plan. It is granular on purpose. Update it as phases complete — a phase
> is not done until its checklist is ticked *and* its documentation is written. **A box is
> ticked only when the named file exists and does what the line says**; a line whose file was
> renamed or dropped gets corrected here rather than left to mislead the next session.

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
| Drill items | held server-side between serve and answer | [ADR-008](DECISIONS.md#adr-008) |
| Prerequisites | met at `interleaved`; un-drillable topics are `explained` | [ADR-009](DECISIONS.md#adr-009) |

---

## 2. Phase 0 — Foundation & documentation

**Goal:** every claim has a note; every note says what it changes in the code; the repo is
scaffolded and CI-ready. No app code.

- [x] Repository scaffold (`backend/`, `web-ui/`, `services/`, `langpack/`, `docs/`, `.claude/`)
- [x] `.claude/research/` — 20 notes (`00` source plan + `01`–`19`)
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
- [x] **`docker build` verified.** The registry became reachable and the three-stage build runs
      clean: a **33 MB** distroless image, `nonroot:nonroot`, no shell. Run against an empty
      volume it installs the starter pack and its Opus clips (342 since ADR-033) by itself and reports
      `packEdition: "starter"` — the whole out-of-the-box claim, exercised through the
      container rather than argued for. Setup, course enrolment (33 notes), a 40-trial ear
      session with audio served, and `házaimban → ház+a+i+m+ban` all confirmed inside it.
- [x] Cross-architecture build verified: `--platform linux/amd64` produces a 34 MB amd64 image
      from an arm64 host, which is what `CGO_ENABLED=0` and `modernc.org/sqlite` buy.
      A single **multi-platform** invocation still needs a buildx driver with the containerd
      image store; `docker-publish.yml` uses one, so the manifest list is built on tag

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

- [x] `pipeline/sources.py` — pinned URLs + SHA256 into `cache/`; refuses to proceed on mismatch
      (named `fetch.py` in the original plan; it fetches *and* declares provenance, so it is
      named for the ledger it owns)
- [x] `pipeline/wiktionary.py` — wiktextract JSONL → `lemma`, `lemma_gloss` (en), `paradigm`, IPA
- [x] `pipeline/wiktionary_ru.py` — RU edition → `lemma_gloss` (ru); EN fallback where absent.
      8 620 lemmas in the full pack, 2 290 of the starter's 5 132, and all 853 course words —
      whose hand-written glosses are promoted over the dictionary's, because the unit means one
      sense and the dictionary lists four. `gloss_langs` is now `["en","ru"]` and `starter.py`
      recomputes it from the rows it copied. Where RU is absent the fallback badge still shows
- [x] `pipeline/frequency.py` — `hu_50k` + Webcorpus → `freq_rank`, `freq_pm`
- [x] `pipeline/harmony.py` — the vowel-harmony engine, with its own tests. Split out of the
      inflection code because harmony is the first thing a learner automates and the single
      most frequent source of suffix errors
- [x] `pipeline/inflect.py` — form generation, split by tolerance: recall-first for recognition,
      precision-first for anything a drill may mark correct
- [ ] `pipeline/analyse.py` — HuSpaCy `hu_core_news_lg` over Tatoeba + Webcorpus sample →
      `sentence_token`; expand `wordform` from corpus forms ∪ Wiktionary paradigms.
      **Not built:** `build.py` fills `sentence_token` by looking each token up in the generated
      `wordform` table instead (`confidence != 'low'`), which is why token resolution is 88.2%
      and not higher, and why those rows carry no `feats`
- [x] `pipeline/features.py` — derive `morph_feature` rows incl. allomorph tables from harmony
      class (`-ban/-ben`, `-hoz/-hez/-höz`, …); RU+EN labels; `teach_order`; `prereq_ids`
- [x] `pipeline/sentences.py` — Tatoeba HU + EN/RU links; **filter audio by per-clip licence**;
      difficulty score
- [x] `pipeline/minimal_pairs.py` — the contrast inventory ([R7](research/07-hvpt.md)) and its
      audio. Landed in Phase 6, where the ear trainer could actually exercise it
- [x] `pipeline/curriculum.py` — `grammar_topic` DAG (**28** topics on hu-v1, not the ~60 the
      original line guessed), ordering from [R15](research/15-hungarian-difficulty.md)
- [x] 2/3/4-gram counts for the attestation guardrail — built inside `pipeline/build.py`
      (there is no `ngram.py`; it is twenty lines and one pass over the sentence table)
- [x] `pipeline/emit.py` — write SQLite, create indexes, `VACUUM`, checksum, `manifest.json`
- [x] `LICENSES.md` — per-source attribution ledger

### Backend

- [x] `internal/lang` — locate/verify/`ATTACH` the pack; version + schema compatibility check;
      refuse to start on a major mismatch with a clear message
- [x] `internal/morph/tokenize.go` — Hungarian-aware tokeniser (digraphs `cs gy ly ny sz ty zs`,
      `dzs`; clitics; hyphenation; sentence split)
- [x] `internal/morph/lookup.go` — `form → []Analysis`; frequency prior for ambiguity;
      `Uncertain` flag when >1 analysis and no sidecar
- [x] `internal/morph/fallback.go` — suffix-stripper from `morph_feature` allomorphs for
      unknown forms
- [x] `internal/morph/segment.go` — morpheme segmentation with VERIFIED boundaries only;
      anything unjustifiable comes back as one honest fused ending
- [x] `internal/morph/diff.go` — align two segmentations, classify the error
      (harmony / case / definiteness / possessor / length / spelling)
- [x] `internal/morph/explain.go` — per-morpheme labels + allomorph rule, both locales,
      assembled from pack data (the backend never writes prose)
- [x] `handlers_analyze.go` — `POST /api/analyze`, `GET /api/lemma/:id`, `GET /api/paradigm/:id`
- [x] ~~Pack download-on-first-run with progress; `--build-arg BAKE_LANGPACK=1` alternative~~ —
      **superseded by the embedded starter pack** ([§10.3](#103-the-pack-is-always-there)). Both
      of the original options make the learner's first experience a download or a build flag;
      the pack that ships inside the binary makes the question not arise
- [x] `.github/workflows/langpack.yml` — **not** a pack build: `make langpack` needs tens of GB
      of scratch and hours of CPU, against a runner's 14 GB and six-hour ceiling, and the
      inputs are checksum-pinned so a maintainer's build is already reproducible. What CI
      checks is what goes stale silently — the 144 pipeline tests, every source declaring a
      licence and appearing in `LICENSES.md`, and (the one with teeth) that the **committed**
      starter artefacts install, open and carry a course. A pipeline change never followed by
      `make starter` fails there instead of shipping last month's syllabus inside the binary.
      The starter artefact IS committed (`backend/internal/lang/starter/`), so a clone builds a
      working binary without it; the 252 MB full pack is not, and has no release automation

### Tests

- [x] **88.2% token resolution** over the pack's own 60k-sentence corpus. Kept, but note what
      it measures: whether the lookup *found* something, using the pack's own tables. A lookup
      table cannot be wrong by that measure, because it **is** the measure.
- [x] **Golden test against held-out UD Hungarian-Szeged** — `internal/morph/golden_ud_test.go`,
      8 873 word tokens the pack never saw. This is the number that answers "was it right":

      | | measured on hu-v1 |
      |---|---|
      | coverage (an analysis at all) | 78.2% |
      | lemma (gold lemma among the candidates) | 75.1% — **96% of what it does analyse** |
      | UPOS | 70.0% |
      | feature set, exact, over the 13 modelled keys | 56.2% |

      The plan asked for lemma ≥ 95% / features ≥ 90% and those are **not** the numbers. The
      gap is a design consequence, not a defect queued for later: those figures assumed
      HuSpaCy, and what ships is a dictionary lookup handed one word with no sentence around
      it ([ADR-002](DECISIONS.md#adr-002)). `vár` is "castle" and "waits"; a human reads the
      clause, this analyser cannot. **Coverage, not correctness, is the binding constraint** —
      proper nouns (60% of 745 unanalysed) and verbs (45% of 927) dominate the misses.

      The treebank is CC BY-NC-SA, so it is a pinned source fetched into `langpack/cache`
      and never committed; the test skips with a message when it is absent. Floors sit ~3
      points under the measured values: they exist to catch a regression, not to look good.
- [x] Certainty labels earn their badge: `certain` analyses carry the gold lemma 93.8% of the
      time against `uncertain`'s 84.3%, so the badge sorts something real. `derived` measures
      higher than both (98.4% on 182 tokens) and is deliberately **not** asserted against them
      — the suffix fallback only runs on words the dictionary missed, which in running text are
      overwhelmingly transparent regulars. That is a selection effect, not a result.
- [x] Tokeniser: digraph, clitic, hyphenation, sentence-boundary fixtures
- [x] `diff.go`: table-driven over every error class
- [x] Fallback analyser on deliberately out-of-vocabulary forms (incl. must-refuse cases)

---

## 5. Phase 3 — SRS core (T1)

**Goal:** a full day's review cycle runs; replaying a log reproduces state exactly.

- [x] `migrations/00002_srs.sql` — `note`, `card`, `review_log`, `lemma_state`, `morph_state`
- [x] `internal/srs/scheduler.go` — `go-fsrs/v4` wrapper; FSRS-6 defaults; desired retention
      from settings, clamped 0.80–0.95 ([R3](research/03-fsrs.md))
- [x] `internal/srs/directions.go` — the five directions; goal-based weighting
      ([R12](research/12-retrieval-direction.md))
- [x] `internal/srs/emit.go` — **dual-space emission**: one answer → 1 card review + N feature
      reviews; rating from correctness × latency; only features *required by the item* graded
- [x] `internal/srs/queue.go` — daily queue assembly; new/review interleave; sibling burying
- [x] `internal/srs/forecast.go` — 90-day workload simulation, **and the governor**
      (`CheckNewCardLimit`, `GovernorVerdict`): refuse to raise the new-card limit past
      sustainable load, always showing the curve first. No separate `governor.go` — the
      refusal is the simulation's verdict and splitting them would have duplicated the model
- [x] ~~`internal/export/apkg.go`, `csv.go`, `backup.go`~~ — **removed**, see
      [§10.5](#105-export-and-backup-removed)
- [x] `handlers_review.go` — `GET /api/review/queue`, `POST /api/review/answer`,
      `POST /api/review/undo`, `GET /api/review/forecast`
- [x] `ReviewView.vue` — chrome-free, keyboard-only (`space`/`1-4`/`e`/`s`/`u`), audio autoplay,
      morpheme diff on answer, feature chips showing what was graded
- [x] `stores/review.ts` — optimistic answer, undo buffer, prefetch next card
- [x] **Property test:** replay `review_log` through FSRS → stored state reproduced exactly,
      fuzzed due dates included (fuzz is seeded); undo is delete-and-replay on the same code
      (`internal/srs/replay.go`, `undo.go`, `replay_test.go`)

---

## 6. Phase 4 — Reader + mining (T2)

**Goal:** read a Telex article end to end with lookups and one-key card creation.

- [x] `migrations/00003_texts.sql` — `text`, `text_token`
- [x] `internal/mining/ingest.go` — paste, URL (readability extraction), `.srt`, `.epub`, `.txt`
- [x] `internal/mining/score.go` — the `n+1` scorer ([`../docs/ARCHITECTURE.md` §5.1](../docs/ARCHITECTURE.md));
      weights from `config.yaml`
- [x] Duplicate suppression — a note is not created twice for the same sentence and
      target form; the client reports it rather than silently doubling review load
- [x] `internal/mining/coverage.go` — **both** lemma coverage and element coverage
      ([R6](research/06-lexical-coverage.md)). It lives in `mining`, not `stats`: coverage is a
      property of a text against a learner, computed while ingesting one, and `mining` imports
      nothing from `internal/` at all — which is what keeps it cheap to test
- [x] `handlers_texts.go` — CRUD, tokenised fetch, mark-known/ignore, `n+1` rail
- [x] `ReaderView.vue` — two-pane; knowledge colouring; pinned coverage meter with the
      95/98% lines labelled *"English-derived, indicative only"*
- [x] `components/reader/WordDetail.vue` — lemma, **breakdown of this exact form**, gloss (RU/EN), IPA, audio,
      full paradigm table, frequency, Tatoeba examples
- [x] Keyboard: `j/k` word, `n/p` sentence, `a` add card, `k` known, `i` ignore, `/` search
- [x] "This analysis is wrong" report → `morph_correction` per-user overlay, rendered as
      `corrected` (an override never masquerades as the dictionary); retractable; empty
      reports kept as signal for the next pack build

---

## 7. Phase 5 — Grammar + curriculum (T6)

**Goal:** the blocked→interleaved gate works, is visible, and is configurable.

- [x] `migrations/00004_curriculum.sql` — `topic_progress`, `drill_attempt`
- [x] `internal/curriculum/dag.go` — prerequisite resolution and unlocking;
      `placement.go` — the adaptive ladder and its seeding
- [x] `internal/drill/schedule.go` — **the gate** ([R5](research/05-interleaving.md)):
      `locked → explained → blocked → interleaved → mastered`, with hysteresis on fallback.
      Pure function of the counters; `resolve.go` walks the DAG with it
- [x] `internal/drill/generate.go` — item generators: allomorph selection, case selection,
      cloze, transformation, MCQ-with-distractors (conjugation choice in `definiteness.go`)
- [x] `internal/drill/harmony.go` — vowel harmony as a **motor skill**: mastery measured by
      latency, not accuracy alone ([R15](research/15-hungarian-difficulty.md))
- [x] `internal/drill/definiteness.go` — conjugation choice, weighted 55% definite-object /
      25% demonstrative-object to match the documented error distribution
- [x] `internal/drill/from_error.go` — error → underlying feature → targeted follow-up items,
      **including the classes a drill cannot treat** (vowel length routes to the ear trainer)
- [x] `GrammarView.vue` — rule card first ([R4](research/04-explicit-instruction.md)), then
      drills; phase badge with a plain-language explanation of why you're in it
- [x] `TopicMap.vue` — the DAG, rendered, with locks and prerequisites visible
- [x] `PlacementView.vue` — the ladder, and the verdict shown before it is applied

**Also in this phase** (not foreseen in the original list, all load-bearing):

- [x] `internal/drill/pending.go` — served items are held server-side, so the expected answer
      never reaches the client and a client cannot claim which features an item graded
- [x] `internal/drill/engine.go`, `store.go` — one transaction per answer: feature reviews,
      the attempt log and the topic's phase move together or not at all
- [x] `internal/morph/forms.go` — trust-ordered form resolution. Nothing that must be *right*
      reads the pack's `paradigm` table, which is generator-only and wrong where it disagrees
      with the dictionary
- [x] `internal/morph/casefold.go` — `COLLATE NOCASE` silently defeated `idx_wordform_form`,
      turning every lookup into a scan of two million rows (~110 ms). Fixed for the reader too
- [x] Language-pack defects found by generating from hu-v1 — see
      [`../docs/LANGPACK.md` §3a](../docs/LANGPACK.md); fixed in the pipeline, guarded at runtime
- [x] `internal/drill/realpack_test.go` — the generators run against the real 264 MB pack when
      it is present, skipped in CI

---

## 8. Phase 6 — Ear + Listen (T5, T3)

- [x] `migrations/00005_hvpt.sql` — `hvpt_trial`, `hvpt_contrast_state`, plus the listening
      side: `text.audio_path`, `listen_state` (the three passes), `listen_log` (the hour ledger)
- [x] `internal/hvpt/hvpt.go` — 2AFC session builder; **refuses below `minTalkers` with the
      numbers**; rotates talkers so consecutive trials never share a voice; FSRS maintenance
      per contrast with **no completed state** and a positive selection floor; adaptive
      weighting = due + error-driven ([R7](research/07-hvpt.md))
- [x] `internal/hvpt/pending.go` — the played member is server-held (ADR-008 for trials)
- [x] **Audio rendered**: `pipeline/minimal_pairs.py --render` → 384 clips, 3 Piper voices
      (anna/berta/imre, dataset CC0), 2 rates, into `media/hvpt/` + `pair_audio`; **342 since
      [ADR-033](DECISIONS.md#adr-033)** (real-word pairs only, clips named by the word)
- [x] `EarView.vue` — two big buttons, green/red, talker badge, running accuracy,
      distinct-talker count; the refusal screen carries have/need and the honest trade-off
- [x] `ListenView.vue` — three-pass protocol as UI state ([R13](research/13-shadowing.md));
      A/B loop; 0.5–1.5× `preservesPitch`; per-sentence dictation with char diff
- [x] Coverage pre-check before playback; warn below ~90% element coverage
- [x] Listening hours: reported seconds (clamped 60 s/report) → `listen_log` → the word-order
      unlock gate reads real hours ([R15](research/15-hungarian-difficulty.md))

---

## 9. Phase 7 — Progress, checkpoints → **v1.0**

- [x] `migrations/00006_progress.sql` — `checkpoint`, `daily_log`
- [x] `internal/stats/streak.go` — streak = **obligations met** (queue cleared + stated
      minutes), not activity; a no-work day breaks it by design; Loop 4's attestation is
      recorded and deliberately ignored ([R14](research/14-motivation.md))
- [x] `internal/stats/checkpoint.go` — M1/M3/M6/M12/M18; measured criteria vs self-attested
      criteria labelled apart; failure locks CEFR levels at/above the gate
      (`gate.locked.checkpoint` in the curriculum) and opens the diagnostic against the three
      documented failure modes, each finding backed by its number
- [x] Per-feature mastery aggregation lives in the overview + client-side grouping (a separate
      heatmap.go was not needed; `srs.Mastery` already carries the per-feature state)
- [x] `ProgressView.vue` — lemma curve vs A1/A2/B1/B2 target lines (labelled as reference
      points); morph heatmap grouped by key; `LocativeGrid.vue` coloured by mastery;
      checkpoint panel with live evidence BEFORE taking; history
- [x] `TodayView.vue` — live loop cards from the daily ledger; obligation bar; streak flame;
      exam countdown (`curriculum.examDate`, [R14](research/14-motivation.md)); **Loop 4
      attestable but un-completable by the app**, reason on screen
- [x] ~~Backup/restore zip; Settings → Data with .apkg/CSV/backup export and restore upload~~ —
      **removed**, see [§10.5](#105-export-and-backup-removed)
- [x] Lemmas enter `learning` when a note is created (never overriding a learner's own
      known/ignored) — the curve and the checkpoints count real state
- [x] README screenshots — six screens from a scratch install, 1280×800, light, RU, in `docs/screenshots/` (2026-09-04)
- [ ] tag `v1.0.0` — left to the maintainer: commits and tags are yours. The blocker named
      here (an unverified `docker build`) is gone — the image builds, runs and self-installs;
      see [§1](#phase-1). What must go into that commit is
      `backend/internal/lang/starter/*.gz`: it is the pack the binary carries, and
      `//go:embed` on a missing file is a compile error, so a clone without it cannot build

---

## 10. After the plan — what v1.0 turned out to need

Five things landed that no phase asked for. Each is here because the app was demonstrably worse
without it, and each is written down so a future session does not "simplify" it back out.

### 10.1 Accounts

The plan had `POST /api/setup` and stopped. That is a single-user app with a login screen: a
household or a tutor with three students had no way to add the second account, and nobody had a
way to change a password.

- [x] `handlers_users.go` — `GET/POST /api/users`, `PATCH/DELETE /api/users/:id` (admin), and
      `POST /api/me/password`, the one account route a learner may call
- [x] `UsersView.vue` + the `/users` route, admin-guarded in the router (**UX only**; the API
      enforces the role independently, and a test walks every verb as a learner)
- [x] `internal/db/users.go` — `ErrLastAdmin`, `ErrDuplicateEmail`. `SetUserRole` and
      `DeleteUser` write first and check the invariant afterwards **inside the same
      transaction**: a count-then-write would let two concurrent demotions each see two admins
      and both succeed. Rolling back a violation cannot. `DeleteUser` also deletes sessions
      explicitly rather than trusting `ON DELETE CASCADE` — a live cookie outliving its account
      is a security hole, not stale data, so it does not hang on a connection-string pragma
- [x] `internal/db/sessions.go` — `DeleteOtherSessions(userID, keep)`: a password change evicts
      every other session and keeps the browser it was typed into

Four rules are pinned by `handlers_users_test.go` because each is a security property rather
than a preference: **no hash ever reaches the wire** (the test greps responses for `$2a$`,
`$2b$`, `passwordHash`, `password_hash`); **the last admin can be neither demoted nor deleted**
(409 — settings and that screen are the only way back, and there is none once they are gone);
**a duplicate email is 409, not the 500** an unmapped `COLLATE NOCASE` constraint would give;
**a password change ends every other session**, which is the whole point of changing one.

### 10.2 The guided course and the hand-off catalogue

The grammar DAG orders the *rules*. The miner scores sentences — once there is a vocabulary to
score against. Between them the app said, in effect, "mine your own sentences", which hands a
beginner a research project instead of a course.

- [x] `langpack/pipeline/course.py`, plus one module per month (`course_m1.py` …
      `course_m6.py`) — 24 units over six months, 853 words. Split by month so the months can be
      written and argued about independently. Dedup keeps the
      first teaching of a word and **prints** the drops; the build **validates** every unit
      against the lemmas and topic codes actually in the pack, so a unit cannot name a word the
      pack cannot look up
- [x] `langpack/pipeline/resources.py` — 29 curated sources, `level` meaning *the level this
      becomes usable at*, not the level it is written at
- [x] `internal/course/course.go` — `Load` + `Resolve`; `migrations/00007_course.sql`
      (`course_progress`)
- [x] `handlers_course.go` — `GET /api/course`, `POST /api/course/:code/enrol`,
      `GET /api/resources`
- [x] `PlanView.vue`, `ResourcesView.vue`, `components/plan/`

Two decisions here are invariants, not preferences:

**A unit is done when its words are in the deck.** There is no completion flag in the schema and
no endpoint to set one. Completion is derived by `Resolve` from `lemma_state`, so the plan screen
cannot drift from the deck — and "I marked it done" cannot mean anything other than "I learned
these words".

**The course ends.** `handedOver` is a state the app is designed to reach: after unit 24 it
stops prescribing vocabulary and points at real sources. [R11](research/11-extensive-reading.md)
measures the effect of reading *volume*, and volume is the one thing a syllabus cannot hand you.
A course that never ends produces a learner who can only read material made for them.

A unit word the pack has no attested sentence for is **skipped and named** in the enrolment
response, never turned into a bare-word card ([R16 §8](research/16-debunked.md)).

### 10.3 The pack is always there

The app booted into "no language pack is installed — download one". That is a build step
wearing a UI: every screen except Settings was dead, and there was no in-app download to click.
Worse, `lang.Detect` trusted the *filename*, so a zero-byte or truncated
`langpack-hu-v1.sqlite` reported as present and left the learner staring at a message telling
them to fetch what appeared to already be there.

- [x] `langpack/pipeline/starter.py` + `make starter` — the starter edition: every
      `morph_feature`, `grammar_topic`, `minimal_pair` and course unit, the top 4,802 lemmas
      with forms/glosses/paradigms, 12,000 sentences whose every word resolves inside that
      lemma set, and the n-gram index in full
- [x] `internal/lang/starter.go` — `//go:embed` of the gzipped pack (~15 MB) plus the ear
      trainer's clips (~0.5 MB); `EnsurePack` extracts beside the target, **proves it opens**,
      then renames into place
- [x] `internal/lang/lang.go` — `Detect` now opens each candidate, newest first, and ignores
      one that will not open
- [x] `SetupView.vue` step 2 — states what is already installed and offers the two genuinely
      optional extras (full pack, speech) as a now/later choice, both defaulting to later.
      Neither has an in-app install path, so "now" means "take me to Settings", never a
      download button this app cannot honour

The 252 MB full pack is an **upgrade that widens the vocabulary**, built with `make langpack`.
Everything the starter omits degrades rather than breaks: a rarer word looks up as unknown,
which the UI already renders honestly.

- [x] Decided against for v1.0 — [ADR-010](DECISIONS.md#adr-010). There is no in-app upgrade
      path: `pack_meta.edition` lets the UI *say* which edition is installed, and the
      instruction is to run `make langpack`. A download button this app cannot honour would be
      worse than the sentence telling the truth

### 10.4 The Makefile

Two toolchains that must run at the same time, with a Vite proxy target written literally in
`vite.config.ts`, is a thing to get wrong by hand.

- [x] `Makefile` — `dev` (both servers, one Ctrl-C), `build`/`run` (the same static binary the
      image ships), `test`/`lint`/`fmt` (what CI gates on), `langpack`/`starter`/`audio`,
      `deps`/`clean`. `make` alone prints them
- [x] `ensure-spa` writes a placeholder `index.html` when there is none, because
      `//go:embed all:dist` does not compile against an absent directory — every fresh clone
- [x] A missing toolchain names itself and says what to install, rather than surfacing as
      "command not found" fifteen lines into a build
- [x] `make clean` never touches `./data`

### 10.5 Export and backup, removed

`internal/export` and its four endpoints (`/api/export/apkg`, `/api/export/csv`,
`/api/export/backup`, `POST /api/import/backup`) are gone, along with the Settings → Data panel
that drove them. Nothing in `backend/`, `web-ui/` or the locale files references them.

What remains true: the data directory is self-contained, so a backup is a `tar` of the volume
and a restore is putting it back. The staged-restore machinery went with the package — there is
no longer any code path that swaps a SQLite file under a live WAL connection, which is what that
machinery existed to prevent.

**This contradicts a decision that is still written down.** [R16 §4](research/16-debunked.md)
and [ADR-003](DECISIONS.md#adr-003) both say `.apkg` export stays first-class "so the learner is
never locked in", and the README used to promise it. No ADR records the reversal.

- [x] [ADR-011](DECISIONS.md#adr-011) supersedes it: lock-in is answered by the data directory
      being yours, not by a file Anki can read — an `.apkg` cannot carry the morph state space,
      which is the whole thesis, so exporting one would hand the learner a vocabulary list and
      call it their progress

---

### 10.6 Review sets, and a card that shows its own memory

A learner with 70 due cards was handed 70 cards. The observation was that it is exhausting, and
that nothing on the card says how the word is doing. [R18](research/18-sessions-and-progress.md)
and [ADR-026](DECISIONS.md#adr-026).

- [x] `internal/srs/memory.go` — `Memory`: stage (Anki's young/mature at 21 days, called a
      convention), retrievability from the library, interval and stability, tallies and the
      last eight outcomes from `review_log`, and `Preview` for what each grade would schedule.
      Attached to every queue item; on practice items without the preview. `memory_test.go`
- [x] `internal/srs/queue.go` — `?order=due|random`; random is `shuffleDayStable`, a per-card
      hash of (user, local day, card), applied after the day cap and before `spreadSentences`.
      `queue_order_test.go` pins that it is a permutation and holds still for the day
- [x] `review_log.answer` is finally written (the column existed since `00002`): what the
      learner typed on a productive card, which the tutor quotes back
- [x] `web-ui/src/lib/sets.ts` — per-device set size (10/15/20/30, default 15) and order;
      `stores/review.ts` deals the head of the served queue as a set, fetches the list again
      when a set finishes (a miss comes back through the server once its `nextDue` has elapsed —
      the in-set retry and its countdown were removed on 2026-09-03), and accounts each set
      (answered, right, missed — listed with their answers, matured, minutes)
- [x] `ReviewView.vue` — set bar, size and order chips, `MemoryMeter.vue` (stage, marks, tally,
      interval change; the recall percentage came off the card on 2026-09-03), interval hints
      under the grades, «ответ верный → через ~3 дн» on typed cards, the verdict in the answer
      box's place, `SetSummary.vue` between sets with `space` for the next one, `MissedList.vue`
      on it and on the done screen; `setComplete` sound
- [x] `TodayView.vue` prints the day in sets from the same size
- [x] `ReviewStartView.vue` at `/review` — the piles with their Breakdowns, the set chips and
      the practice block; the card moved to `/review/session` and starts from its query
      (`?scope=`, `?practice=`); `ScopeTabs.vue` deleted ([ADR-028](DECISIONS.md#adr-028))

### 10.7 The grammar tutor

The grammar screen could state a rule and drill it, and could not answer a question about it.
[R19](research/19-grounded-tutor.md) and [ADR-027](DECISIONS.md#adr-027); the design in
[`../docs/TUTOR.md`](../docs/TUTOR.md).

- [x] `internal/llm` — `Stream` beside `Chat`, OpenAI streaming wire format
- [x] `internal/tutor` — the embedded corpus (`kb/`), section loader, keyword-and-topic
      retrieval, the prompt with its budgets and the system texts in both languages, the
      backtick dictionary check, the thread store; `tutor_test.go`
- [x] `internal/tutor/kb/` — 21 generated files from Wikipedia, Wiktionary and the
      public-domain FSI course (`langpack/pipeline/tutor_kb.py`, `make tutor-kb`), 14
      hand-written reference notes, `LICENSES.md`
- [x] `migrations/00015_tutor.sql`, `handlers_tutor.go` — `GET/POST/DELETE /api/tutor/:topic`,
      the answer as server-sent events, one flight per learner; `handlers_tutor_test.go` pins
      grounding, labelling, language fallback and that nothing reaches a note or a state space
- [x] `TutorPanel.vue` under the lesson on the grammar screen, streaming, with the model named
      under every answer and unattested words listed; `api/tutor.ts` reads the stream
- [x] **The tutor is a chat** ([ADR-029](DECISIONS.md#adr-029), 2026-09-03): `tutor_thread`
      (migration `00016`, existing threads migrated one-to-one), `tutor.InferTopic` finds the
      lesson for a free chat's question and the answer prints it, prompt v2 (Latin orthography
      always, examples always, the no-lesson case stated), `AiTeacherView.vue` as a chat list
      with new/delete and the topic as an option; a failed thread load now says so instead of
      rendering nothing, and the `done` event's `unattested` is never `null` — the actual cause
      of the first learner's "blank until reload" panel
- [x] `services.llm` requires a model name and the settings row verifies it against
      `GET /v1/models`; local servers are sent `reasoning_effort: none` (2026-09-03)
- [x] **Translation as explanation** ([ADR-030](DECISIONS.md#adr-030), 2026-09-03): prompt v3 —
      the language rule names the explanation, not the material («ТОЛЬКО ПО-РУССКИ» had a 1.7B
      model hand the Russian sentence back), a translation request answers with the sentence,
      its literal rendering and a word-by-word breakdown; `max_tokens` 1,000 and a 150 s
      timeout to fit it; a budget cut is marked `interrupted` (`llm.ErrTruncated`); a
      backticked sentence is dictionary-checked word by word; the page names a model whose
      tag says it is under 7B
- [x] **A chat's controls** ([ADR-031](DECISIONS.md#adr-031), 2026-09-03): any text that
      arrived is `done` + `interrupted`, `error` only when nothing arrived (a budget cut had
      shown «ничего не изменилось» under a stored partial); `POST /api/tutor/:id/stop` cancels
      the flight's context; `replace` on a question truncates and re-asks
      (`Store.TruncateFrom`); the context meter — `stream_options.include_usage` +
      Ollama's `/api/ps`, stored per answer (migration `00017`), a bar and a sentence under
      the composer, a warning when the window was full; the page is the chat — no page
      title, full viewport height, reading column, composer at the bottom with the notice,
      the meter and the sources under it
- [x] **The tutor from the review card** ([ADR-032](DECISIONS.md#adr-032), 2026-09-03):
      «Спросить учителя», key `a`, answer phase only; `TutorAside.vue` in the inspector's side
      column with the question pre-written from the card (`seed` on `TutorConversation`),
      one chat per sitting, the composer's keys kept out of the card's handler
- [x] **The tutor reads the card** ([ADR-034](DECISIONS.md#adr-034), 2026-09-04): `POST
      /api/tutor/:id {card}` — nine server-composed templates (`internal/tutor/card.go`) with
      answer shapes, a CARD block after the lesson (prompt v4), grounding from `targetFeats`,
      both phases with the answer withheld before it and the question counted as a hint
      (`usedHint`), the day's review chat (`kind = review`, migration `00018`), the button top
      right, «засчитается как «Трудно»» on the capped grades, the question limit in characters
- [x] **«Перевести» everywhere** ([ADR-035](DECISIONS.md#adr-035), 2026-09-04): one gloss
      picker (`lib/gloss.ts`), the crib labels an other-language pack line and offers the
      translator under it, keyed to the card; the word breakdown's examples go through the
      same crib; list rows mark «(англ.)»
- [x] **Second round of the first learner's feedback** (2026-09-03): the memory chips speak
      Russian («закрепляю», not «молодая»); grade previews read from `preview.at`, not the
      clock (the 10-minute step had shown «~7 мин»); the key strip and the milestone centre on
      the page (`--rail`); the tutor log follows only while at the bottom; a prompt echo is cut
      (`tutor.EchoAt`, `cutReason: echo`); the ear trainer plays words only
      ([ADR-033](DECISIONS.md#adr-033)) — `unattested` gate, word-keyed clip names, audio rows
      remapped on refresh, starter pack and media rebuilt
- [ ] A Russian reference corpus beside the English one — the known asymmetry in TUTOR.md §7
- [ ] Cyrillic-typed suffixes («ем») as an inference signal — today the prompt answers about
      `-em` but the inference sees no suffix; a transliteration table for the 1:1 letters would
      close it

### The look, and the polish before the tag (2026-09-04, ADR-036 → ADR-038)

The first learner's second verdict was on the look — «шаблонный, сгенеренный нейронкой» —
and on what a release has to survive. Three ADRs:

- **ADR-036 — two faces.** Literata Variable for page titles, the wordmark and every Hungarian
  string, marked `lang="hu"` and set by one base rule in `assets/tailwind.css`; Inter stays the
  instrument. Self-hosted subsets only (`assets/fonts.css`), the teal favicon with a serif «ő».
  Fraunces was the first choice and has no Cyrillic — the Russian headings decided it.
- **ADR-037 — five line drawings** in the manner of Kalocsa and Matyó embroidery,
  `currentColor`, decorative, on the empty states, the done screen, the 404 and under the
  wordmark at the two doors. Never a metric.
- **ADR-038 — ready for a stranger.** `describeError` for every failure, a global handler, the
  theme applied before the bundle, docs corrected (offline is *planned*), the locale and docs
  checks runnable locally and inside `make test` with `-race`, the pipeline's venv found by the
  Makefile, `make refresh`, the password floor from bootstrap.

Still on the list: README screenshots, `CHANGELOG.md`, the tag.

### Two more from the first learner (2026-09-04, ADR-034 amended, ADR-039)

- **The review tutor is one chat per WORD, not per day** (ADR-034, amended). The panel opened
  on the previous word's conversation and carried that answer into the model's window on every
  later question. `tutor_thread.note_id` (migration `00019`), the word as the chat's name in
  the rail, a generation counter in `useTutorThread` so a still-streaming answer cannot write
  into the next word's log, and the standing honesty paragraph off that panel — the per-answer
  labels are what defend it.
- **ADR-039 — the card says what it takes for a word to stop coming back.** «что это значит?»
  on the memory row: the stage's meaning, what a right answer and a miss schedule next, that a
  card never ends but returns ever more rarely, that the tally is the log and not points, and
  that «В корзину» is the one control that removes a word. No bar and no denominator — there
  is no such number.

## 11. Phase 8–10 — v1.1 onward

**Phase 8 · Speech (T4).** `services/tts` (Piper), `services/asr` (faster-whisper);
`internal/speech` provider abstraction + content-addressed cache; recording UI;
**explicit segmental feedback** naming the phoneme and enqueuing the HVPT contrast
([R8](research/08-capt-asr.md)); confidence gating because Hungarian WER ≈ 11%.

**Landed ahead of these: sentence translation.** `services/mt` (NLLB-200-distilled-600M under
CTranslate2), `internal/mt`, `POST /api/translate`, and the reader's `t` key. Not part of Phase 9
and not the `llm` slot: a dedicated 600M translation model beats a 7–8B chat model at
Hungarian→Russian, and its output is a labelled crib rather than content
([ADR-014](DECISIONS.md#adr-014)).

**Phase 9 · LLM tutor (T4 roleplay, T7).** `services/nlp` (HuSpaCy); `internal/llm`;
`internal/corpus/attest.go` — **the attestation guardrail**; writing correction in
prompt-then-reveal style ([R9](research/09-corrective-feedback.md)); voice roleplay
([R10](research/10-llm-tutors.md)).

**Phase 10 · v1.2 / v1.3.** Subtitle mining at scale, EPUB library, FSRS optimizer sidecar,
ECL mock exams, multi-user polish.

---

## 12. Standing rules

1. **Behaviour change → doc change in the same commit.** The `docs/` tree is the contract.
2. **Pedagogical code cites its note.** `// [R5] ... see .claude/research/05-interleaving.md`
3. **Both locales, always.** A PR adding an `en` key without `ru` fails review.
4. **Never edit an applied migration.** Add a new numbered file.
5. **`config.yaml` is mutated only via `Server.updateConfig`** (clone → validate → atomic save
   → swap). Secrets go in SQLite, never in YAML.
6. **The app must remain fully usable with every sidecar stopped.** Verified at each release.
7. **No feature ships that we cannot justify.** If the evidence is weak, say so in the UI.
