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
| [docs/TUTOR.md](docs/TUTOR.md) | The grammar tutor: what it reads, how the prompt is built for a small model, what it may not do |
| [.claude/PLAN.md](.claude/PLAN.md) | The granular phase plan and its checklists |
| [.claude/DECISIONS.md](.claude/DECISIONS.md) | ADRs — what was considered and why this won |
| [.claude/research/](.claude/research/README.md) | **20 evidence notes.** Every pedagogical claim traces here |
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

**One test, not the suite** — the fastest loop, and the one thing the Makefile has no target for:

```bash
cd backend && go test ./internal/srs/ -run TestSpreadSentences -v   # one test, or a prefix
cd backend && go test ./internal/tutor/ -count=1                    # one package
```

Every `make` Go target runs the hidden `ensure-spa` first, because `//go:embed all:dist` will not
compile without `backend/internal/web/dist`: a hand-run `go test ./internal/api/` on a fresh
clone dies with `pattern all:dist: no matching files found` until any `make` Go target has
written the placeholder.

`make test` is the CI gate and takes minutes — and it can fail for a reason that is not yours.
`internal/drill`'s real-pack tests lose the CPU race when `go test ./...` runs every package at
once under `-race`, and hit **Go's default 10-minute per-package timeout**: `FAIL
tanulom/internal/drill 602s` with a goroutine dump out of `realpack_test.go`. That package passes
alone in ~330 s with `-race` and ~12 s without. Before concluding the backend is broken, rerun
with `make test GO_TEST_FLAGS="-race -count=1 -timeout 25m"`.

`make docs-check` (inside `make test`) is what **enforces** three of the rules below rather than
merely asking for them: every `[R…]` citation resolves, every research note states its code
impact, and `ru.json` / `en.json` carry the same keys. A convention with a checker is not advice.

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
  **Speech is the exception that proves it**: `tts` defaults to the `device` provider — the Web
  Speech API in the learner's browser, no endpoint, nothing deployed — so a fresh install has
  audio everywhere ([ADR-012](.claude/DECISIONS.md#adr-012)). The server cannot verify a browser
  and does not claim to: `/api/capabilities` reports `device` as configured and NOT reachable,
  and the client names the voice it actually found. The 🔊 controls never consult the server.
- **Curriculum order is ADVICE, not a lock** ([ADR-013](.claude/DECISIONS.md#adr-013)). An unmet
  prerequisite attaches `Decision.Advice` and the topic is practised anyway. Only two things
  hard-lock: a failed checkpoint, and the listening volume word order needs. Locking on order
  wedged a learner behind `vowel-harmony` — a motor skill the app itself says takes three
  months — with all of A1 shut behind it. Do not restore it.
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
- **`produce` grades the DICTIONARY form; `cloze` grades the form in the sentence.** They are
  different questions and one note carries both. Grading `produce` against `note.target_form`
  printed «день; солнце» and demanded `napokra` — a case the card never named, on a card whose
  placeholder promises «одно слово по-венгерски». `expectedAnswer` in `handlers_review.go` is the
  only place that decides this. [ADR-017](.claude/DECISIONS.md#adr-017).
- **The reveal must state the answer, and the client must never derive it.** «Подсказка» reveals
  without submitting, so there is no diff and no verdict — `QueueItem.Expected` carries the
  graded string for exactly that. Both it and the answer path go through `expectedFor`; a second
  copy of the produce/cloze rule in the client is how the hint comes to show one word while the
  grader wants another.
- **A cloze needs a SPAN, not features.** Features are what it can additionally grade; a
  featureless cloze grades the card and writes nothing to `morph_state`. Requiring them denied a
  sentence card to 23 of 37 words mined from a text. Both note creation and course enrolment ask
  `clozeable(note)`, so they cannot drift.
- **Sibling burial spans the DAY, not the page load.** `burySiblingCards` keeps one card per
  note within one queue build; a reload builds another, so answering `házban`'s cloze and
  refreshing served the same note's produce card a second later — with the answer still in the
  learner's memory, testing nothing, and reading as the app having forgotten every answer.
  `buryAnsweredSiblings` drops cards whose note was answered today by a DIFFERENT card; the
  card that was answered is kept, because its return ten minutes later is FSRS's learning step
  and not a sibling. Both halves are gated on `review.burySiblings`.
- **Free practice schedules NOTHING.** `/api/practice/*` is a second way to answer a card, never
  a second scheduler: no card state, no `morph_state`, and above all nothing in `review_log` —
  undo is delete-and-replay over that log and replay must reproduce stored state bit for bit, so
  a row that never fed the scheduler cannot live there. It writes `practice_log` and
  `daily_log.active_ms` only, never `due_left`, or a practice session would hand out a streak
  nobody earned. Both modes grade through `gradeTyped`/`expectedAnswer` so the verdict cannot
  drift. [ADR-019](.claude/DECISIONS.md#adr-019). Round `size` and `mode=weak` (lowest
  retrievability, weakest card per word) are the learner's EXPLICIT, on-screen choices — the
  hidden agenda stays rejected, the criterion is printed, weak falls back to random on a virgin
  deck and the response echoes what was actually served
  ([ADR-024](.claude/DECISIONS.md#adr-024)).
- **«Знаю» is practised, never scheduled.** The button sets a WORD aside — `note.trashed_at`,
  every direction of it, because hiding one card hands the same word back through another door.
  It is not a delete: the note, its cards, their FSRS state and every `review_log` row survive,
  which is what makes restore return the card *overdue* rather than new, and what keeps undo
  (delete-and-replay over that log) working across a trashing. `scope=trash` is accepted by
  `/api/practice/*` — practice records nothing, so nothing about a set-aside word moves — and
  **refused by `/api/review/queue`**; answering a card trashed since the queue was built is a 409.
  `Scope.filter()` carries the not-trashed clause for all three normal scopes so a new query
  cannot forget it; the few that build their own `WHERE` use `srs.NotTrashed`. Re-adding a
  trashed word (reader, or re-enrolling its unit) RESTORES it — `FindDuplicate` matches trashed
  notes, so without that «добавить» reported success on a word that stayed invisible forever.
  `all == course + reader` still holds: all three exclude the trash.
  [ADR-025](.claude/DECISIONS.md#adr-025). It ALSO marks the lemma `known`, and restore reverts
  it to `learning` ([ADR-041](.claude/DECISIONS.md#adr-041)): a word the learner says they know
  is a fact about the WORD, and without it the reader went on offering it as unknown in its n+1
  scoring. The learner-facing name is «Знаю» everywhere; the `trash` SCOPE VALUE stays on the
  wire, because it is ADR-025's contract and renaming it would break every stored reference.
- **A session never asks about one sentence twice in a row.** Ten words mined from a paragraph
  are ten notes over three sentences, created in one batch — so note order hands them out in a
  run, and due order does the same a day later. `srs.spreadSentences` deals the queue out by
  sentence: **a queue in which no sentence repeats comes back UNCHANGED** (the ordinary case, and
  what lets it run over the due list without disturbing oldest-first), and where one does repeat,
  the repeats dictate the order — most-cards-left first, never the sentence just served, ties on
  first appearance. Deterministic, never random: a session topped up by a maturing learning step
  must not look as though it reshuffled itself. Applied after each trim, never before — the
  `maxReviewsPerDay` cap drops from the latest-due end and the new-card budget takes the first N
  in curriculum order, so which cards are served stays untouched; only the order changes.
- **Review scope is a filter, never a second scheduler.** `?scope=course|reader` selects rows off
  `note.source`; the daily new-card budget stays GLOBAL, or a learner takes it twice over. Each
  tab shows due AND new — the reader pile sits at 0 due and 88 new, and one number would lie.
  [ADR-016](.claude/DECISIONS.md#adr-016). `text:<id>` is a SUB-slice of the reader pile via
  `note_origin`, drawn as a chip and never as a fourth tab — `all == course + reader` must keep
  holding, and the link is written on the duplicate-add branch too
  ([ADR-022](.claude/DECISIONS.md#adr-022)).
- **Review sets are the CLIENT's, and the server never hears of them.** The day's queue is dealt
  out in sets of a per-device size (default 15, `lib/sets.ts`) by `stores/review.ts`; the server
  serves exactly the list it served before, so `Breakdowns`, `QueueCounts` and the streak are
  untouched and «к повторению» stays `dueNow`. **A set is always the head of the served list, and
  the list is fetched again the moment a set finishes** — before its summary shows. That refresh
  is the only way a missed card comes back, so nothing is asked early. The client keeps NO retry
  list and NO countdown of its own: «вернутся через 0:41» over an emptied set was the mechanic
  the first learner asked to have removed. A miss is *listed* with its answer instead
  (`MissedList.vue`); a hit, even «Трудно», is never re-served in the set. `?order=random` is
  day-stable per card (`shuffleDayStable`), applied after the day cap and before
  `spreadSentences`, and never changes WHICH cards are served.
  **The choice is made on the start screen, never over the card:** pile, set size, order and a
  practice round's shape live at `/review` and travel to `/review/session` in the query; nothing
  else starts a session. [ADR-026](.claude/DECISIONS.md#adr-026),
  [ADR-028](.claude/DECISIONS.md#adr-028), [R18].
- **A card's memory figures are measured or computed by the library — nothing on that row goes
  up on its own.** `srs.Memory` is retrievability from `Scheduler.Retrievability`, the stored
  interval and stability, tallies from `review_log`, and `Scheduler.Preview` for what each grade
  would schedule (labelled approximate — fuzz is seeded at answer time; absent in practice, where
  nothing is scheduled). Young/mature splits at 21 days, Anki's convention, and the UI says so.
  **The review card does not print the retrievability** — in the learning steps the model
  computes it against an interval of minutes and it read «100%» over five crosses, which a
  learner reads as the app claiming they know a word they just missed five times. **Do not put it
  back on the card**; it stands in the words browser, where every card has graduated. Anything
  that would make this a score — XP,
  combos, a number that only rises — belongs to [R16 §3] and is not added here.
  **The card DOES say what «~10 мин» is a step towards.** `srs.Memory` carries `stepsLeft` /
  `stepsTotal` (go-fsrs's own `RemainingSteps` against its `LearningSteps`, `{1, 10}` by default)
  and `graduatesOnGood`, asked of `Scheduler.PreviewNext` rather than guessed from the interval.
  The four-rung ladder draws the stage the server already reports; it is a POSITION, not a score
  — it can go back down, it has no end, and nothing on it accumulates. There is still no
  «N правильных подряд» threshold, because FSRS has none, and the copy says so. «что это значит?»
  explains what makes a word stop coming back — the stage, what each answer schedules, that the
  tally is a log with no «N in a row» threshold, and that «В корзину» is the one control that
  removes a word. **No completion bar: there is no denominator, and inventing one is the score
  [R14] rules out** ([ADR-039](.claude/DECISIONS.md#adr-039)).
- **The grammar tutor explains; it mints nothing.** No code path from `tutor_message` to `note`,
  `card`, `user_gloss` or either state space — `handlers_tutor_test.go` counts those tables
  before and after a question. Every answer names its model and lists the backticked Hungarian
  the pack cannot find. The lesson is found **per question** and the answer prints which one it
  used («по уроку: …» / «по справочнику, без урока»); never hide that label — an inferred choice
  the learner cannot see is one they cannot correct. `tutor` imports nothing from `curriculum`.
  Three things a change here breaks silently, so they are stated and not only cited: the prompt's
  language rule names the EXPLANATION and never the material — **never write `ONLY` back in**
  (`tutor_test.go` pins its absence), or a small model answers «напиши на венгерском …» with the
  Russian sentence unchanged; a cut answer is `done` + `interrupted`, **never** `error`, which
  means nothing arrived, and `MaxAnswerTokens` and `streamTimeout` are sized **together** because
  a word-by-word breakdown is the one answer allowed past eight sentences; and a review chat is
  **one per WORD per local day** (`tutor_thread.note_id`),
  with `ReviewView` keeping the thread id WITH its note and `useTutorThread` carrying a
  generation counter — neither is optional, or the panel serves the previous word's conversation.
  The review card's tutor reads the card **from the server** (`POST /api/tutor/:id {card}`): the
  client never assembles the facts and may not derive the expected answer. **Asking never
  changes a word's schedule** ([ADR-046](.claude/DECISIONS.md#adr-046)): before the answer the
  prompt withholds it and `tutor.Allowed` refuses the templates that would name it, so a
  question is not a hint — there is no `asked` event and no `consultTutor`, and «Подсказка» is
  the only thing that sets `usedHint`. A price on asking taxes exactly the reading [R4] and
  [R9] say is worth the most; do not reintroduce one. Nothing on the review path reads a tutor table, and the standing
  honesty paragraph and the sources list stay OFF that panel — what defends an answer travels
  with the answer.
  `internal/tutor/kb` is CC BY-SA + public domain with its own `LICENSES.md`; `src-*.md` are
  generated by `make tutor-kb`, `ref-*.md` are hand-written — never edit a generated file.
  **Read [docs/TUTOR.md](docs/TUTOR.md) before changing any of it**, and the ADRs it rests on:
  [ADR-027](.claude/DECISIONS.md#adr-027), [ADR-029](.claude/DECISIONS.md#adr-029),
  [ADR-030](.claude/DECISIONS.md#adr-030), [ADR-031](.claude/DECISIONS.md#adr-031),
  [ADR-034](.claude/DECISIONS.md#adr-034), [R19].
- **The expected answer never leaves the server.** Items are held in `internal/drill/pending.go`
  between serve and answer — [ADR-008](.claude/DECISIONS.md#adr-008).
- **The embedded SPA has no modification times.** `//go:embed` reports the zero instant for
  every file, so a modtime-based 304 answers "not modified" to any conditional request.
  `index.html` is the one unfingerprinted file — it names the current asset hashes — so it is
  served by `api.indexHandler` with an **ETag over its bytes**, on GET *and* HEAD, and a missing
  `/assets/*` is a 404 rather than the shell. Both failures look identical from the outside: a
  blank page after an update, fixed only by a hard reload.
- **A LINE BREAK ends a sentence, and the splitter needs the source text to see one.**
  Whitespace is not tokenised, so a newline survives only as the gap between one token's
  `End` and the next one's `Start` — which is why `morph.SplitSentences` takes the text as
  well as the tokens. Without it a heading, carrying no terminal punctuation, swallowed the
  sentence below it and mined cards asking about
  `Egy átlagos nap Budapesten Reggel hét órakor ____.` A blank line always splits; a single
  newline splits only before a capital, because the reader invites the learner to paste
  subtitles and those wrap mid-sentence. `text_token.sent_idx` is frozen at ingest, so
  `api.repairSentences` re-splits a stored text when it is opened.
- **Offsets are UTF-8 BYTES on the server and UTF-16 units in the browser.** `Token.Start/End`,
  `note.target_start/target_end` and `text_token.start/end` are byte offsets into Go strings.
  A JavaScript client must never slice with them: `Jó éjszakát, Timmy!` blanked as
  `Jó é_____immy!` for exactly this reason. Split on the server (`srs.splitCloze` sends
  `cloze: {before, after}`) or re-encode on the client (`ListenView`'s `TextEncoder` slice).
- **`COLLATE NOCASE` defeats the pack's indexes.** `idx_wordform_form` is BINARY-collated, so
  `WHERE form = ? COLLATE NOCASE` scans two million rows. Use `caseVariants` +
  `IN (?,?,?)` — `internal/morph/casefold.go`.
- **No HVPT contrast is ever marked complete.** Effects decay; contrasts get FSRS maintenance
  forever, and the adaptive selection weight has a positive floor. A session below
  `hvpt.minTalkers` distinct voices is **refused with the numbers**, never silently degraded.
  See [R7](.claude/research/07-hvpt.md). **Every pair member is a word the dictionary
  carries** ([ADR-033](.claude/DECISIONS.md#adr-033)): the inventory once held a dozen
  non-words glossed «(нет слова)», and the learner's verdict was «как будто таких слов нет».
  `minimal_pairs.unattested` refuses the build and the refresh; clips are named by the word,
  never by the pair id, which a refresh reassigns.
- **The review log replays to stored state bit-for-bit** — fuzz is seeded, so this includes due
  dates. Undo is delete-and-replay on that property (`internal/srs/replay.go`); do not add
  stored-previous-state bookkeeping beside it, and keep `review_time` at millisecond precision.
  **Bit-for-bit holds on ONE CPU architecture**: go-fsrs's fuzz generator is floating-point and
  Go fuses multiply-adds on arm64 only, so the same seed draws a different fuzz on CI's amd64
  than on an arm64 laptop. A test whose setup differs only by a fuzzed day passes here and fails
  there (`reconcile_test.go` did) — pin structural differences (state, steps, lapses) and
  reproduce CI with `GOARCH=amd64 go test` under Rosetta. ARCHITECTURE.md §5.7.
  Stored state includes `remaining_steps` — go-fsrs's position within the learning steps.
  Dropping it from any load/save path silently re-graduates «Снова» on a learning card to
  tomorrow instead of one minute; migration 00011 and the boot-time reconciliation exist
  because it was dropped once ([ADR-020](.claude/DECISIONS.md#adr-020)).
- **Every review number a learner sees comes from `srs.Breakdowns`** — post-burial,
  day-granular ([ADR-021](.claude/DECISIONS.md#adr-021)). The tab strip, «Сегодня» and
  `daily_log.due_left` (via `ActionableDue`) speak that one vocabulary: `dueNow` is what the
  queue will actually serve (one card per word; graduated cards count all local day, learning
  steps once elapsed), `dueLaterToday`+`nextDueAt` name the steps that return today, and
  `buriedToday` is where hidden-but-due cards are counted instead of inside a bigger number.
  Do not reintroduce a raw card count on any screen — «26 к повторению» over a 10-item session
  is exactly how this app confused its first learner. The streak needs `dueNow == 0`, not the
  later-today steps: a 10-minute step at 23:55 must not break a day.
- **Hours and streaks accumulate; they are never claimed.** Listening AND reading progress are
  clamped to 60 s per report (`maxReportSeconds`, shared); the streak counts obligations met
  (queue cleared + stated minutes), and Loop 4's attestation is recorded but deliberately
  ignored by it. [R14] **Reading reports time too** — it was the one loop that reported nothing,
  so an hour with a text left the day unmet ([ADR-044](.claude/DECISIONS.md#adr-044)). A second
  counts only while the tab is VISIBLE and the learner interacted recently
  (`composables/useActiveTime.ts`); the server's clamp means the client does not have to be
  trusted. The streak's definition is unchanged — what changed is that the minutes now include
  work that was always work. `daily_log.read_s` sits beside `listen_s` so the chart can say
  where an hour went, and where a day failed the UI names WHICH half failed: they fail
  independently, and «не выполнена» on a day whose queue was clear pointed at the wrong thing.
- **A pack is always present — never write a "no pack" dead end again.** The binary embeds a
  starter pack (`internal/lang/starter.go`) and `lang.EnsurePack` installs it on first boot;
  `lang.Detect` **verifies that a candidate opens** rather than trusting the filename, because a
  zero-byte download once reported as present and left every language feature dead behind a
  message telling the user to fetch what was already on disk. A pack-dependent route answering
  503 is a genuine failure path, not the normal first-run state: UI copy must not send the
  learner off to download anything, and `packPresent: false` from `/api/bootstrap` means
  something broke. The full pack is an **upgrade** (`make langpack`).
  **`backend/internal/lang/starter/*.gz` is committed, not generated** — `//go:embed` on a
  missing file is a compile error, so gitignoring it would mean a fresh clone cannot build.
  Change the pipeline and you must run `make starter` and commit the result, or the binary ships
  last month's course; CI checks this. `make starter` needs an ABSOLUTE `STARTER_DIR` — a
  relative one resolves against `langpack/`, where the recipe cd's, and silently writes a shadow
  directory nothing reads ([ADR-010](.claude/DECISIONS.md#adr-010)).
- **A course unit's completion is derived from the deck, never stored.** `course.Resolve` calls
  a unit `done` when every one of its words is in `lemma_state` as `learning`/`known`.
  `course_progress` records enrolment only — do not add a `completed` flag, an endpoint that
  sets one, or a client-side override. The moment completion is storable, the plan screen can
  disagree with the deck, and "done" stops meaning "learned".
  **But the SCREEN must not turn that derivation into a claim about the learner.** `Resolve`
  calls the first non-done unit `current`, which is advice about what to take next; labelled
  «сейчас» it told someone who had finished month 1 and opened nothing since that they were in
  month 2. The chip says «следующий», the dot follows ENROLMENT (something the learner did) and
  the legend draws all four states: пройден · начат · следующий · впереди. Same rule as
  `fromHere`: keep the derivation, fix what the UI claims about it.
- **No XP, no levels, no leagues, no leaderboards, no gems, no lootboxes.** Gamification's
  effect on *competence* is null, and extrinsic point economies are the part most likely to
  crowd out the real goal — [R14](.claude/research/14-motivation.md),
  [R16 §3](.claude/research/16-debunked.md). The permitted motivational surface is exactly:
  the streak (obligations met), the lemma curve, the morph heatmap, checkpoint pass/fail, and
  the exam countdown. Every one of those is a **competence** signal with something measured
  behind it. A request for "a bit more fun" is answered inside that set.
- **No LLM-generated Hungarian becomes a card without passing `internal/corpus/attest.go`** —
  the guardrail lands with the LLM in Phase 9; `internal/corpus` does not exist yet and nothing
  generates Hungarian *as material* yet. The tutor's Hungarian is explanation, labelled
  and dictionary-checked, and stays on the screen it was written on.
  **The AI dictionary entry is the same kind of thing and a DIFFERENT code path**
  ([ADR-042](.claude/DECISIONS.md#adr-042)). `handlers_ai_entry.go` is deliberately not a tutor
  route: the tutor mints nothing and a test counts the tables to prove it, so routing this
  through it would delete that guarantee. The entry is offered ONLY where the pack has nothing —
  the pack's own is better — every Hungarian token in it goes through
  `tutor.Backticked`/`Unattested`/`wordExists` **including the ones the model forgot to
  backtick** (otherwise a model that drops them turns the guardrail off silently), and it reaches
  `user_gloss` only as `machine` on an explicit save and a card's back only after «Принять как
  своё» re-saves it `manual`. Its example sentences are illustration, say so on the panel, and
  never become notes. Prompt: `.claude/prompts/word-entry.md`.
- **Machine translation is display-only, and always says who wrote it.** `services/mt` (NLLB-200)
  generates Russian and English, not Hungarian, so `corpus` attestation cannot help — a wrong
  translation is perfectly well-formed. **Labelling is the entire defence**: `model` travels with
  every row from `POST /api/translate`, cached rows included, and the UI prints it beside the
  text, never in a tooltip. It never fills a note's gloss, a card's answer or anything graded,
  and saving it does not change that. It MAY stand beside the Hungarian as prompt CONTEXT on a
  `cloze` — the graded string is still the Hungarian form — but never on the back of a card
  ([ADR-018](.claude/DECISIONS.md#adr-018)). Kept in `user_gloss` it is `source='machine'` with
  the model, and reaches a CARD only after «Принять как своё» re-saves it as `manual`: a card's
  back is what `produce` grades against, so it has to be a line a person accepted rather than one
  the app laundered on their behalf. `lib/gloss.ts` is the ONE place that picks a gloss language
  and reports the fallback — no view keeps a `ru || en` of its own — and `SentenceCrib` prints
  the machine line below the human one, never instead of it
  ([ADR-035](.claude/DECISIONS.md#adr-035)). `services.mt` refuses `provider: openai`: it speaks
  its own `/translate` and is deliberately a different tool from the `llm` slot — a 600M
  translation model beats a 7–8B chat model at hu→ru.
  [ADR-014](.claude/DECISIONS.md#adr-014).

## Conventions that bite

- **No jargon in learner-facing copy.** «корпус» / "corpus" is our word for a build input, not
  a word a learner has any reason to know — say "живые примеры" / "real examples". Same for
  topic codes (`definite-conjugation`), checkpoint slugs (`m3`) and feature ids: every one of
  them has a human name somewhere, and the screen that shows the slug is always the screen
  where it is least readable.
- **A sentence in the pack is one a person wrote.** The corpus is Tatoeba plus a heavily
  filtered slice of OPUS OpenSubtitles, and the subtitle side is chosen by RECURRENCE with a
  majority vote on the translation — OpenSubtitles' automatic alignment fails at the head of
  every film, so a translation with no majority is dropped while the sentence is kept.
  `langpack/pipeline/subtitles.py`. `make subtitles` adds them to an existing pack in minutes
  (the sentence stage never calls HuSpaCy); the **starter pack takes `source='tatoeba'` only**,
  because it is compiled into a binary we publish and OPUS's licence does not cover that.
- **Verb forms are generated for recognition, never for production.**
  `langpack/pipeline/verbs.py` fills the indicative cells Wiktionary misses (`hoztunk` was
  absent while `hoztuk` was attested). Generated rows lose to attested at lookup, are refused
  as drill answers, and the suppletive verbs are skipped entirely. `make verbs` backfills a
  pack in seconds.
- **The corpus lemma id and the dictionary lemma disagree.** Anything looking for "a real
  sentence containing this word" must fall back from `sentence_token.lemma_id` to a surface-form
  search over `morph.CaseVariants`, then re-verify with the analyser. `nagyon` is tagged as
  `nagy`; joining on lemma id alone silently reported a corpus gap across 820 sentences and
  dropped the word from its course unit. `handlers_course.go:noteFromSentences`.
- **`services.llm` needs a model name, and the settings row verifies it.** The request names
  the model, every answer is labelled with it, and Ollama refuses one without — so a blank
  model is a client that can only fail, and it failed as «модель не ответила» under a row that
  said «на связи». `llm.New` returns `ErrNoModel` (→ `errors.llmNoModel`, the dial named), the
  probe asks a reachable endpoint `GET /v1/models` and reports `no model` / `unknown model` with
  the list for the picker, and a `local` provider is sent `reasoning_effort: none` because a
  thinking model spends `MaxAnswerTokens` on hidden reasoning first. Ollama also needs a
  context of 8k+ for the tutor's prompt, and only the learner can set that — docs/TUTOR.md §9.
- **A feature id is not a label.** `Case=Sbl` identifies a feature; it does not name one for a
  learner. The pack ships `morph_feature.label_*` and `explain_*` in both locales — resolve
  through `stores/features.ts`, never print the bare id as the only text. These labels live in
  the pack rather than in `vue-i18n` on purpose: they are facts about the language being taught
  and they version with the pack, so ADR-004 does not apply to them.
- **Grammar teaches before it tests.** `GrammarView` opens a topic on the LESSON with no
  exercise on screen; a button moves to the drills. [R4] is the strongest finding in the set and
  a screen that leads with the exercise is contradicting it. **On a RETURN visit the exercise
  leads** ([ADR-043](.claude/DECISIONS.md#adr-043)) — the FIRST visit is what [R4] governs, and
  making a learner scroll past the whole lesson every time was the complaint. The evidence for
  "already read it" is `topic_progress.attempts > 0` plus a per-device record of the
  «К упражнениям» press. **Never the PHASE**: `drill/schedule.go` starts every drillable topic
  at `blocked` ([R5]), so treating that as evidence puts an exercise in front of every
  first-time topic. Lesson prose lives in
  `langpack/pipeline/lessons.py`, and `curriculum.validate` enforces a floor on its length
  because every one of them was once a fifty-character blurb.
- **Pedagogical code cites its note:** `// [R5] ... see .claude/research/05-interleaving.md`.
  **No design decision may cite a paper that has no note**, and no note exists without a
  "What this changes in our code" section.
- **Both locales, always.** A change adding an `en` key without its `ru` counterpart is
  incomplete. [ADR-004](.claude/DECISIONS.md#adr-004).
- **Two faces, one lever each** ([ADR-036](.claude/DECISIONS.md#adr-036)). Hungarian is
  marked `lang="hu"` and set by the base rule in `assets/tailwind.css` — never with a font
  class; page titles use the `page-title` utility (or `PageHeader.vue`), never a hand-written
  `text-3xl font-semibold`. Fonts are bundled subsets from `assets/fonts.css`; nothing loads
  from the network.
- **Drawings are chrome, never a metric** ([ADR-037](.claude/DECISIONS.md#adr-037)). The line art in
  `components/art` is one colour (`currentColor`), decorative (`aria-hidden`), under 2 KB a
  motif, and lives on empty states, the done screen, 404 and under the wordmark at the two
  doors. It never draws progress — no rings, flames or numbers (UX.md §14.5) — and never sits on
  a `Note`, a card or the Progress screen.
- **Icons: `@lucide/vue`**, NOT `lucide-vue-next` (deprecated). Some names changed in v1
  (`Home`→`House`); verify via type-check.
- **Tailwind v4 resets buttons to `cursor: default`.** Reuse `buttonVariants`, `TabsTrigger`,
  `SelectTrigger`, `DropdownMenuItem` rather than hand-rolling a bare `<button>`.
- **UI = hand-rolled primitives on Tailwind v4** in the shadcn idiom: `src/components/ui`
  holds button, card, input, label, select, badge, progress, skeleton — cva variants, design
  tokens from `assets/tailwind.css`. `reka-ui` sits in package.json UNUSED (no imports); do not
  assume shadcn-vue components exist — build the primitive or reuse one. Names there are
  intentionally single-word; `eslint.config.ts` disables `vue/multi-word-component-names`.
  Shared page chrome (`PageHeader`, `EmptyState`, `Note`, `KeyBar`) lives in `components/common`.
- **Light theme is the DEFAULT** (`:root` in `assets/tailwind.css` is the light palette, `.dark`
  the override; `index.html` ships `class="light"`, `stores/ui.ts` honours an explicit OS dark
  preference). Both palettes are measured — see docs/UX.md §14.1; keep every value's contrast
  comment in step if you touch one.
- **Migrations**: add `internal/db/migrations/NNNNN_name.sql` with `-- +goose Up`/`Down`, dialect
  `sqlite3`. **Never edit an applied migration.** They are embedded and run inside `db.Open` —
  there is no migrate command and distroless has no shell to run one.
- **Every failure a learner sees goes through `lib/errors.ts` `describeError`**
  ([ADR-038](.claude/DECISIONS.md#adr-038)). The tutor speaks in i18n keys, the older handlers
  in English prose, the client in sentinels; one function turns all three into a sentence in
  the learner's language, and `main.ts` toasts whatever nobody caught. Never print `e.message`
  on a screen — `network-unreachable` reached a learner four times that way.
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
- **The export is a FULL dump, and never an `.apkg`.** `internal/profile` +
  `/api/profile/{export,import,size}` carry every user-scoped table — both state spaces, the
  review log, texts, glosses, the daily ledger, tutor threads — plus `media/`, as a zip.
  ADR-011's refusal of `.apkg` stands unchanged and this is not that format
  ([ADR-040](.claude/DECISIONS.md#adr-040)). Three things make it safe to keep:
  **columns are discovered** via `PRAGMA table_info` rather than listed, so a migration cannot
  silently drop one; a **manifest coverage test** fails when a new table is neither exported nor
  excluded with a reason; and the import's integrity check is the **replay property** — every
  card's log is replayed and compared to the state it arrived with, failing on the same
  architecture and warning across two. **Secrets are omitted** (`settings` rows with
  `secret = 1`) and both screens say so. Import **replaces**, in one transaction, for the acting
  user only: merging two FSRS histories means inventing a learner's past.
- **The app cannot replace a teacher.** Loop 4 is un-completable by the app, on purpose.
