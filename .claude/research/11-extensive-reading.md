# R11 — Extensive reading

**Confidence: High.**

## Claim

Reading large amounts of comprehensible text improves reading rate, reading comprehension **and**
vocabulary. Adults benefit most, and screen-based reading has outperformed paper.

## Evidence

**Jeon & Day (2016)**, meta-analysis of extensive reading — 49 primary studies, **71 unique
samples, 5,919 participants**, 1980–2014.

| Comparison | Effect |
|---|---|
| Pre → post within ER groups | **d = 0.79** |
| ER vs control | **d = 0.57** |

Moderators:
- **EFL settings > ESL settings** (i.e. ER matters more where ambient exposure is low).
- **Web-based stories > paper books.**
- **Age matters: adults benefit most** — larger existing vocabulary and reading experience make
  them better equipped to read extensively.

Recent update: Learning a Language Through Reading — meta-analysis, *Educational Psychology
Review* (2025) — https://link.springer.com/article/10.1007/s10648-025-10068-6

- Jeon & Day 2016 — https://scholarspace.manoa.hawaii.edu/items/b52a5591-ec04-4026-816f-434361a4e899
- ER research overview — https://ajels.ust.edu.ph/wp-content/uploads/2023/01/AJELS-Vol10-RA1.pdf

## What this changes in our code

1. **The Reader is a first-class trainer, not a utility.** It is Loop 2 in the daily plan, with
   its own time budget, and volume read is tracked as a headline metric
   (`stats.tokens_read_total`) — because volume is the mechanism.
2. **We optimise for "keep reading", not "look up everything".** One keystroke to gloss, one to
   mark known, one to make a card — and no modal dialogs, ever. Anything that stops the flow of
   reading is working against the effect this note documents.
3. **The adult-and-screen moderators are a licence to build this digitally** and to use real
   adult material (Telex, 444, Index, NAV instructions) rather than childish graded readers.
   Difficulty is controlled by [`n+1` selection](06-lexical-coverage.md), not by infantilising
   the content.
4. **The library persists and accumulates.** `text` / `text_token` tables keep every ingested
   text tokenised, so re-reading is cheap and coverage over your *personal corpus* becomes a
   meaningful, improving number.
5. **ER complements but never replaces explicit instruction** ([R4](04-explicit-instruction.md)).
   The Grammar trainer is not optional and the app never suggests that reading alone suffices.

## Caveats

- ER studies are almost entirely about English, and mostly about learners who already have a
  writing system in common with the target language.
- Self-selection is a real confound: learners who read extensively may differ systematically
  from those who do not.
- "Web-based > paper" is from a moderator analysis of a small subset; do not over-read it.

## Related

[[R6 lexical coverage]](06-lexical-coverage.md) · [[R4]](04-explicit-instruction.md)
