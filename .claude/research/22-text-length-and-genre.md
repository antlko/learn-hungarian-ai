# R22 — Three texts per topic: length and genre as a narrow-reading cluster

Written 2026-11-12 for ADR-075 (every basic unit gets a short, a medium and a long authored text of three
different genres). It records the reasoning behind a design change and cites only notes in this library
([R6](06-lexical-coverage.md), [R11](11-extensive-reading.md), [R21](21-graded-authored-texts.md)). No new
paper was read for it; each claim says what supports it.

## Claims and evidence

### 1. Coverage decides whether a text can be read, and it is measured in running words — confidence: high for English, see [R6](06-lexical-coverage.md)

Nation's English figures: about 95% coverage for comprehension with support, about 98% for unassisted reading.
The figures are English, not Hungarian; Hungarian is agglutinative, so a "word" is a wider bundle and the numbers
can only be a direction. Our gate (`MIN_TOP_SHARE` over frequency-ranked lemmas, the unit's own words exempt)
is the same idea at a lower, tunable threshold ([R21](21-graded-authored-texts.md)).

### 2. Volume matters; beginners need something to read at all — confidence: medium-high, see [R11](11-extensive-reading.md)

Jeon & Day (2016), the extensive-reading meta-analysis summarised in [R11](11-extensive-reading.md): reading
volume improves reading ability and vocabulary when the text is easy. Three texts per unit triple what a
learner can read at a unit's level without leaving it, and a longer text is the first step toward volume.

### 3. Repetition: a word has to come back several times — confidence: medium, SECONDARY SOURCE

Webb (2007) is quoted in the reading literature for "about 10 encounters" before a word is reliably learned
from text; we have not read the paper itself, so the number is a secondary-source figure. We use only the
direction: a unit's words should recur, so the build reports **recycling** (the share of a unit's words that
occur at least 3 times across its three texts, target 70%) as a report-only metric, not a gate.

### 4. Narrow reading and genre variety — confidence: medium (design inference)

Narrow reading (several texts on one topic) keeps vocabulary stable while the learner meets it in different
syntax and register; a note, a conversation and a story about the same topic do that. Different genres also
differ in register, so the learner meets the topic in the forms real Hungarian takes (an SMS, a notice). This
is an inference from [R11](11-extensive-reading.md), not an experiment of ours.

### 5. Reading while listening — confidence: medium, see [R11](11-extensive-reading.md)

A text read with its audio (TTS sidecar, optional) supports reading fluency and decoding; the reader already
plays audio per sentence, so longer texts get more of it for free.

### 6. The tier word-count table is an INFERENCE

Short 30-130, medium 90-300, long 180-480 words by band (A1: 30-70 / 90-160 / 180-280; A2 months 3-4:
50-100 / 140-220 / 250-380; A2 months 5-6 and themes: 70-130 / 180-300 / 320-480) with at most 10, 12 or 14
words a sentence. These are not published norms: they come from what a beginner can finish in one sitting and
from the 8-14-sentence stories of R21 scaled up. They sit in `pipeline/texts.py` (`RANGES`, `MAX_SENTENCE`,
`PARAS`) so they can be retuned after real texts exist.

## What this changes in our code

- `langpack/pipeline/texts.py`: three texts per unit from `langpack/texts/<unit>.json`, with the checks V1-V6
  (token rule, band gate, frequency gate, size by band, three distinct genres, ru+en per sentence); the
  recycling and generated-form counts are reported, never gating.
- `reading_set` gains `tier`, `genre`, `words`, `checked` (guarded `ALTER`); the set codes stay the old
  «Диалог»/«Рассказ» codes plus `<unit>-long`, so imports sync in place.
- `--brief UNIT --tier T` prints an author's brief so a human or a neural net can write one text at a time.
- `make starter` (`--require-all`) refuses a unit without a full valid triplet; legacy text is not accepted.
- INVARIANTS / CLAUDE.md: reading-set texts remain the one AI-written Hungarian in the pack, provenance in
  data, no UI badge (ADR-075).
- Open: a native speaker's read; retuning the size table and recycling target on real texts.
