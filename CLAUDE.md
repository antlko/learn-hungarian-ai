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
(Vite proxies `/api` and `/media` to Go on `:8080`, per `web-ui/vite.config.ts`).

## Commands

**Frontend** (from `web-ui/`): `npm run dev` · `npm run type-check` · `npm run lint`
(oxlint→eslint, both `--fix`) · `npm run build`.
**Backend** (from `backend/`): `go run ./cmd/tanulom --data-dir ../data` · `go vet ./...` ·
`go test ./...` · `go build ./...`.
**Single binary**: build SPA → `cp -r web-ui/dist backend/internal/web/dist` →
`CGO_ENABLED=0 go build ./cmd/tanulom` (the `Dockerfile` automates this).
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
- **No HVPT contrast is ever marked complete.** Effects decay; contrasts get FSRS maintenance
  forever. See [R7](.claude/research/07-hvpt.md).
- **No LLM-generated Hungarian becomes a card without passing `internal/corpus/attest.go`.**

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
- Known limitations are stated in the product, not only in docs: FSRS's efficiency figure is a
  simulation; per-user FSRS optimisation is absent in v1.0; coverage thresholds are
  English-derived; Hungarian ASR has ~11% WER; only 3 synthetic voices exist for HVPT.
- **The app cannot replace a teacher.** Loop 4 is un-completable by the app, on purpose.
