# R25 — Exercise variety and repetition: what a topic's drills should rotate

Written 2026-11-21 for ADR-086. It adds no new study to the library and **cites no figure of its own**: it applies [R1](01-retrieval-practice.md), [R2](02-spacing.md), [R5](05-interleaving.md), [R9](09-corrective-feedback.md), [R15](15-hungarian-difficulty.md) and [R16](16-debunked.md). Where a claim rests on general knowledge of the field rather than a note in this library it says so. Hungarian-specific evidence on drill variety is thin; nearly all of it is about English or other L2s.

## Claims and how far they carry

### 1. Repeating the same few items trains the items, not the rule — confidence: medium

Drilling a small fixed set lets a learner answer from memory of the item ("`ház` -> `házban`") instead of from the rule. This is the usual distinction between item learning and rule learning in the skill-acquisition literature; we state it as a well-known caution, not as a measured effect size. It is the failure the user saw: practising a topic again returned the same words.

### 2. Varying surface form and context while the target stays fixed is a "desirable difficulty" — confidence: medium

Bjork's desirable-difficulties framing (variation, spacing, interleaving, retrieval) says conditions that slow initial performance can improve transfer. [R1](01-retrieval-practice.md) and [R2](02-spacing.md) are the high-confidence parts; the claim that varying *item type* per se helps grammar is weaker, and we do not rely on it for scheduling. We use variety to avoid item memorisation and boredom, and keep the rule constant.

### 3. Interleaving helps only after a competence floor — confidence: high (as already adopted)

[R5](05-interleaving.md): the blocked-then-interleaved gate stays. Rotating *kinds* inside the blocked phase is not interleaving of topics: the topic and its features stay fixed, only the way of asking changes. Mixing kinds inside a session is allowed only in the interleaved phase, where the 80% gate has already passed.

### 4. Production and recognition are different skills — confidence: high

[R1](01-retrieval-practice.md) and [R12](12-retrieval-direction.md): you get back the direction you trained, and recall beats recognition. So typed kinds (`person`) are worth having but are not served until a learner has answered enough blocked items correctly in choice form (>= 10); choice kinds (`qword`, `sort`) stay recognition and are rated at most Good, as every tap already is ([ADR-060](../DECISIONS.md)).

### 5. A multi-part item must not count one fact several times — confidence: high (arithmetic, not a finding)

A `sort` item places six words into suffix columns. Six separate FSRS reviews of the same feature in the same second would shrink its stability estimate for reasons unrelated to memory. Each feature is graded once per item: Again if any tile carrying it is wrong, otherwise Good at most. [R3](03-fsrs.md) assumes one review is one retrieval.

### 6. Word order, true/false and free writing do not test our features — confidence: medium

Hungarian word order is free and marks focus ([R15](15-hungarian-difficulty.md)); marking one order wrong would be dishonest and tests no morphological feature. True/false is reading comprehension, not grammar. Free writing exists in «Письмо» ([R24](24-written-corrective-feedback.md)). Model-made items are excluded by [R16](16-debunked.md) and [ADR-072](../DECISIONS.md).

### 7. External reference: the MagyarOK A1 exercise taxonomy — confidence: n/a

A popular A1 textbook uses gap-fill with the lemma in brackets, question-word choice, sorting into ending columns, paradigm tables, multi-gap texts and matching. We use the **taxonomy of types** as a checklist only. The book's content is copyrighted; every item here is generated from attested pack sentences. We have not verified the book's pedagogy; it is a reference for variety, not evidence of effect.

## What this changes in our code

- `internal/drill`: an anti-repeat **filter** built from `drill_attempt` history per user and topic (reject exact keys, used sentences and lemmas used >= 2; relax in three stages; never below `Count`). It is not a scheduler and does not touch weak-feature oversampling or `schedule.go`.
- Kind rotation: blocked = one least-recently-used kind per session; typed kinds after >= 10 correct blocked attempts; interleaved weights toward least-recently-used kinds.
- New kinds `person`, `qword`, `sort`; `cloze` shows the lemma in brackets after the gap. `sort` grades each feature once.
- Migration `00025` rebuilds `drill_attempt` (`item_key`, `corpus_sentence`; kind CHECK dropped).
- `GET /api/grammar/topics/:code/examples`: attested example sentences in the lesson.
- Not built (phase 2): paradigm, multi-gap, match.
