# R16 — Things we deliberately do NOT build

**Confidence: High.** Null results and failed replications are evidence too.

A feature list is defined as much by refusals as by inclusions. Each entry here is a thing a
language app would plausibly ship, and a reason we don't.

## 1. Learning styles (visual / auditory / kinaesthetic)

**Status: not supported.** Repeated failures to find the predicted interaction between a
learner's stated style preference and the modality that actually helps them. The "meshing
hypothesis" — that matching instruction to preferred style improves outcomes — is the specific
claim that fails.

**We therefore do not:** ask the learner their "style", offer style-based paths, or tag content
by modality preference.

**We do:** use multiple modalities for *everyone*, because dual coding and modality variety help
in general — that is a different claim, and it survives.

## 2. Passive background listening to incomprehensible audio

**Status: near-zero effect.** Source plan §1.2. Input has to be comprehensible to be input;
below roughly 90% coverage it is acoustic wallpaper.

**We therefore do not:** ship a "listen in the background" mode or count unattended playback
toward listening time.

**We do:** compute coverage of a transcript **before** playback and warn when it is too low
([R6](06-lexical-coverage.md)); count only foregrounded, interacted-with listening.

## 3. Gamified point economies as the core loop

**Status: motivating, not instructive.** Gamification improves autonomy and relatedness, shows
**minimal impact on competence**, and the Duolingo evidence base is methodologically weak
([R14](14-motivation.md)).

**We therefore do not:** build XP, gems, leagues, leaderboards, or lootboxes.

**We do:** keep a streak defined as *obligations met*, and show real competence signals.

## 4. A custom spaced-repetition algorithm

**Status: a solved problem; you will not beat it.** Source plan §7.1 rule 3.

**We therefore do not:** invent scheduling maths. `internal/srs/` wraps `go-fsrs/v4` (FSRS-6)
and adds only the *application* of it to a second state space.

**We do:** keep `.apkg` export first-class so the learner is never locked in.

## 5. "Fluent in 3 months"

**Status: arithmetically impossible.** FSI places Hungarian at ~1,100 class hours to
professional working proficiency. Three months of full-time study is ~500 hours at best.

**We therefore do not:** display projected fluency dates, promise levels by date, or let
marketing copy imply speed.

**We do:** show the honest arithmetic in the onboarding — hours/day → projected months to A2/B1/B2
using the source plan's own table — and let the learner set a realistic pace.

## 6. Server-side "AI grading" presented as authoritative

**Status: overconfident given Hungarian ASR/LLM quality.** WER ≈ 11% for the best open Hungarian
Whisper fine-tune; LLM Hungarian is materially weaker than its English.

**We therefore do not:** present a pronunciation score or a grammar judgement as ground truth.

**We do:** gate ASR feedback behind a confidence threshold ([R8](08-capt-asr.md)), gate LLM
output behind corpus attestation ([R10](10-llm-tutors.md)), and label both as fallible in the UI.

## 7. Replacing the human teacher

**Status: the one thing the app structurally cannot do.** Source plan §3, Loop 4.

An LLM has no social stakes, understands broken Hungarian far too well, and gives no feedback on
pauses, intonation or pace. The signal a learner needs is *a real Hungarian failing to
understand them*.

**We therefore do not:** let the app mark Loop 4 complete, or frame roleplay as equivalent
practice.

**We do:** make Loop 4 a manual tick with an explanation of why the app refuses to do it for you.

## 8. Word-level flashcards without context

**Status: contrary to the whole thesis.** For an agglutinative language, a bare lemma card
teaches nothing about the morphology that carries the meaning.

**We therefore do not:** offer a bare `ház → дом` card as a default note type.

**We do:** require a sentence context on every note (source plan §3: "Только предложения, не
слова"), and make `cloze` the highest-weighted direction ([R12](12-retrieval-direction.md)).

## What this changes in our code

Nothing in this note describes something we build — that is the point. Concretely, the
following must **not** exist in the codebase, and a change reintroducing any of them should be
rejected with a link to this note:

| Must not exist | Enforced by |
|---|---|
| A learner "style" preference, or content tagged by modality preference | absence from `settings`; no `style` field in `config.yaml` |
| A background/unattended playback mode, or listening time counted without interaction | `internal/stats` counts only foregrounded, interacted sessions |
| XP, gems, leagues, leaderboards, lootboxes | absence; `internal/stats/streak.go` counts obligations met |
| Hand-written scheduling maths | `internal/srs` wraps `go-fsrs/v4` unmodified — [ADR-003](../DECISIONS.md#adr-003) |
| Projected fluency dates or "level by date" promises | onboarding shows the hours arithmetic instead |
| An unqualified pronunciation score or grammar verdict | `internal/speech/feedback.go` confidence gate; `internal/corpus/attest.go` |
| A completion control for Loop 4 that the app can press | `TodayView.vue` — manual tick only, with the reason shown |
| A bare `lemma → gloss` note type as a default | note creation requires sentence context |

## Related

[[R14 motivation]](14-motivation.md) · [[R8]](08-capt-asr.md) · [[R10]](10-llm-tutors.md) · [[00 source plan §1.2, §7.1]](00-source-plan.md)
