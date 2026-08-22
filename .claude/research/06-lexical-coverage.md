# R6 — Lexical coverage thresholds

**Confidence: High for English. The transfer to Hungarian is an open question and we say so.**

## Claim

Comprehension of a text depends sharply on the proportion of running words the reader knows.
Two thresholds recur:

- **~95% coverage** — minimally acceptable comprehension, adequate *with* support.
- **~98% coverage** — comprehension without support (no dictionary, no glosses).

## Evidence

| Source | Finding |
|---|---|
| Laufer & Ravenhorst-Kalovski (2010), *Reading in a Foreign Language* 22(1) | 95% = minimally acceptable; 98% = optimal / unassisted |
| Nation (2006), *Canadian Modern Language Review* | 95% coverage ≈ **5,000** word families (with proper nouns); 98% ≈ **8,000** word families |
| Hu & Nation (2000) | Original demonstration of the coverage–comprehension relationship |

Nation's breakdown for 95%: 78% from the 1K band + 8% from 2K + 3% from 3K + 3% from 4K–5K +
3% proper nouns. Spoken text needs less: ~6,000–7,000 families for 98%.

- Laufer & Ravenhorst-Kalovski 2010 — https://files.eric.ed.gov/fulltext/EJ887873.pdf
- Nation 2006 — https://www.researchgate.net/publication/239928724
- Follow-up review — https://files.eric.ed.gov/fulltext/EJ1316858.pdf

## The Hungarian problem — read this before using the numbers

These figures are **English word-family counts**. They do not transfer to Hungarian cleanly:

1. **The unit is wrong.** An English "word family" bundles a handful of inflections. A Hungarian
   lemma generates *hundreds* of forms through agglutination. `ház` alone yields `házban`,
   `házba`, `házból`, `házam`, `házaim`, `házaimban`, … — counting these as separate items
   would inflate any vocabulary estimate by an order of magnitude.
2. **Coverage is bimodal in a way English's is not.** A Hungarian reader can know the lemma and
   still fail on the form, because the *suffix* is unknown. Lemma coverage overstates real
   comprehension.
3. The source plan already flags this (§2.6) and prescribes counting by lemma. It is right.

**Our resolution:** we count coverage two ways and show both.

- **Lemma coverage** — % of running tokens whose lemma is known. Comparable to the literature.
- **Element coverage** — % of running tokens where the lemma **and every morphological feature
  on that token** are known. Strictly lower, and the honest number for an agglutinative
  language. This is the one the `n+1` scorer uses.

## What this changes in our code

1. **`internal/stats/coverage.go` computes both metrics.** The reader shows lemma coverage
   prominently and element coverage next to it, with a tooltip explaining the difference.
2. **The 95/98% threshold lines are drawn on the coverage meter but explicitly labelled**
   *"English-derived, indicative only"* in both locales. We do not present them as Hungarian
   facts. `web-ui/src/components/reader/CoverageMeter.vue`.
3. **The listening pre-check uses ~90% element coverage** as the "this is too hard" warning
   threshold, per the source plan's §3 rule ("если понимаете <85% — материал слишком сложный").
4. **`n+1` selection targets element coverage ≥ 0.90** with 1–2 unknown elements — see
   `internal/mining/score.go` and [R11](11-extensive-reading.md).
5. **Vocabulary goals are stated in lemmas**, matching the source plan's targets: A1 700–900,
   A2 ~2,000, B1 3,500–4,500, B2 6,000–8,000. Never in wordforms. `internal/stats/goals.go`.

## Related

[[R11 extensive reading]](11-extensive-reading.md) · [[R15 Hungarian difficulty]](15-hungarian-difficulty.md)
