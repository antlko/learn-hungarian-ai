# CLAUDE.md

Index for Claude Code. **Keep this file small** — it is loaded into every session and every
subagent. Depth lives in `docs/` and `.claude/`; fetch it on demand (see "Context budget").

## Project

**tanulom** — a self-hosted, research-grounded Hungarian trainer. **Go backend** (`backend/`) serves a
**Vue 3 SPA** (`web-ui/`) plus a JSON API; in production the SPA is embedded into one static binary
(one Docker container). Optional sidecars (`services/`: TTS, ASR, MT) are never required.

**Thesis:** a wordform is not an atom in an agglutinative language. The app tracks **two independent
FSRS state spaces — lemmas and morphological features** — and derives `n+1` selection, a per-suffix
weakness map and targeted generation from them. A change that mixes the two spaces is the wrong change.

## Context budget — read this before reading anything else

The docs total ~700 KB (≈175k tokens). Reading them whole is the failure mode this section prevents.

- **Never `Read` a whole doc over ~15 KB.** The big ones: `docs/UX.md` 166 KB, `.claude/DECISIONS.md`
  198 KB, `docs/API.md` 70 KB, `.claude/PLAN.md` 48 KB, `docs/ARCHITECTURE.md` 45 KB, `docs/LANGPACK.md` 45 KB,
  `docs/INVARIANTS.md` 43 KB, `docs/TUTOR.md` 34 KB.
- **Locate, then read a slice:** `grep -n '^## ' <doc>` for the outline, `grep -n '<keyword>' <doc>`, then
  `Read` with `offset`/`limit` (~40–120 lines). An ADR: `grep -n '^## ADR-0NN' .claude/DECISIONS.md`.
- **Ask the topic map first** (below) instead of guessing which doc.
- **Wide exploration or "which doc says X?" goes to `researcher`** (cheap, read-only, returns a short
  answer) — see the Workflow section. Do not pull ten files into the main context to answer one question.
- The rules in "Invariants digest" are enough to avoid most mistakes; open `docs/INVARIANTS.md` only for the
  one rule you are about to touch.

### Topic map — which slice to open

| You are changing… | Open (a slice, not the file) |
| --- | --- |
| Scheduling, FSRS, review queue, undo, burial, sets | `docs/ARCHITECTURE.md` §srs/review; `docs/INVARIANTS.md` rules on review/replay/burial |
| Drills, options, grading, `produce`/`cloze` | `docs/ARCHITECTURE.md` drill; ADR-008/017/060 |
| A screen's look, copy, keys, a11y, colours | `docs/UX.md` — only that screen's `## N.` section (outline: Shell 2, Today 3, Review 4, Read 5, Listen 6, Ear 7, Grammar 8, Progress 9, Settings 10, colours 11, a11y 12, design system 14) |
| An endpoint or DTO | `docs/API.md` section for it; `backend/internal/api/dto.go`; mirror in `web-ui/src/types` |
| Config field | `docs/CONFIGURATION.md`; `backend/internal/config` |
| Language pack, build pipeline, sources | `docs/LANGPACK.md` |
| Grammar tutor / LLM prompts | `docs/TUTOR.md`, `.claude/prompts/` |
| Why a feature is the way it is | `docs/PEDAGOGY.md`, then the cited `.claude/research/NN-*.md` |
| "Was this decided before?" | `grep -n '<term>' .claude/DECISIONS.md` then read that ADR |
| What the app does, plainly | `docs/FEATURES.md` |
| Running / testing / deploying | `docs/DEVELOPMENT.md`, `docs/SELFHOSTING.md` |

**When you change behaviour, update the matching doc in the same change** — and edit the *section*, do not
re-read the file to "check consistency". A new design decision is an ADR in `.claude/DECISIONS.md`; cite
research as `[R5]` (note must exist; `make docs-check` enforces it).

## Working directories and commands

Run `npm` from `web-ui/`, `go` from `backend/`. Dev: Vite proxies `/api` and `/media` to Go on `:8012`.
**Prefer the `Makefile`** (`make` lists targets): `make dev` (Go + Vite), `make test` (the CI gate, takes
minutes), `make build` (static binary), `make langpack` / `make starter`, `make docs-check`.
The API port is written in four places (`internal/config` is the source of truth, `DEV_ADDR`,
`vite.config.ts`, `Dockerfile`) — change one, change all four.

Underneath: frontend `npm run dev|type-check|lint|build`; backend `go run ./cmd/tanulom --data-dir ../data`,
`go vet ./...`, `go test ./...`. **No frontend test framework exists** — verify UI by running it (below).

Fast loops:

```bash
cd backend && go test ./internal/srs/ -run TestSpreadSentences -v   # one test / prefix
cd backend && go test ./internal/tutor/ -count=1                    # one package
```

`make` Go targets run `ensure-spa` first (`//go:embed all:dist` needs `backend/internal/web/dist`); a
hand-run `go test ./internal/api/` on a fresh clone fails with `pattern all:dist: no matching files`
until any `make` Go target has written the placeholder.

`make test` can fail for a reason that is not yours: `internal/drill` real-pack tests hit Go's 10-min
package timeout under `-race`. Rerun `make test GO_TEST_FLAGS="-race -count=1 -timeout 25m"` before
concluding the backend is broken. `make docs-check` enforces: `[R…]` citations resolve, research notes
state code impact, `ru.json`/`en.json` have the same keys.

## Workflow: agents and verification (MANDATORY)

Full protocol: the project skill **`/tanulom-workflow`** (`.claude/skills/tanulom-workflow/SKILL.md`).
Load it for any non-trivial task. Summary:

- **The main conversation is the orchestrator.** It keeps requirements and decisions and stays small.
  Project agents live in `.claude/agents/`: `researcher` (Haiku, read-only: docs, code search, "where/what"),
  `adviser` (Opus: hard decisions and plans), `developer` (Sonnet: implements + runs tests),
  `reviewer` (Sonnet, read-only: independent verdict on the real diff).
- **Always use them when they exist** for anything beyond a 1–3-file obvious fix: doc lookups and
  codebase surveys → `researcher`; ambiguous/architectural → `adviser`; implementation → `developer`;
  then `reviewer` for risky areas (listed in the skill). Do not read big docs yourself to brief an agent;
  pass file paths and section names, and take back short reports.
- **Every UI change is tested manually in Claude Code's built-in browser** (`mcp__Claude_Browser__*`,
  `preview_start` with the `tanulom` config in `.claude/launch.json`) before it is called done: load
  the screen, exercise the change, check console/network errors, screenshot, and check both locales,
  light and dark, and mobile width when layout is touched. "Type-check passes" is not verification.
  Use a scratch data dir — never the real `data/` (see memory: scratch-verification).

## Invariants digest — breaking these causes real bugs

One line each; **full text, reasons and ADR links: `docs/INVARIANTS.md`** (grep the bold phrase).

**Scheduling and data**
- Two state spaces stay separate (`lemma_state` vs `morph_state`); one cloze answer = 1 card review + N
  feature reviews, **only for features the item required** (`internal/srs/emit.go`).
- Never write scheduling maths: `internal/srs` wraps `go-fsrs/v4` unmodified (ADR-003).
- Review log replays to stored state bit-for-bit (seeded fuzz; ms `review_time`; keep `remaining_steps`);
  undo = delete-and-replay. Bit-exactness holds per CPU arch.
- A practice answer (`/api/practice/*`) is a real review via `Engine.Answer` (ADR-064), except words in «Знаю»
  and new cards (`practice_log` only, `counted:false`); undo also deletes its `practice_log` row.
- Word progress 0–100 is display-only (`srs.progressOf`: stage band + stability; marks 20/70; 180 days = 100;
  ADR-070). It never feeds scheduling and is never called a score.
- «Знаю» = `note.trashed_at` (+ lemma `known`), never a delete; `scope=trash` refused by the review queue.
- Every review number a learner sees comes from `srs.Breakdowns`; never show a raw card count.
- Review scope is a filter, never a second scheduler; the daily new budget stays global.
- Review sets are client-side (`stores/review.ts`); the server never hears of them. Refresh after each set.
- Sibling burial spans the day; a session never asks one sentence twice in a row (`spreadSentences`).
- Course unit completion is derived (`course.Resolve`), never stored; "next" follows enrolment.
- Language pack is read-only (`ATTACH`); corrections go to `morph_correction` overlay. A pack is always
  present (embedded starter, `lang.EnsurePack`); `backend/internal/lang/starter/*.gz` is committed.
- Migrations: new `NNNNN_name.sql`, never edit an applied one. Keep `CGO_ENABLED=0`.
- `config.yaml` mutated only via `Server.updateConfig`; new fields also go in `config/clone.go`.
  Defaults live as exported constants in `internal/config`. `normalize()` fills zero fields only.
- Secrets live in SQLite, are stripped from DTOs, never enter YAML.

**Drills and grading**
- A drill's correct answer must be attested in the pack; never read the pack's `paradigm` table for anything
  that must be right — resolve via `internal/morph/forms.go`.
- Item grades only the features it required. `produce` grades the DICTIONARY form, `cloze` the sentence form
  (`expectedAnswer` is the one place). The client never derives the expected answer (`expectedFor`).
- Options built server-side from attested forms (`internal/api/options.go`); tap = typed answer, max Good.
- Interleaving gated behind an 80% competence floor; `drill/schedule.go` gate stays a pure function.
- Curriculum order is ADVICE, not a lock (ADR-013); only a failed checkpoint and listening volume hard-lock.
- A cloze needs a span, not features. Expected answer never leaves the server (`drill/pending.go`).

**Language and text**
- Morphology never calls Python at runtime (lookup ladder: exact → prior → suffix; ADR-069).
- Offsets are UTF-8 bytes on the server, UTF-16 in the browser — never slice client-side with them.
- A line break ends a sentence (`morph.SplitSentences` needs the source text).
- `COLLATE NOCASE` defeats pack indexes — use `caseVariants` + `IN (...)`.
- Corpus lemma id ≠ dictionary lemma: fall back to surface-form search.
- Verb forms generated for recognition, never production.

**LLM, MT, sidecars**
- Tutor explains, mints nothing (no path to `note`/`card`/`user_gloss`/state); labels model + lesson; asking
  never changes a schedule; never write `ONLY` into its prompt; read `docs/TUTOR.md` before touching it.
- No LLM Hungarian becomes a card without `internal/corpus/attest.go` — except learner-reviewed, labelled
  AI sentences for unillustrated course words (ADR-073); AI dictionary entry is a separate
  path (ADR-042). Reading sets are the one labelled AI-written Hungarian in the pack
  (`source='authored'`: «Рассказ» ADR-074, «Диалог» ADR-066), filtered out everywhere else.
- MT is display-only and always labelled with its model; `lib/gloss.ts` picks gloss language.
- App fully usable with every sidecar stopped; managed components never block boot; endpoints never stored.
- `services.llm` needs a model name (`llm.New` → `ErrNoModel`).

**Motivation and honesty**
- No XP, levels, leagues, gems, lootboxes; allowed surface = streak, lemma curve, heatmap, checkpoints,
  exam countdown, estimated level. Hours/streaks accumulate, never claimed. Drawings are chrome, not metrics.
- Uncertain analyses, fallback glosses, unattested output, low-confidence ASR are labelled in the UI;
  "we have not built this" is a supported outcome; the guided course is finite (24+20+20 core units).
- Export is a full dump, never `.apkg`; import replaces for the acting user only.
- HVPT contrasts are never "complete"; every minimal-pair member must be a dictionary word.

**Frontend conventions**
- Learner-facing copy has no jargon (no "corpus", slugs, feature ids). Feature labels come from the pack
  via `stores/features.ts`. Both locales (`ru` + `en`) for every key.
- Every learner-visible failure goes through `lib/errors.ts` `describeError`; never print `e.message`.
- Hungarian text is `lang="hu"` (base rule in `assets/tailwind.css`); titles use `page-title`/`PageHeader`.
- Icons from `@lucide/vue` (not `lucide-vue-next`). Reuse `buttonVariants` etc.; `reka-ui` is unused.
  Light theme is the default; both palettes are contrast-measured (UX.md §14.1).
- Frontend `src/types` mirror `backend/internal/api/dto.go` (camelCase). Fiber v3 handlers return
  `fiber.NewError`. Logging is slog JSON. Pedagogical code cites `// [R5] see .claude/research/…`.
- Grammar teaches before it tests (first visit lesson, return visit exercise — judge by `attempts`, not phase).
- The embedded SPA has no modtimes: `index.html` is served with an ETag by `api.indexHandler`.
