# Pedagogy

How every learning feature in this app is justified, and where we deliberately depart from the
brief. If you are about to change how something teaches, read this first.

Evidence lives in [`.claude/research/`](../.claude/research/README.md). This document maps
**features → notes**; the notes carry the effect sizes and caveats.

---

## 0. The warning that comes first

From [`research/00-source-plan.md`](../.claude/research/00-source-plan.md) §7.1, quoted because
it applies to everyone who works on this codebase — including whoever is reading this now:

> Айтишник, который вместо изучения языка пишет приложение для изучения языка, — это не шутка,
> а массовое явление. Программирование даёт немедленное дофаминовое подкрепление, изучение
> венгерского — нет. Через полгода у вас будет красивый репозиторий и уровень A1.

The product's answer to this: the home screen is the **study plan**, not a dashboard of the
app's own features; Loop 4 (a human teacher) is the one thing the app structurally refuses to
simulate; and the checkpoints are real assessments that can be failed.

Building the app is not studying. The app should say so, and it does.

---

## 1. Feature → evidence map

| Feature | Where | Basis |
|---|---|---|
| Spaced scheduling exists at all | `internal/srs` | [R2](../.claude/research/02-spacing.md) |
| FSRS-6 specifically | `internal/srs/scheduler.go` | [R3](../.claude/research/03-fsrs.md) |
| Everything is a retrieval attempt; no passive "study mode" | all of `internal/drill`, `ReviewView` | [R1](../.claude/research/01-retrieval-practice.md) |
| Immediate feedback on every answer | `ReviewView.vue` | [R1](../.claude/research/01-retrieval-practice.md) |
| Five card directions, independently scheduled | `internal/srs/directions.go` | [R12](../.claude/research/12-retrieval-direction.md) |
| `spell` direction is on by default | `internal/srs/directions.go` | [R12](../.claude/research/12-retrieval-direction.md), [R15](../.claude/research/15-hungarian-difficulty.md) |
| Rule card before drills | `GrammarView.vue`, `grammar_topic.explain_md` | [R4](../.claude/research/04-explicit-instruction.md) |
| **Blocked → interleaved gate** | `internal/drill/schedule.go` | [R5](../.claude/research/05-interleaving.md) |
| Interleaved items are *confusable*, not merely varied | `internal/drill/generate.go` | [R5](../.claude/research/05-interleaving.md) |
| Dual coverage metrics (lemma + element) | `internal/mining/coverage.go` | [R6](../.claude/research/06-lexical-coverage.md) |
| `n+1` selection targets 1–2 unknown elements at ≥90% coverage | `internal/mining/score.go` | [R6](../.claude/research/06-lexical-coverage.md) |
| Reader optimised for volume, not lookup | `ReaderView.vue` | [R11](../.claude/research/11-extensive-reading.md) |
| Adult content, difficulty controlled by selection not infantilisation | `internal/mining` | [R11](../.claude/research/11-extensive-reading.md) |
| Multi-talker HVPT, `hvpt.minTalkers` enforced; refusal carries the numbers | `internal/hvpt/hvpt.go` `SessionPlan` (ships **3**, not the evidence's 5 — §2.7) | [R7](../.claude/research/07-hvpt.md) |
| Consecutive trials rotate talkers by construction | `internal/hvpt/hvpt.go` | [R7](../.claude/research/07-hvpt.md) |
| Contrasts get FSRS maintenance forever; selection weight never zero | `internal/hvpt/hvpt.go` `contrastWeights` | [R7](../.claude/research/07-hvpt.md) |
| Listening hours accumulate from playback, cannot be claimed | `handlers_listen.go` (60 s clamp) | [R15](../.claude/research/15-hungarian-difficulty.md) |
| Pronunciation feedback names the phoneme *(v1.1, not yet built)* | `internal/speech/feedback.go` | [R8](../.claude/research/08-capt-asr.md) |
| Segments → ASR; prosody → shadowing *(SpeakView is v1.1)* | `SpeakView` / `ListenView.vue` | [R8](../.claude/research/08-capt-asr.md), [R13](../.claude/research/13-shadowing.md) |
| Three-pass listening protocol as UI state | `ListenView.vue` | [R13](../.claude/research/13-shadowing.md) |
| Correction is prompt-then-reveal, enforced server-side *(v1.1, not yet built)* | `handlers_write.go`, `prompts/correct-writing.md` | [R9](../.claude/research/09-corrective-feedback.md) |
| Errors generate targeted drills | `internal/drill/from_error.go` | [R9](../.claude/research/09-corrective-feedback.md) |
| Voice-first roleplay *(v1.1, not yet built)* | `internal/speech`, `prompts/roleplay-system.md` | [R10](../.claude/research/10-llm-tutors.md) |
| Corpus attestation guardrail *(v1.1; the pack ships the `ngram` index already)* | `internal/corpus/attest.go` | [R10](../.claude/research/10-llm-tutors.md) |
| Streak = obligations met, not activity | `internal/stats/streak.go` `Day.Met` | [R14](../.claude/research/14-motivation.md) |
| Loop 4 attestation ignored by the streak | `daily_log.human`, `ComputeStreak` | [R14](../.claude/research/14-motivation.md), [R16](../.claude/research/16-debunked.md) |
| Checkpoints: measured vs self-attested criteria labelled apart | `internal/stats/checkpoint.go`, `ProgressView.vue` | [R14](../.claude/research/14-motivation.md) |
| Failed checkpoint locks levels; diagnostic = the three failure modes | `stats.BlockedLevels`, `stats.Diagnose` | [R14](../.claude/research/14-motivation.md) |
| The booked exam as a first-class object with a countdown | `curriculum.examDate`, `TodayView.vue` | [R14](../.claude/research/14-motivation.md) |
| Morpheme breakdown names the feature and the allomorph rule | `internal/morph/segment.go`, `explain.go` | [R4](../.claude/research/04-explicit-instruction.md) |
| No XP / leagues / gems | *absence* | [R14](../.claude/research/14-motivation.md), [R16](../.claude/research/16-debunked.md) |
| Checkpoints block advance on failure | `internal/stats/checkpoint.go` | [R14](../.claude/research/14-motivation.md) |
| Curriculum ordering | `langpack` `grammar_topic.teach_order` | [R15](../.claude/research/15-hungarian-difficulty.md) |
| Harmony mastery measured by **latency** | `internal/drill/harmony.go` | [R15](../.claude/research/15-hungarian-difficulty.md) |
| Definiteness drills weighted 55/25 | `internal/drill/definiteness.go` | [R15](../.claude/research/15-hungarian-difficulty.md) |
| Focus/word-order locked behind listening volume | `internal/curriculum/dag.go` (the flag), `internal/drill/schedule.go` (the gate) | [R15](../.claude/research/15-hungarian-difficulty.md) |
| Prerequisites are met at `interleaved`, not `mastered` | `internal/drill/resolve.go` | [R5](../.claude/research/05-interleaving.md) |
| A topic with no exercises says so instead of blocking the curriculum | `Phase.Explained`, `internal/drill/schedule.go` | [R4](../.claude/research/04-explicit-instruction.md) |
| Placement never seeds "mastered", and seeds carry their own observation count | `internal/curriculum/placement.go` | [R3](../.claude/research/03-fsrs.md) |
| A vowel-length error routes to the ear, not to more typing | `internal/drill/from_error.go` | [R8](../.claude/research/08-capt-asr.md), [R15](../.claude/research/15-hungarian-difficulty.md) |
| Drill answers are real retrievals and enter the morphological state space | `internal/srs/review.go` `GradeFeatures` | [R1](../.claude/research/01-retrieval-practice.md) |
| Loop 4 un-completable by the app | `TodayView.vue` | [R16](../.claude/research/16-debunked.md) §7 |
| The day's queue dealt in sets; a miss returns at the set's end once its step has elapsed | `stores/review.ts`, `lib/sets.ts` | [R18](../.claude/research/18-sessions-and-progress.md) §1–§2 |
| A card shows its measured memory — recall chance, stage, history, what each grade schedules — and never a score | `internal/srs/memory.go`, `MemoryMeter.vue` | [R18](../.claude/research/18-sessions-and-progress.md) §4–§5, [R14](../.claude/research/14-motivation.md) |
| The set summary is informational feedback: measured figures, no praise | `SetSummary.vue` | [R18](../.claude/research/18-sessions-and-progress.md) §4 |
| «что это значит?» explains what makes a word stop coming back — the stage, what each answer schedules, that the tally is a log, and that «В корзину» is the one control that removes a word — with no completion bar, because there is no denominator | `MemoryMeter.vue` | [R18](../.claude/research/18-sessions-and-progress.md) §4–§5, [R14](../.claude/research/14-motivation.md), [R16 §3](../.claude/research/16-debunked.md) |
| The grammar tutor explains from the lesson, grounded, short-context, labelled; mints nothing | `internal/tutor`, `TutorPanel.vue` | [R19](../.claude/research/19-grounded-tutor.md), [R4](../.claude/research/04-explicit-instruction.md), [R10](../.claude/research/10-llm-tutors.md) |
| The tutor sits after the lesson, never before it | `GrammarView.vue` | [R4](../.claude/research/04-explicit-instruction.md) |
| The tutor translates into Hungarian on request as explanation — the sentence, then word by word, then the learner's own attempt compared ([ADR-030](../.claude/DECISIONS.md#adr-030)) | `internal/tutor/prompts/`, rule 4 | [R4](../.claude/research/04-explicit-instruction.md), [R19](../.claude/research/19-grounded-tutor.md) |
| The tutor is one key away from the review card, with the question about the card already written ([ADR-032](../.claude/DECISIONS.md#adr-032)) | `components/review/TutorAside.vue` | [R4](../.claude/research/04-explicit-instruction.md) |

---

## 2. Divergences from the source plan

The brief is excellent and mostly implemented as written. These eight points are where the
evidence says otherwise. Each one changes code.

### 2.1 Interleaving — the big one

**Source plan §2.3:** introduce definite/indefinite conjugation at month 3, then interleave
immediately, as a "desirable difficulty".

**What the evidence says.** [R5](../.claude/research/05-interleaving.md):
Brunmair & Richter (2019, 59 studies) put interleaving at **g = 0.42** overall — and
**specifically attenuated for explicit, rule-based and language-learning material**. The large
effects come from visual category learning. Hwang (2025, *Language Learning*) titles the paper
*"Undesirable Difficulty of Interleaved Practice"* and finds **initial blocked practice is
necessary** for declarative knowledge in learners below a competence threshold.

Definite/indefinite conjugation at month 3 is precisely the worst case: rule-based, explicit,
early-stage, and with no L1 analogue for a Russian or English speaker.

**What we do instead.** `internal/drill/schedule.go` gates the transition:

```
locked      → prerequisites unmet
blocked     → rolling accuracy (window 20) < 0.80
interleaved → rolling accuracy ≥ 0.80
mastered    → interleaved accuracy (window 50) ≥ 0.90
```

Falls back to `blocked` if accuracy drops below `0.80 − hysteresis(0.10)`. Thresholds and
windows are in `config.yaml` under `drill:`. **The phase is shown to the learner** with a
plain-language explanation — we are holding interleaving back on purpose, and saying so.

When interleaving does start, it mixes *confusable* items (definite vs indefinite of the same
verb; `-ban` vs `-ba` on the same noun), because discriminability is the mechanism.

### 2.2 HVPT durability

**Source plan §1.1:** HVPT has "подтверждённым долгосрочным удержанием".

**What the evidence says.** [R7](../.claude/research/07-hvpt.md): the immediate effect is robust
(g = 0.92 pre/post across 79 studies) but syntheses report **effects diminishing at one- to
two-month follow-up**. Durability is the method's weak point, not its strength.

**What we do instead.** No contrast is ever marked complete. Every contrast carries FSRS state
and receives maintenance trials indefinitely (`internal/hvpt/store.go`). There is no "done"
badge in the Ear view, by design.

### 2.3 Coverage thresholds

**Source plan §2.6** already flags this correctly. We implement the flag rather than the number:
95/98% are **English word-family** figures and an English word family is nothing like a
Hungarian lemma. We compute **lemma coverage** (comparable to the literature) and **element
coverage** (lemma *and* every feature on the token known — the honest number), show both, and
label the threshold lines *"English-derived, indicative only"* in both locales.

`n+1` selection uses element coverage. See [R6](../.claude/research/06-lexical-coverage.md).

### 2.4 Pronunciation feedback must be explicit

**Source plan §7.4 (v2):** "запись голоса + whisper-large-v3-hu, diff произношения".

**What the evidence says.** [R8](../.claude/research/08-capt-asr.md): ASR training overall
g = 0.69, but **g = 0.86 with explicit corrective feedback** and materially less with indirect
feedback such as plain dictation.

**What we do instead.** Feedback names the phoneme, states the contrast, and enqueues the
matching HVPT pair — *"you produced `ö` where `ő` was required; the vowel is short"*. Never a
similarity score alone. And because Hungarian WER ≈ 11%, feedback is gated behind a confidence
threshold and labelled as fallible.

### 2.5 LLM correction style

**Source plan §6.1 #3** already asks for correction-plus-drills. We add the ordering constraint:
[R9](../.claude/research/09-corrective-feedback.md) finds **prompts beat recasts**, so the
learner must attempt the repair before the answer is revealed. It will be enforced
server-side in `handlers_write.go`, not merely in the UI — the writing loop is Phase 9 and
nothing of it is built yet.

### 2.6 Where the app must implement its own SRS

**Source plan §7.1 rule 3:** "Никакого своего SRS-алгоритма. Экспорт в Anki — и всё."

The *intent* — do not invent scheduling maths — is correct and we honour it: `internal/srs`
wraps `go-fsrs/v4` unmodified. But scheduling has to live in the app, because **Anki has no
representation for the morphological feature state space**, which is the entire product thesis.
See [ADR-003](../.claude/DECISIONS.md#adr-003).

The second half of that rule — "экспорт в Anki — и всё", keep `.apkg` export first-class so
nothing is locked in — **we no longer meet**. `internal/export` and its endpoints were removed;
the answer to lock-in is now that the data directory is the learner's outright. That is a
genuine divergence from a decision this document endorsed, recorded here rather than quietly
dropped: see [PLAN §10.5](../.claude/PLAN.md).

---

### 2.7 The HVPT talker floor is what we can meet, not what the evidence asks for

[R7](../.claude/research/07-hvpt.md) is unambiguous that talker variability is the active
ingredient and that five or more distinct voices is the number the training effects were
measured at. `config.EvidenceMinTalkers` records that.

`config.DefaultMinTalkers` ships **3**, and the two differ on purpose. Three is how many
freely-licensed Hungarian TTS voices exist (`hu_HU-{anna,berta,imre}`, CC0). Shipping the
evidence number would mean the ear trainer refuses every session on every fresh install — a
screen nobody can pass, which trains nothing and reads as a broken feature rather than as an
honest limit.

So the shipped floor is the one we can actually meet, and the Ear screen states the shortfall
permanently: three synthetic voices, the research wants five or more, the effect is weaker in
proportion. Rate variants are recorded as rates and **never** counted as extra talkers — that
would be the dishonest version of this fix. Adding human recordings to the pack raises the
count and the caveat goes away; an admin who has other voices can raise `hvpt.minTalkers`
themselves.

### 2.8 The app teaches a fixed syllabus before it teaches self-direction

The brief's model is sentence mining from real material from early on. The mining machinery is
built and is the long-term engine, but pointing a week-one learner at Telex and telling them to
mine is handing over a research project, not a course: `n+1` selection needs a vocabulary to
score against, and at zero known lemmas every sentence scores the same.

So the pack carries an explicit six-month syllabus — twenty-four units, each naming the errand
it buys, the grammar it leans on and the words to own — and the app prescribes until it runs
out. Then it stops and hands over to a curated resource catalogue.

The hand-off is the design, not a limitation. [R11](../.claude/research/11-extensive-reading.md)
is about volume of self-chosen input; a course that never ends produces a learner who can only
read material made for them. `internal/course` implements the prescribing half and knows how to
stop.

### 2.9 Sets are a usability decision the evidence permits, not one it demands

No study compares a 15-card sitting with a 70-card one on retention. What
[R18](../.claude/research/18-sessions-and-progress.md) supports is narrower: relearning each item
to one correct recall per sitting (Rawson & Dunlosky), a near and visible goal sustaining
effort (goal-gradient, endowed progress — consumer studies, about persistence), and measured
progress being feedback where points are rewards (Deci, Koestner & Ryan; Hattie). The set size
is therefore a per-device dial with a default of 15, not a setting with a citation behind the
number, and the note says so.

## 3. What we do not build, and why

Summarised from [R16](../.claude/research/16-debunked.md): learning styles, passive background
listening, XP/leagues/gems, a custom SRS algorithm, "fluent in 3 months" projections,
authoritative-looking AI grading, replacing the human teacher, and bare word-level flashcards.

Read the note before proposing any of them.

---

## 4. Honest limitations, stated in the product

These appear in the UI, not only here:

1. **FSRS's efficiency claim is a simulation**, not a controlled trial.
2. **Per-user FSRS optimisation is not available in v1.0** — `go-fsrs`'s optimizer is still in
   development, so everyone gets default FSRS-6 parameters.
3. **Coverage thresholds are English-derived.**
4. **Hungarian ASR has ~11% WER** — the recogniser will sometimes contradict a correct learner.
5. **LLM Hungarian is weaker than its English** — hence attestation gating.
6. **HVPT has only 3 synthetic voices available**; we reach the 5-talker floor with Common
   Voice recordings and rate jitter, and the Ear view reports how many talkers a session used.
7. **The app cannot replace a teacher**, and the Today view says so where Loop 4 lives.
