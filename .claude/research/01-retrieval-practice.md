# R1 — Retrieval practice (the testing effect)

**Confidence: High.** Among the most replicated findings in cognitive psychology.

## Claim

Actively retrieving information from memory produces markedly better long-term retention than
restudying the same material for the same amount of time. The effect grows when retrieval is
**effortful** and when **feedback** follows.

## Evidence

| Source | k | Effect |
|---|---|---|
| Rowland (2014), *Psychological Bulletin* 140:1432–63 | 61 experiments | **g = 0.50** (testing vs restudy) |
| Adesope, Trevisan & Sundararajan (2017), *Review of Educational Research* | 217 studies | **g = 0.61** |

Rowland's moderator analysis: the effect is **larger when material is more complex**, when
retrieval is **more effortful**, and when **feedback is given**. Adesope et al. found classroom
experiments show benefits comparable to laboratory ones — it is not a lab artefact.

- Rowland 2014 — https://pubmed.ncbi.nlm.nih.gov/25150680/ · [full text](https://courseware.epfl.ch/assets/courseware/v1/fdde2f0aa590bf3b1324077a6bf1540c/asset-v1:EPFL+DEMO+2020+type@asset+block/Rowland2014-meta-analysis.pdf)
- Adesope et al. 2017 — https://journals.sagepub.com/doi/abs/10.3102/0034654316689306
- Overview of effect sizes — https://pdf.retrievalpractice.org/MetaAnalysisGuide.pdf

## What this changes in our code

1. **There is no "study mode" that just shows you material.** Every learning interaction in
   `internal/drill/` and the review loop is a retrieval attempt. Reading in `internal/mining/`
   is the one exception, and it is framed as input, not study.
2. **Feedback is immediate and unconditional** — `web-ui/src/views/ReviewView.vue` reveals the
   correct answer plus the morpheme breakdown on every single answer, right or wrong. Never a
   bare "incorrect".
3. **We prefer effortful formats.** Card direction weighting in `internal/srs/directions.go`
   favours `produce` and `cloze` over `recognize`, because free recall is more effortful than
   recognition. Recognition is used only to bootstrap a brand-new lemma.
4. **Latency is recorded on every answer** (`review_log.duration_ms`) because effortfulness is
   a moderator we can actually observe, and it feeds the rating derivation in
   `internal/srs/emit.go`.

## Caveats we must not gloss over

- Effect sizes come overwhelmingly from verbal material in laboratory-like conditions with
  retention intervals of days to weeks, not years.
- "More effortful is better" has a ceiling: retrieval that fails completely teaches nothing.
  This is why the `n+1` scorer ([R6](06-lexical-coverage.md)) targets *achievable* difficulty
  rather than maximum difficulty.

## Related

[[R2 spacing]](02-spacing.md) · [[R3 FSRS]](03-fsrs.md) · [[R12 retrieval direction]](12-retrieval-direction.md)
