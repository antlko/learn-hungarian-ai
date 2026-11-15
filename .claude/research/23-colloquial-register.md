# R23 — Colloquial register: spoken Hungarian beside the written norm

Written 2026-11-14 for ADR-077 (a colloquial talk text and mini dialogues per unit). It records what we could
and could not establish. It cites only notes of this library ([R6](06-lexical-coverage.md),
[R11](11-extensive-reading.md), [R22](22-text-length-and-genre.md)); the colloquial sources are web pages and
papers collected for the author's guide, with their confidence marked in the working notes
(`.agent-context/COLLOQUIAL_*.md`, not part of the library). **The evidence is thin; everything authored from
it is AI-written and needs a native speaker's review.**

## Claims and evidence

### 1. Spoken Hungarian uses discourse particles the textbook norm omits — confidence: medium

A Szeged corpus summary lists *hát, ugye, szóval, úgyhogy, persze, egyébként* as the most frequent markers; a
study of the particle *hát* (Alberti et al. 2022) is read in full. The remaining particle functions (*na, jaj,
jé, hopp, naná, mondjuk, amúgy*) rest on dictionary entries and the researcher's own knowledge (LOW): the
words are real and common, the exact usage nuance is not verified.

### 2. Clippings, contractions and ellipsis are normal in speech and chat — confidence: low to medium

*suli, kaja, buli, szülinap, ovi, cuki, nemtom, kéne* are attested in school-slang theses and a Telex piece on
*nem tudom/nemtom* (MED); pro-drop and fragment answers are standard descriptions of Hungarian. Which of them
a beginner may be shown at which level is **our judgement** (the A1/A2/B1 column of `texts_colloquial.tsv`).

### 3. Address and greeting register must match the setting — confidence: medium to high

Vecsernyés-Kovács (2022) and a school survey describe the *szia / jó napot / csókolom* split by age and
distance. A text keeps one address form per scene; peer-only forms (*csá, mizu*) are B1 and never go to a
stranger.

### 4. Easy to read means short and recycled, not slangy — confidence: medium, inference from R6, R11, R22

[R6](06-lexical-coverage.md) coverage and [R11](11-extensive-reading.md) extensive reading say a text must be
mostly known words; so colloquial items are few and level-gated (an A1 text uses only A1 items), and a text is
rejected when more than 20% of its words are markers. The 20% cap, the minimum of one marker and the mini sizes
(2-10 turns) are **design inferences with no source**, tunable after reading real output.

### 5. What we could not establish

No Hungarian-specific study of which spoken forms beginners should meet first; no native review of our
lexicon; the "do not use" list (vulgar words, slurs) is a safety choice, not a finding.

## What this changes in our code

- `pipeline/texts_colloquial.tsv`: a level-gated `[allowed]` lexicon (seeded only with items we are certain are
  real, common and non-vulgar) and a `[banned]` list that fails every text.
- `pipeline/texts.py`: loader for `langpack/talk`, V4c sizes, V1' (colloquial rows), V3 exemption of earlier-
  taught lemmas and markers for talk/mini, the marker minimum and 20% cap, the author hint, `--require-talk`,
  the `talk`/`mini` briefs; the catalogue carries `register`, `who`, `scene*`.
- Open: native review of every colloquial row and text; retune sizes, the marker cap and levels on real output.
