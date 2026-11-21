# R26 — Pictures and matching in grammar drills

Written 2026-11-21 for ADR-088 and ADR-089. It adds **no study of its own**; it applies [R1](01-retrieval-practice.md), [R16](16-debunked.md) and [R25](25-exercise-variability.md). Hungarian-specific evidence on pictures or matching is absent; what follows is general L2 research, mostly on English and mostly on concrete nouns.

## Claims and how far they carry

### 1. Pictures plus words can help (dual coding) — confidence: low-medium

Dual coding is discussed in [R16](16-debunked.md) as a theory that is often over-sold. A 2025 meta-analysis (Zhang, Sala & Gobet) found that pictures plus L1 translation only marginally beat pictures alone (g = 0.271, p = 0.067: not conventionally significant). We read this as "pictures are not harmful and may help vocabulary", not as support for pictures in grammar.

### 2. Learners get overconfident with pictures — confidence: low

We know this from a single secondary source. We act on it only by never letting a picture define correctness and by keeping the answer a typed-form recognition graded by the pack.

### 3. Picture research is mostly about concrete nouns — confidence: medium

Locative relations (in/on, into/out of) are concrete and drawable; most cases are not. That is why `scene` covers six cases only: Ine/Ill/Ela and Sup/Sbl/Del. Ade/All/Abl are excluded because "at" is not drawable apart from `mellett`, and postpositions take the nominative.

### 4. Emoji differ by platform — confidence: medium

Miller et al. (ICWSM 2016) found misreadings of the same emoji across platforms. Hence embedded SVGs, not system emoji fonts.

### 5. Matching must not be solvable without the grammar — confidence: high (reasoning, not a finding)

A singular-to-plural pairing is solved by stem matching; a lemma-to-translation pairing trains the lemma space. Only cue -> form pairs of one lemma (pronoun or question word -> attested form) test a feature. Each feature is graded once per item, as in [R25](25-exercise-variability.md) claim 5 and [R3](03-fsrs.md).

### 6. Coverage is small — confidence: measured

On the data pack 22 of the 300 most frequent nouns (7.3%) have a picture by gloss match; this is a lower bound (top-frequency nouns are abstract). Pictures therefore stay a grammar cue, not a vocabulary system.

## What this changes in our code

- `internal/pictures` (embedded SVG, catalogue, sanitiser) and `GET /api/pictures/:file`; `internal/drill/scene.go`, `match.go`, bare mode in `person.go`.
- New kinds `scene` and `match`; `match` grades each feature once, never Person/Number.
- Noto Emoji (Apache-2.0) in `NOTICE` and `THIRD_PARTY_LICENSES/`; credits in Settings.
- A picture never defines the expected answer; the form comes from `internal/morph`.
- Not built: picture cue on word cards, picture dictation, audio, typed paradigm.
