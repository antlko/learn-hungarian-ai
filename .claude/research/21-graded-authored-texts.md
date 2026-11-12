# R21 — One connected, graded, authored text per unit

Written 2026-11-12 for ADR-074 (a «Рассказ» per course unit replaces the Tatoeba «Фразы» sets).
This note records the reasoning for a design change; it cites only notes in this library. No new external
paper was read for it, so every claim below is marked with what supports it.

## Claims and evidence

### 1. A text a beginner can read needs near-total coverage of its words — confidence: high (for English), see [R6](06-lexical-coverage.md)

[R6](06-lexical-coverage.md): about 95% coverage is the minimum for comprehension with support and about
98% for unassisted reading. A beginner knows a few hundred words, so a text at that coverage cannot be
found, it has to be written to a vocabulary. The Tatoeba sets could pick a sentence whose words are all in
band, but not a set whose words recur, so each line was a fresh vocabulary test.

### 2. Volume is the mechanism, and volume needs something to read from day one — confidence: high, see [R11](11-extensive-reading.md)

[R11](11-extensive-reading.md) says reading volume complements instruction and needs a vocabulary floor.
Its point 3 prefers real adult material over graded readers. That holds from B1 on; at A1–A2 no real
Hungarian text sits at 95% for a learner who owns a few hundred words. This note narrows that point: for
the 36 basic-tier units a short authored text is a bridge, labelled as AI-written, and real texts take
over through the reader and the resource list.

### 3. Connected text beats a bag of sentences because words come back — confidence: medium (design inference from R6/R11, not a primary source)

A set of 20 unrelated lines meets each unit word about once. One scene of 8–14 sentences can meet each
unit word twice or more and keep its neighbours (names, places, the same verb) in view. Repetition inside
a text is what turns the unit's words from "seen" to "recognised"; that is the practical reading of
[R11](11-extensive-reading.md)'s "volume" at A1.

### 4. Caregiver-style language is the right register for a first text — confidence: medium-low (a design choice; no study of Hungarian cited here)

What a beginner needs is what a child hears: here-and-now reference (what is in the room, who is doing
what), repetition, and short frames reused with one slot changed («Anna a konyhában van. Péter a szobában
van.»). We take these three features as writing rules. We do not claim they were shown to help adult
learners of Hungarian; they are the best available prior and the text is labelled accordingly.

### 5. A frequency gate is a stricter, checkable stand-in for «in band» — confidence: medium

`lemma.cefr` is an estimate from frequency rank ([R6](06-lexical-coverage.md)) and a dialogue can be
in band and still be strange (rare words picked for their topic). A gate on running tokens (share of
tokens whose lemma ranks in the top N) is checkable by the build and tells the author which word is
the outlier. The thresholds (500/1000/2000, 85%) are our tuning, not a published figure; they should be
revised after reading a few texts, and the report prints the numbers so that can happen.

### 6. Attestation proves a form exists, not that a sentence is idiomatic — confidence: high (what the code checks)

As for the dialogues (ADR-066): the build can prove every form is attested and every lemma is in band, not
that the sentence is natural Hungarian. The label «Написано ИИ, носителем не проверено» stays on every
text; the text feeds reading only.

## What this changes in our code

- `langpack/pipeline/stories.py` (+ `stories_m1..m6.py`, `stories_themes.py`): one authored text per basic
  unit, 8–14 sentences in 2–4 paragraphs, written as `sentence.source = 'authored'`; validation reuses
  `dialogues.check_line` plus the frequency gate (`TOP_N`, `MIN_TOP_SHARE`); `--brief` prints what an author
  (human or model) needs.
- `reading_set_sentence.para` marks the paragraph; the app joins sentences with a space and paragraphs
  with a blank line.
- `reading_sets.py` loses the Tatoeba picker, `reading_sets_exclude.txt` and the tests that only proved
  it; `validate` requires every reading set to be authored and labelled.
- INVARIANTS and CLAUDE.md digest: reading sets are the one labelled AI-written Hungarian in the pack.
- Open: a native speaker's read of the texts; threshold tuning after reading real output.
