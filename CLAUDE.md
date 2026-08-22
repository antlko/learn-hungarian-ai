# CLAUDE.md

Guidance for Claude Code (claude.ai/code) working in this repository.

## Project

**tanulom** — a self-hosted, research-grounded Hungarian trainer. A **Go backend** (`backend/`)
serves a **Vue 3 SPA** (`web-ui/`) plus a JSON API. In production the built SPA is embedded into
a single static binary (one Docker container). Optional sidecars (`services/`) add NLP, TTS and
ASR; **the core is complete without them.**

**The thesis:** every other tool treats a wordform as an atom. For an agglutinative language
that is a category error. This app tracks **two independent FSRS state spaces — lemmas and
morphological features** — and derives `n+1` selection, a per-suffix weakness map and targeted
generation from them. If a change breaks that separation, it is the wrong change.

## Read this first

This file is the **index**: commands, invariants, gotchas. Depth lives in `docs/` and
`.claude/` — follow the pointer instead of re-deriving behaviour from the code.

| Doc | Read it when you need |
| --- | --- |
| **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** | **How it behaves**: the two state spaces, data ownership, package graph, key algorithms, request flows, "where to make a change" map |
| **[docs/PEDAGOGY.md](docs/PEDAGOGY.md)** | **Why any learning feature is the way it is** — feature→citation map, and the six documented divergences from the source brief |
| [docs/LANGPACK.md](docs/LANGPACK.md) | Pack schema, build pipeline, sources + licences, versioning, quality gates |
| [docs/UX.md](docs/UX.md) | Screen-by-screen, keyboard map, colour semantics, a11y, offline |
| [docs/API.md](docs/API.md) | Every endpoint + object shape, with implementation status |
| [docs/CONFIGURATION.md](docs/CONFIGURATION.md) | Every `config.yaml` field |
| [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) | Running both dev servers, building the single binary, testing conventions |
| [docs/SELFHOSTING.md](docs/SELFHOSTING.md) | Deployment, reverse proxy, backup, sizing |
| [.claude/PLAN.md](.claude/PLAN.md) | The granular phase plan and its checklists |
| [.claude/DECISIONS.md](.claude/DECISIONS.md) | ADRs — what was considered and why this won |
| [.claude/research/](.claude/research/README.md) | **18 evidence notes.** Every pedagogical claim traces here |
| [.claude/prompts/](.claude/prompts/README.md) | Every LLM prompt template, versioned, with failure modes |

**When you change behaviour, update the matching doc in the same change** — these files are the
contract future sessions rely on.

## Working directories

Two toolchains: run `npm` from `web-ui/`, run `go` from `backend/`. In dev they run side by side
(Vite proxies `/api` and `/media` to Go on `:8012`, per `web-ui/vite.config.ts`).

## Commands

**From the repo root, the `Makefile` drives both toolchains** — `make` alone lists every target.
`make dev` runs the Go API and Vite together, `make test` is exactly what CI gates on,
`make build` produces the static binary, `make langpack` / `make starter` rebuild the packs.
Prefer it over hand-running the halves; it knows the things that are easy to get wrong (the
`:8012` proxy target, the placeholder `dist/` that `//go:embed all:dist` needs on a fresh clone).
The port is written down in four places — `internal/config` (the source of truth), `DEV_ADDR`,
`vite.config.ts` and the `Dockerfile`. They drifted once and broke `make dev` and
`docker compose up` at the same time; change one and you change all four.

Underneath: **frontend** (from `web-ui/`) `npm run dev` · `npm run type-check` · `npm run lint`
(oxlint→eslint, both `--fix`) · `npm run build`; **backend** (from `backend/`)
`go run ./cmd/tanulom --data-dir ../data` · `go vet ./...` · `go test ./...` · `go build ./...`.
**Single binary**: build SPA → `cp -r web-ui/dist backend/internal/web/dist` →
`CGO_ENABLED=0 go build ./cmd/tanulom` (`make build` and the `Dockerfile` both automate this).
**No frontend test framework is wired** — verify UI changes by running the dev servers; don't
invent an `npm test`.

## Invariants — breaking these causes real bugs

- **The two state spaces stay separate.** `lemma_state` answers "do you know this word";
  `morph_state` answers "do you know this grammatical fact". A single cloze answer emits
  **1 card review + N feature reviews**, and **only features actually required by the item are
  graded** (derived from the blanked span's `feats`, not from every token). Grading unrequired
  features turns the heatmap into noise. `internal/srs/emit.go`.
- **We never write scheduling maths.** `internal/srs` wraps `go-fsrs/v4` (FSRS-6) unmodified.
  See [ADR-003](.claude/DECISIONS.md#adr-003).
- **The language pack is read-only.** `ATTACH`ed, never written. Learner corrections go to
  `morph_correction` in the user DB and apply as an overlay at lookup time.
- **Morphology never calls Python at runtime.** Lookup ladder: exact `wordform` → frequency prior
  (flagged `uncertain`) → suffix fallback (flagged `derived`) → optional HuSpaCy sidecar. See
  [ADR-002](.claude/DECISIONS.md#adr-002).
- **`config.yaml` is mutated only via `Server.updateConfig`** (clone → validate → atomic save →
  swap under lock). Add new `Config` fields to `config/clone.go` too. Never touch `s.cfg`
  directly.
- **Secrets never leave the server and never enter YAML.** Stored in SQLite, stripped from DTOs
  (presence exposed as `secrets: {k: true}`), preserved on `PUT` when blank or omitted.
- **Route guards are UX only** — the API enforces roles independently.
- **The app must remain fully usable with every sidecar stopped.** Verified at each release.
- **Interleaving is gated behind a competence floor.** Blocked practice until rolling accuracy
  ≥ 80%, then interleaved, with hysteresis on fallback. This contradicts the source brief on
  purpose — see [R5](.claude/research/05-interleaving.md). Do not "fix" it back.
  The gate in `internal/drill/schedule.go` is a **pure function** of the learner's counters, and
  it exports the reason for its verdict as an i18n key. Keep it pure: the UI's claim that it is
  showing you the rule the server applied is only true while that holds.
- **A drill's correct answer must be attested in the pack.** Distractors may be constructed —
  a wrong allomorph is not a word — but the form marked right has to be one the language has.
  Resolve forms through `internal/morph/forms.go`. **Never read the pack's `paradigm` table for
  anything that must be right**: it is generator-only and says `házan` where the dictionary
  attests `házon`.
- **An item grades only the features it required.** Pure harmony practice grades
  `Skill=VowelHarmony` and not the case, because the learner was never asked what the case
  means. A case choice between forms of one noun does not grade harmony, because every option
  shares the stem's harmony class.
- **The expected answer never leaves the server.** Items are held in `internal/drill/pending.go`
  between serve and answer — [ADR-008](.claude/DECISIONS.md#adr-008).
- **`COLLATE NOCASE` defeats the pack's indexes.** `idx_wordform_form` is BINARY-collated, so
  `WHERE form = ? COLLATE NOCASE` scans two million rows. Use `caseVariants` +
  `IN (?,?,?)` — `internal/morph/casefold.go`.
- **No HVPT contrast is ever marked complete.** Effects decay; contrasts get FSRS maintenance
  forever, and the adaptive selection weight has a positive floor. A session below
  `hvpt.minTalkers` distinct voices is **refused with the numbers**, never silently degraded.
  See [R7](.claude/research/07-hvpt.md).
- **The review log replays to stored state bit-for-bit** — fuzz is seeded, so this includes due
  dates. Undo is delete-and-replay on that property (`internal/srs/replay.go`); do not add
  stored-previous-state bookkeeping beside it, and keep `review_time` at millisecond precision.
- **Hours and streaks accumulate; they are never claimed.** Listening progress is clamped to
  60 s per report; the streak counts obligations met (queue cleared + stated minutes), and
  Loop 4's attestation is recorded but deliberately ignored by it. [R14]
- **A pack is always present — never write a "no pack" dead end again.** The binary embeds a
  starter pack (`internal/lang/starter.go`) and `lang.EnsurePack` installs it on first boot;
  `lang.Detect` **verifies that a candidate opens** rather than trusting the filename, because a
  zero-byte download used to report as present and leave every language feature dead behind a
  message telling the user to fetch what was already on disk. A pack-dependent route answers
  503 as a genuine failure path, not as the normal first-run state; UI copy must not send the
  learner off to download anything, and `/api/bootstrap` reporting `packPresent: false` means
  something broke. The full pack is an **upgrade**, built with `make langpack`.
  **`backend/internal/lang/starter/*.gz` is committed, not generated** — `//go:embed` on a
  missing file is a compile error, so gitignoring it would mean a fresh clone cannot build.
  Change the pipeline and you must run `make starter` and commit the result, or the binary
  ships last month's course. CI checks this ([ADR-010](.claude/DECISIONS.md#adr-010)).
- **A course unit's completion is derived from the deck, never stored.** `course.Resolve` calls
  a unit `done` when every one of its words is in `lemma_state` as `learning`/`known`.
  `course_progress` records enrolment only — do not add a `completed` flag, an endpoint that
  sets one, or a client-side override. The moment completion is storable, the plan screen can
  disagree with the deck, and "done" stops meaning "learned".
- **No XP, no levels, no leagues, no leaderboards, no gems, no lootboxes.** Gamification's
  effect on *competence* is null, and extrinsic point economies are the part most likely to
  crowd out the real goal — [R14](.claude/research/14-motivation.md),
  [R16 §3](.claude/research/16-debunked.md). The permitted motivational surface is exactly:
  the streak (obligations met), the lemma curve, the morph heatmap, checkpoint pass/fail, and
  the exam countdown. Every one of those is a **competence** signal with something measured
  behind it. A request for "a bit more fun" is answered inside that set.
- **No LLM-generated Hungarian becomes a card without passing `internal/corpus/attest.go`** —
  the guardrail lands with the LLM in Phase 9; `internal/corpus` is an empty directory today and
  nothing generates Hungarian yet.

## Conventions that bite

- **Pedagogical code cites its note:** `// [R5] ... see .claude/research/05-interleaving.md`.
  **No design decision may cite a paper that has no note**, and no note exists without a
  "What this changes in our code" section.
- **Both locales, always.** A change adding an `en` key without its `ru` counterpart is
  incomplete. [ADR-004](.claude/DECISIONS.md#adr-004).
- **Icons: `@lucide/vue`**, NOT `lucide-vue-next` (deprecated). Some names changed in v1
  (`Home`→`House`); verify via type-check.
- **Tailwind v4 resets buttons to `cursor: default`.** Reuse `buttonVariants`, `TabsTrigger`,
  `SelectTrigger`, `DropdownMenuItem` rather than hand-rolling a bare `<button>`.
- **UI = shadcn-vue** (Reka UI + Tailwind v4). Primitives in `src/components/ui` are
  intentionally single-word; `eslint.config.ts` disables `vue/multi-word-component-names` there.
- **Migrations**: add `internal/db/migrations/NNNNN_name.sql` with `-- +goose Up`/`Down`, dialect
  `sqlite3`. **Never edit an applied migration.** They are embedded and run inside `db.Open` —
  there is no migrate command and distroless has no shell to run one.
- **Fiber v3 handlers** return `fiber.NewError(code, msg)` (central `errorHandler` renders
  `{"error": msg}`); success via `c.JSON`.
- **Keep `CGO_ENABLED=0`** — `modernc.org/sqlite` is pure Go and that is what makes the binary
  static and cross-compilable to arm64. Do not add a C-dependent library.
- **Frontend types in `src/types` mirror `backend/internal/api/dto.go`** (camelCase). Keep in sync.
- **Defaults live in one place** — exported constants in `internal/config`, so API fallbacks
  cannot drift from what a fresh config gets. Don't re-hardcode a default at a use site.
- **`normalize()` fills zero fields only.** Changing a default affects new installs and unset
  fields; never silently rewrite a user's stored values.
- **Logging** is slog JSON to stdout; prefer `slog.InfoContext(c.Context(), …)` with key/value
  attrs. Level via `TANULOM_LOG_LEVEL`.

## Honesty rules

This project's credibility rests on not overclaiming. These are product requirements, not style:

- Uncertain morphological analyses, fallback glosses, unattested LLM output and low-confidence
  ASR are **labelled in the UI**.
- **"We have not built this" is a supported outcome.** A topic with no drill generator resolves
  to the `explained` phase and says so; a topic the pack cannot supply attested material for
  reports `noMaterial` rather than inventing answers. Thirteen of twenty-eight topics produce
  drills on hu-v1, and the app does not pretend otherwise.
- **An error a drill cannot fix is routed, not drilled.** A vowel-length or rounding mistake is
  a perception problem; `internal/drill/from_error.go` returns `drill.remedy.ear` and the UI
  says that typing practice will not treat it.
- Known limitations are stated in the product, not only in docs: FSRS's efficiency figure is a
  simulation; per-user FSRS optimisation is absent in v1.0; coverage thresholds are
  English-derived; Hungarian ASR has ~11% WER; only 3 synthetic voices exist for HVPT.
- **The guided course ends, and says so.** Twenty-four units, then `handedOver` and a catalogue
  of real Hungarian. Do not extend it into an endless drip — a course that never ends produces a
  learner who can only read material made for them [R11].
- **The app has no export.** `internal/export` and `/api/export/*` were removed; lock-in is
  answered by the data directory being the learner's, not by a file format. Do not document
  endpoints that do not exist. [ADR-011](.claude/DECISIONS.md#adr-011) supersedes
  [R16 §4](.claude/research/16-debunked.md)'s "keep `.apkg` export first-class": the format
  cannot carry the morph state space, so exporting one would hand the learner a vocabulary
  list and call it their progress.
- **The app cannot replace a teacher.** Loop 4 is un-completable by the app, on purpose.
