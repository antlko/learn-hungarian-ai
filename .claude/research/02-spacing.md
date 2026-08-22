# R2 — The spacing effect (distributed practice)

**Confidence: High.**

## Claim

Study episodes separated in time produce better long-term retention than the same total study
time massed together. The **optimal gap is not fixed** — it scales with how long you need to
remember the material.

## Evidence

Cepeda, Pashler, Vul, Wixted & Rohrer (2006), *Psychological Bulletin* — "Distributed practice
in verbal recall tasks: A review and quantitative synthesis":

- **839 assessments across 317 experiments in 184 articles.**
- Spacing (spaced vs massed) and lag (more vs less spaced) both benefit retention.
- **Key finding:** inter-study interval and retention interval *interact* — the ISI producing
  maximal retention **increases as the retention interval increases**. There is no single
  universal optimal gap.
- Expanding intervals outperformed uniform ones.

Follow-up: Cepeda et al. (2008), *Psychological Science*, "Spacing effects in learning: A
temporal ridgeline of optimal retention" — the optimal gap is roughly 10–20% of the desired
retention interval.

- Cepeda et al. 2006 — https://augmentingcognition.com/assets/Cepeda2006.pdf
- Cepeda et al. 2008 — https://laplab.ucsd.edu/articles/Cepeda%20et%20al%202008_psychsci.pdf
- Author page — https://www.yorku.ca/ncepeda/publications/CPVWR2006.html

## What this changes in our code

1. **This is the justification for having a scheduler at all**, rather than a fixed daily list.
   `internal/srs/` exists because of this finding.
2. **We do not hardcode intervals.** The ISI×RI interaction is exactly what a DSR model like
   FSRS ([R3](03-fsrs.md)) parameterises, which is why we adopt FSRS rather than a fixed
   ladder like SM-2's 1d/6d/×2.5.
3. **Desired retention is a user-facing setting**, not a constant — because "how long do you
   need to remember this" is a user goal, and the optimal schedule follows from it. Exposed in
   Settings → Review, default 0.90, range clamped to 0.80–0.95 in
   `internal/config/config.go`.
4. **The daily plan spreads work across the day** rather than concentrating it. The four loops
   in the Today view are deliberately separate sessions — this is spacing applied at the level
   of the daily schedule, not just the card.

## Caveats

- Almost all of this literature uses verbal recall over days-to-months. Extrapolating to
  multi-year retention is an extrapolation, not a finding.
- "Expanding beats uniform" is weaker than the core spacing effect and has failed to replicate
  in some later work. We get expansion from FSRS as a *consequence* of stability growth, not as
  a design goal in itself.

## Related

[[R1 retrieval practice]](01-retrieval-practice.md) · [[R3 FSRS]](03-fsrs.md)
