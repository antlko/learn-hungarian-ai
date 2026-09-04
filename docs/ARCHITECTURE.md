# Architecture

How the system behaves. Read this before changing anything structural.

**Read first:** [`.claude/DECISIONS.md`](../.claude/DECISIONS.md) for *why* the shape is what it
is. This document describes the *what*.

---

## 1. Shape

A **static Go binary** embeds the built Vue SPA and serves both the API and the UI on one port.
Optional sidecars add speech and NLP. Nothing in v1.0 requires a sidecar.

```
                    ┌──────────────────────────────────────────┐
   browser ────────►│ tanulom  (one static binary, distroless) │
                    │  ┌────────────────────────────────────┐  │
                    │  │ Fiber v3 · API + SPA catch-all     │  │
                    │  ├────────────────────────────────────┤  │
                    │  │ srs · morph · mining · drill · hvpt│  │
                    │  │ curriculum · course · stats · lang │  │
                    │  └───────┬──────────────────┬─────────┘  │
                    │          ▼                  ▼            │
                    │   tanulom.db          langpack-hu-vN     │
                    │   (read-write)        (read-only, ATTACH)│
                    └──────────┬───────────────────────────────┘
                               │  optional, probed at runtime
              ┌────────────────┼─────────────────┬──────────────┐
              ▼                ▼                 ▼              ▼
        services/nlp     services/tts      services/asr     any OpenAI-
        HuSpaCy          Piper             faster-whisper   compatible LLM
```

---

## 2. The two state spaces — the core idea

Everything distinctive follows from refusing to treat a wordform as an atom.

```
  "házaimban"
       │
       ▼  internal/morph — indexed lookup in the language pack
  ┌────────────────────────────────────────────────┐
  │ lemma: ház                                     │
  │ feats: Case=Ine | Number[psor]=Plur |          │
  │        Person[psor]=1 | Number=Sing            │
  │ segmentation: ház + a + im + ban               │
  └───────┬──────────────────────────┬─────────────┘
          ▼                          ▼
   lemma_state[ház]          morph_state[Case=Ine]
   (FSRS-6 state)            morph_state[Number[psor]=Plur]
                             morph_state[Person[psor]=1]
                                   (FSRS-6 state each)
```

Hungarian UD exposes **22 case values** plus mood, verb form, definiteness, possessor marking,
voice, aspect and degree — roughly **120 teachable atoms** after merging allomorph variants.
Inventory in [R17](../.claude/research/17-hungarian-nlp-stack.md).

**What this unlocks:**

| Capability | Why it needs two spaces |
|---|---|
| Real `n+1` | `elmentem volna` has *zero* unknown lemmas and *two* unknown features. Lemma-only counting calls it easy; it isn't. |
| Per-suffix weakness map | "Your inessive is solid; your conditional is at 0.41 retrievability" is only expressible if features have their own state. |
| Targeted generation | LLM prompts carry the known-lemma set *and* the feature to drill. |
| Honest progress | You cannot fake B1 by grinding vocabulary while morphology rots. |

---

## 3. Data ownership

Two SQLite files in the data dir, plus one YAML.

| Store | Holds | Mutability |
|---|---|---|
| `config.yaml` | settings: review params, mining weights, drill thresholds, service endpoints | hand-editable; app writes atomically |
| `tanulom.db` | users, sessions, **secrets**, notes, cards, review logs, both state spaces, texts, drills, HVPT trials + contrast states, the listening ledger, the daily ledger, checkpoints, corrections, `course_progress` | read-write |
| `langpack-hu-vN.sqlite` | lemmas, wordforms, paradigms, features, sentences, minimal pairs, curriculum, n-grams, the guided course (`course_unit`, `course_word`) and the hand-off catalogue (`resource`) | **read-only**, `ATTACH`ed |

**Invariants.**

1. **`config.yaml` is mutated only via `Server.updateConfig`** — clone → mutate → validate →
   atomic save (temp+rename) → swap under lock. Never touch `s.cfg` directly.
2. **Secrets never go in YAML.** API keys live in `tanulom.db`, are stripped from DTOs (only a
   `secrets: {key: true}` presence flag is exposed), and are preserved on `PUT` when omitted.
3. **The language pack is never written to.** Learner corrections to a bad analysis go in
   `morph_correction` in the user DB and are applied as an overlay at lookup time.
4. **`normalize()` fills zero fields only.** Changing a default affects new installs and unset
   fields — never silently rewrites a user's stored values.
5. **Migrations are embedded and run inside `db.Open`.** There is no migrate command and the
   distroless image has no shell to run one. Never edit an applied migration.
6. **The split follows who owns the fact.** An ordering of Hungarian vocabulary is language
   content, so the syllabus is in the pack; which units a learner took up is learner state, so
   `course_progress` is in the user DB. Note what `course_progress` does *not* hold: a
   "completed" flag. A unit is done when its words are in the review deck — see [§5.11](#511-the-guided-course--internalcourse).

---

## 3a. Pack bootstrap

**There is always a pack.** The binary embeds one (`internal/lang/starter.go`), and
`lang.EnsurePack` installs it into the data directory on first boot — gunzip beside the target,
**prove it opens**, then rename into place. The ear trainer's clips ride along in the same
embedded archive, skipping any file already present so a re-install never clobbers learner media.

Two failures shaped this:

- **An app that boots into "download a language pack" is a build step wearing a UI.** Every
  screen except Settings is dead, and the newcomer is asked to solve a distribution problem
  before reading a word of Hungarian. The starter pack — the whole curriculum and course, the
  5,059 lemmas with their 517,837 forms and glosses (2,259 of them glossed in Russian, and all
  853 course words), 11,400 sentences it can fully explain, every minimal pair — is 15.6 MB
  gzipped in the binary, beside 513 KB of Opus clips, and is enough to work through the first
  months for real. The 252 MB full pack is an **upgrade** that widens the vocabulary, built with
  `make langpack`; it is offered, never required.
- **`lang.Detect` verifies that a candidate opens, rather than trusting the filename.** A
  truncated download or an interrupted copy leaves a zero-byte `langpack-hu-v1.sqlite`, which
  the old name-only check reported as present. The server then started with morphology, reading,
  drills, the ear trainer and the curriculum all dead, behind a message telling the user to
  download a pack that was, as far as they could see, already there. Verifying costs one `Open`
  per candidate at startup, and newest-first means a working older pack beats a broken new one.

`EnsurePack` runs before the pack is opened, so a corrupt file is *replaced* rather than merely
reported. If even that fails the server still boots and every language route answers 503 — the
same honest degradation as an absent sidecar — and the log says why.

`GET /api/bootstrap` therefore reports `packPresent: true` on a fresh install.
`pack_meta.edition` distinguishes `starter` from `full`, so the UI can offer the upgrade without
guessing from the file size.

---

## 4. Package graph

Import direction matters; these are the edges that prevent cycles. Every edge below is a real
import — `grep -h '"tanulom/internal/' internal/<pkg>/*.go` reproduces the list.

```
cmd/tanulom
    └── api ──┬── config ── state
              ├── db (goose migrations, embedded)
              ├── auth
              ├── lang ── (embeds the starter pack; opens + ATTACHes)
              ├── morph ── lang
              ├── srs ── db
              ├── mining ── (pure: text and rows in, scores out)
              ├── curriculum ── lang
              ├── course ── lang
              ├── drill ── curriculum, morph, srs, lang, db
              ├── hvpt ── drill, srs, lang, db
              ├── stats ── db
              ├── mt ── config (optional sidecar; a nil client is "switched off")
              ├── llm ── config (OpenAI-compatible chat, blocking and streamed)
              ├── tutor ── llm, db (the embedded reference corpus, retrieval, topic inference, the prompt, threads)
              └── web (//go:embed all:dist)

    reserved, empty until v1.1:  corpus · speech · media
```

`internal/mining` imports nothing from `internal/` at all: ingestion, coverage and the `n+1`
scorer take plain values, which is what makes them cheap to test. `internal/course` sits beside
`curriculum` rather than under it — one owns the order rules arrive in, the other the order
words do, and neither should know about the other's DAG.

**Never:** `lang` or `morph` importing `srs` (they are language knowledge, not learner state).
**Never:** `srs` importing `drill` (the scheduler must not know about exercise types).
**Never:** `course` importing `srs` — the syllabus is pack content; the join to the learner's
deck happens in `api` (`Server.inPlaySet`), because `lemma_state` keys on the pack's `lemma_id`
across two separate databases.
**Never:** `llm` importing anything that would let it write cards directly — generation goes
through `corpus` attestation and then through the normal note-creation path.
**Never:** `tutor` importing `srs`, `drill` or `course`, or anything under `internal/api`
writing tutor output into `note`, `card`, `user_gloss` or either state space. The tutor
explains; it mints nothing ([ADR-027](../.claude/DECISIONS.md#adr-027)), and
`handlers_tutor_test.go` counts those tables before and after a question.
**Never:** `mt` output reaching a note, a gloss or a graded answer. It is a display-only crib,
labelled with the model that wrote it wherever it appears
([ADR-014](../.claude/DECISIONS.md#adr-014)); the guardrail `corpus` provides for generated
Hungarian cannot catch a translation that is merely wrong, so labelling is the whole defence.

`internal/export` is **gone**, with the `.apkg`/CSV/backup endpoints it served. Data safety is a
filesystem property here: the data directory is self-contained, so a backup is a `tar` of a
volume and a restore is putting it back ([SELFHOSTING.md](SELFHOSTING.md#backup-and-restore)).

---

## 5. Key algorithms

### 5.1 `n+1` sentence scoring — `internal/mining/score.go`

```
known(lemma)   := status = 'known' OR retrievability(lemma_state) ≥ θr     θr default 0.80
known(feature) := retrievability(morph_state) ≥ θr
unknown(S)     := |lemmas ∉ known| + w_f · |features ∉ known|              w_f default 0.7

score(S) = a₁ · gauss(unknown(S); μ=1.5, σ=0.8)     // the n+1 sweet spot
         + a₂ · [elementCoverage(S) ≥ 0.90]          // comprehensibility floor  [R6]
         + a₃ · utility(new lemmas)                  // frequency-weighted payoff
         + a₄ · relevance(current curriculum topic)
         + a₅ · lengthFit(n; target 6–14)
         − a₆ · dupPenalty(S)
```

All weights in `config.yaml` under `mining:`. Tunable without a rebuild.

### 5.2 Dual-space review emission — `internal/srs/emit.go`

One cloze or produce answer emits **1 card review + N feature reviews**.

| Outcome | FSRS rating |
|---|---|
| correct, latency < learner's median | `Easy` (4) |
| correct, normal latency | `Good` (3) |
| correct but slow, or after a hint | `Hard` (2) |
| incorrect | `Again` (1) |

**Only features actually required by the item are graded** — derived from the blanked span's
`feats`, not from every token in the sentence. Grading unrequired features is what would turn
the heatmap into noise.

### 5.3 New-card governor — `internal/srs/forecast.go`

Simulate 90 days from the current stability distribution, new-card limit `L`, desired retention
`R`. If projected peak load exceeds `daily_minutes / median_seconds_per_review`, **refuse to
raise `L`** and render the curve. The learner may override — after seeing it.

This exists because the brief names review-avalanche as the single most common cause of quitting.

### 5.4 Corpus attestation — `internal/corpus/attest.go` *(v1.1, not yet written)*

2–4-grams of any LLM-generated Hungarian are checked against the pack's `ngram` index.
Unattested spans are marked *«не подтверждено корпусом» / "not attested in corpus"* and
**cannot become a card without an explicit override**. See
[R10](../.claude/research/10-llm-tutors.md).

The `ngram` index ships in both packs already — it is tiny and it is a guardrail — but
`internal/corpus` does not exist until Phase 9 brings the LLM in. Nothing generates
Hungarian in v1.0, so there is nothing yet for it to guard.

### 5.5 The blocked → interleaved gate — `internal/drill/schedule.go`

A pure function of the learner's counters. Same numbers in, same verdict out — which is what
lets the UI show the learner the exact rule the server applied.

```
phase(topic) :=
    locked        if a prerequisite has not itself reached `interleaved`,
                  or the listening gate is shut                             [R15]
    explained     if no generator can build items for this topic
    blocked       if rolling_accuracy(window = drill.gateWindow) < gateThreshold
    interleaved   if that accuracy >= gateThreshold
    mastered      if interleaved_accuracy(window = masteryWindow) >= masteryThreshold
                  AND, for a motor-skill topic, median latency <= harmonyLatencyMs
```

Four things about this are load-bearing:

1. **A prerequisite counts as met at `interleaved`, not at `mastered`.** Requiring mastery of
   every ancestor would serialise a two-year curriculum into a crawl. [R5]'s floor is about
   whether the learner can absorb the load of the topic in front of them.
2. **`explained` exists so an un-drillable topic cannot wall off the curriculum.** The alphabet
   has a real rule card and no exercises; it teaches by explanation and does not gate.
3. **Two accuracy windows, not one.** The gate window counts *every* answer, so the fallback can
   see a collapse. The mastery window counts only answers served under interleaving — otherwise
   a learner could reach "mastered" without ever having had to discriminate.
4. **The door swings both ways, with hysteresis.** Below `gateThreshold − hysteresis` a topic
   falls back to `blocked` — from `mastered` too, all the way down, because if the learner can
   no longer do it at all then mixing it with confusable items is exactly wrong.

### 5.6 Drill item generation — `internal/drill/generate.go`

Six generators, chosen per topic from the features it teaches rather than from a table, so
adding a topic to the pack needs no code change: `allomorph | case | conjugation | cloze |
transform | mcq`.

Two rules constrain every one of them.

- **The correct answer must be attested.** Distractors may be constructed — a wrong allomorph is
  not a word, which is the point — but the form marked right has to be one the language has.
  Resolution goes through `internal/morph/forms.go`, never through the pack's `paradigm` table,
  which is generator-only and disagrees with the dictionary in places.
- **An item declares exactly the features it required.** A harmony item grades
  `Skill=VowelHarmony` alone; a case choice between forms of one noun does not grade harmony at
  all, because every option shares the stem's harmony class.

Phase decides the mix. `blocked` serves **one kind**; `interleaved` shuffles kinds and contrasts
minimally different items — `-ban` against `-ba` of the *same* noun, definite against indefinite
of the *same* verb. Variety is not the mechanism; discriminability is. [R5]

Availability is **proven by building an item**, cached per topic: a topic can pass every
structural test and still produce nothing, because the pack's attested `olvashat` carries no
features at all. Saying "we have not built exercises for this" is a supported outcome.

### 5.7 Replay — `internal/srs/replay.go`

The review log is complete so that replaying it reproduces stored FSRS state **bit for bit**
(fuzz is seeded from review time + reps, so even fuzzed due dates replay). Two things stand on
that property:

- **Undo** is "delete the last log row and replay the rest" — in both state spaces, in one
  transaction. No stored-previous-state bookkeeping that can drift.
- The **property test** (`replay_test.go`, 200 randomised trials) that catches any change —
  including an FSRS upgrade — breaking the correspondence.

### 5.8 The ear trainer — `internal/hvpt`

2AFC identification over the pack's minimal pairs. Three [R7] rules are load-bearing:
**multiple talkers are the active ingredient** (a session below `hvpt.minTalkers` distinct
voices is refused with the numbers, and the session plan rotates voices so consecutive trials
never share one); identification, not same/different; **no contrast is ever complete** — every
contrast stays on FSRS maintenance with a positive selection weight forever. The played member
is held server-side (ADR-008 applies to trials too).

### 5.9 The daily ledger and the streak — `internal/stats`

`daily_log` rows are written at interaction time; the streak is computed from them with one
rule ([R14]): a day counts when the due queue was cleared AND the stated minutes were reached.
Loop 4's attestation is stored in the same row and deliberately ignored by the streak — it is
signal for the checkpoint diagnostic only. Checkpoints gate the curriculum: a failed
assessment locks its CEFR level and above (resolved in `drill.Resolve`, reason
`gate.locked.checkpoint`) until a retake passes.

### 5.10 Morpheme-aware diff — `internal/morph/diff.go`

Segment the learner's answer and the target, align morphemes, classify the divergence:
`harmony | case | definiteness | possessor | vowel_length | agreement | spelling`. The **class**,
not the mere fact of being wrong, drives what gets drilled next.

### 5.11 The guided course — `internal/course`

Twenty-four units over six months, shipped in the pack: each names the errand it buys ("I can
register at a GP"), the grammar topics it leans on, and the words to own. It answers the
question the DAG and the miner both leave open — *what do I do today* — for as long as a
beginner needs it answered, and then stops.

Two properties are load-bearing.

**Completion is derived, never stored.** `Syllabus.Resolve` takes an `inPlay(lemma)` predicate
and a unit is `done` when every one of its words is in that learner's `lemma_state` as
`learning` or `known`. `course_progress` records enrolment and the counts an enrolment produced;
there is no completion column, and adding one would let the plan screen disagree with the deck.
The join lives in `api.Server.inPlaySet` because the two tables are in two databases.

**The hand-off is the design.** `plan.handedOver` goes true when the last unit is done, and the
app stops prescribing vocabulary in favour of the pack's `resource` catalogue. A course that
never ends produces a learner who can only read material made for them
([R11](../.claude/research/11-extensive-reading.md)).

Enrolling a unit builds one **sentence** note per word — shortest attested sentence containing
that lemma, so a first encounter is not also a parsing exercise — and marks the lemma
`learning`. A word the pack has no attested sentence for is **skipped and named**, not turned
into a bare-word card ([R16 §8](../.claude/research/16-debunked.md)).

---

## 6. Request flows worth knowing

**Answering a review card**
```
POST /api/review/answer
  → srs.Grade(card, rating)                    // FSRS-6 next state
  → morph.Diff(answer, target)                 // if typed
  → srs.EmitFeatureReviews(item.requiredFeats) // second state space
  → db tx: card + review_log + morph_state[]   // one transaction, all or nothing
  → drill.MaybeEnqueueFromError(errorClass)
  → respond with next card prefetched
```

**Ingesting a text**
```
POST /api/texts
  → mining.Extract(source)          // readability / srt / epub parse
  → morph.Tokenize + Lookup         // pack lookup, sidecar if available
  → persist text + text_token
  → stats.Coverage(text, user)      // lemma + element
  → mining.Score(sentences)         // n+1 rail
```

**Answering a drill item**
```
POST /api/drill/answer  {itemId, answer, durationMs}
  → drill.Pending.Take(itemID)          // the server held the expected answer
  → morph.DiffForms | option compare    // typed vs choice
  → srs.RequiredFeatures(item.feats)    // only what the item tested
  → drill.Fold(progress, servedPhase)   // the window the answer belongs in
  → drill.Evaluate(progress, gate)      // pure: the phase and the reason for it
  → db tx: feature_review[] + morph_state[] + drill_attempt + topic_progress
  → drill.FromError(class, feature)     // what to practise next, or "a drill won't fix this"
```

**Capability probing**
```
GET /api/capabilities
  → parallel HEAD/health to nlp, tts, asr, llm endpoints, 2s timeout
  → {name, configured, reachable, latency_ms, detail}
  → cached 30s; UI renders capability dots and degrades features
```

---

## 7. Where to make a change

| I want to… | Go here |
|---|---|
| Change how intervals are computed | You don't. `internal/srs/scheduler.go` wraps FSRS unmodified — see [ADR-003](../.claude/DECISIONS.md#adr-003). Storage must round-trip the WHOLE `CardState`, `remaining_steps` included: dropping a field re-breaks the learning steps ([ADR-020](../.claude/DECISIONS.md#adr-020)) |
| Change what a review number means | `internal/srs/breakdown.go` — the ONE source every screen reads ([ADR-021](../.claude/DECISIONS.md#adr-021)). Never hand a screen a raw card count |
| Split or filter the review queue | `internal/srs/scope.go` (the `Scope` filter), `queue.go`, `handlers_review.go` — and remember the daily new cap is GLOBAL. A per-text slice goes through `note_origin` and `text:<id>` ([ADR-022](../.claude/DECISIONS.md#adr-022)) |
| Hide a word from the queue without deleting it | `internal/srs/trash.go` + `handlers_trash.go`. `Scope.filter()` already excludes `note.trashed_at` for every normal scope, and `srs.NotTrashed` is for the queries that build their own `WHERE`. Practice may draw from `scope=trash`; the review queue must not ([ADR-025](../.claude/DECISIONS.md#adr-025)) |
| Change the ORDER cards are served in | `spreadSentences` in `internal/srs/queue.go`, applied after every trim so it changes the order and never the selection. A queue with no repeated sentence must come back untouched. The learner's `order=random` is `shuffleDayStable` — a per-card hash, stable for the day, applied after the cap and before the spread ([ADR-026](../.claude/DECISIONS.md#adr-026)) |
| Change what a card says about its own memory | `internal/srs/memory.go` (`Memory`, `StageOf`, `AttachMemory`). Figures are computed by the library or read from `review_log`; add nothing that goes up on its own ([R18](../.claude/research/18-sessions-and-progress.md)) |
| Change the size of a review set, or what ends one | `web-ui/src/lib/sets.ts` and `stores/review.ts`. Sets are the client's presentation of the served queue; the server, `Breakdowns` and the streak know nothing about them ([ADR-026](../.claude/DECISIONS.md#adr-026)) |
| Change what the learner chooses before a sitting — pile, set size, order, a practice round's shape | `web-ui/src/views/ReviewStartView.vue` (`/review`). The card at `/review/session` takes its intent from the query (`?scope=`, `?practice=`) and starts nothing on its own ([ADR-028](../.claude/DECISIONS.md#adr-028)) |
| Change what the grammar tutor is told, reads or may do | `internal/tutor` — `prompts/` (the system text), `prompt.go` (order and budgets), `retrieve.go`, `infer.go` (which lesson a free chat's question is about), `store.go` (threads), `kb/` (the corpus, `make tutor-kb` for the generated half); `handlers_tutor.go` gathers the input and builds the topic index. It must keep explaining and never minting — [`TUTOR.md`](TUTOR.md), [ADR-027](../.claude/DECISIONS.md#adr-027), [ADR-029](../.claude/DECISIONS.md#adr-029). The chat's controls are [ADR-031](../.claude/DECISIONS.md#adr-031): `POST /api/tutor/:id/stop` cancels the flight's context, `replace` on a question truncates and re-asks, and every answer stores the stream's `usage` plus the window Ollama reports on `/api/ps` (`prompt_tokens`, `answer_tokens`, `context_tokens`) for the context meter |
| Add a way to answer a card that must not schedule | `internal/api/handlers_practice.go` + `internal/srs/practice.go` — and never `review_log`, which replay depends on |
| Judge a text's difficulty | `internal/mining/level.go` (`EstimateLevel`, pure) + `levelInputs` in `handlers_texts.go`; the LLM second opinion and manual override in `handlers_level.go` ([ADR-023](../.claude/DECISIONS.md#adr-023)). The band table is the pack's `lemma.cefr` — do not fork a third one |
| Browse the learner's words | `internal/api/handlers_words.go` — status from `lemma_state`, memory figures derived from the CARDS (lemma_state's FSRS columns are dormant) |
| Call the configured language model | `internal/llm` (OpenAI-compatible chat, key via the `llmKey` secret; `Chat` for one completion, `Stream` for a streamed one). Prompts live versioned in `.claude/prompts/` — or, for the tutor, embedded from `internal/tutor/prompts/` with the note in `.claude/prompts/` beside it — never as ad-hoc Go strings |
| Change what a typed answer is graded against | `expectedAnswer` in `internal/api/handlers_review.go`, and [R12] |
| Change what gets scheduled | `internal/srs/directions.go`, `emit.go` |
| Change how sentences are chosen | `internal/mining/score.go` + `mining:` in `config.yaml` |
| Add a grammar topic | `langpack/pipeline/curriculum.py`, then rebuild the pack |
| Change the blocked→interleaved gate | `internal/drill/schedule.go` + `drill:` in `config.yaml` |
| Change what unlocks what | `langpack/pipeline/curriculum.py` (the DAG) + `internal/drill/resolve.go` (how it is walked) |
| Add an exercise type | a generator in `internal/drill/generate.go`, a `Kind`, and a branch in `web-ui/src/components/grammar/DrillItem.vue` |
| Find the surface form of a paradigm cell | `internal/morph/forms.go` — **never** the pack's `paradigm` table, which is generator-only |
| Add a minimal pair | `langpack/pipeline/minimal_pairs.py`, then `python -m pipeline.minimal_pairs --pack … --media … --voices-dir …` to render its audio |
| Change what counts as a streak day | `internal/stats/streak.go` (`Day.Met`) |
| Add a checkpoint criterion | `internal/stats/checkpoint.go` (`Specs`) + both locale files |
| Change a course unit's words or its goal | `langpack/pipeline/course_mN.py` (one module per month), then rebuild the pack. Never in Go — the syllabus is pack content |
| Change what makes a unit done, or what enrolment creates | `internal/course/course.go` (`Resolve`) and `internal/api/handlers_course.go` (`noteForWord`). Completion stays **derived from the deck** |
| Add or drop a hand-off resource | `langpack/pipeline/resources.py`, then rebuild the pack |
| Change what the binary ships with | `langpack/pipeline/starter.py` + `make starter`; the artefact is embedded by `internal/lang/starter.go` |
| Add an API endpoint | `internal/api/handlers_*.go`, DTO in `dto.go`, mirror in `web-ui/src/types/`, document in [`API.md`](API.md) |
| Change how sentences are translated | `services/mt/app.py` (the model) and `internal/mt` (the client). Add a target language in `internal/mt`'s `flores` map **and** the sidecar's `SUPPORTED_TARGETS` |
| Change what the translator's status row says | `services/mt/app.py` (`_report`), `internal/mt`'s `Status`, `handlers_mt.go`, and the block in `SettingsView.vue`. The capability probe is deliberately **not** involved: it flattens a downloading service's 503 into "unreachable" ([ADR-015](../.claude/DECISIONS.md#adr-015)) |
| Change the learner's own dictionary | `handlers_glosses.go` + migration `00009_user_gloss.sql`. `source` must survive the round trip: a machine gloss stays labelled and stays off cards until accepted ([ADR-014](../.claude/DECISIONS.md#adr-014)) |
| Name a morphological feature | `langpack/pipeline/features.py`, then `make refresh-curriculum` **and** `make starter`. Never in the UI — a feature label is a fact about the language and versions with the pack. Only the FAMILY names (`Падежи`, `Время`) live in `vue-i18n`, because they name a category rather than a feature |
| Add an optional service | a `Service` field in `internal/config`, a line in `clone.go`, the probe map in `handlers_capabilities.go`, `serviceField`, `settingsDTO`, `SERVICE_NAMES` in `SettingsView.vue`, and `capabilities.<name>` in **both** locales |
| Change the schema | new `internal/db/migrations/NNNNN_name.sql`; never edit an applied one |
| Add a language | out of scope for v1 — the pack schema is language-agnostic but the curriculum and minimal pairs are not |

---

## 8. Frontend

Vue 3 `<script setup>`, Pinia, vue-router, Tailwind v4, shadcn-vue (Reka UI), `@lucide/vue`
(**not** `lucide-vue-next`), `vue-i18n`, `vite-plugin-pwa`.

- Stores: `auth`, `settings`, `ui`, `capabilities`, `review`, `curriculum`, `milestone`,
  `deck`, `features`, `speech`. The review store deals the served queue into **sets**
  (`lib/sets.ts` holds the per-device size and order) and fetches the list again at every set
  boundary — that refresh, not a client-side retry list, is how a missed card comes back once
  the `nextDue` the server returned has elapsed.
  Everything else fetches through `src/api/index.ts` inside the view that needs it — a store
  exists where state outlives a screen, not as a reflex.
- The grammar screen's keyboard map lives in `components/grammar/DrillItem.vue` on a **window**
  listener. A choice item focuses nothing — focusing an answer button would visually nominate an
  answer — so a handler bound to the component root never fires.
- `src/types/` mirrors `backend/internal/api/dto.go` (camelCase). Keep them in sync when shapes
  change.
- Router guards gate setup / login / role — **UX only**. The API enforces roles independently.
- Offline: **planned, not shipped.** Nothing is precached in v1.0; the review queue needs the
  server (UX.md §13).

Screens, keyboard map and colour semantics: [`UX.md`](UX.md).
