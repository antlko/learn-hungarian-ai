# R20 — Practice that counts, and a progress number per word

Written 2026-11-11 for ADR-064 (practice answers are reviews) and ADR-070 (word progress 0–100).
Sources were checked on that date; where a claim could only be partly confirmed, it says so.

## Claims and evidence

### 1. FSRS already prices an early or same-day answer — confidence: high (read in the library we ship)

The primary source is the code we run, `go-fsrs/v4@v4.0.0`, not a description of it:

- A review on a later day uses `nextRecallStability` (`arithmetic.go:96-116`). The gain is multiplied by a
  term that grows as the recall chance at review time falls, so an answer given early (recall chance near 1)
  adds little and an answer given on time adds more. The gain is never negative.
- A review on the same day (elapsed 0) uses `shortTermStability` (`arithmetic.go:83-90`), and for
  Hard/Good/Easy the result is floored at the current stability. With the default weights a settled card
  therefore gains nothing from a second answer on the same day.
- Again uses `nextForgetStability` (`arithmetic.go:118-130`): stability drops and a Review-state card goes
  to relearning (`scheduler_basic.go:219`).

The public description agrees on direction: "The smaller the value of R, the larger the SInc value", and
the same-day formula is `S' = S·e^{w17·(G−3+w18)}·S^(−w19)`
(open-spaced-repetition, *The Algorithm*, https://github.com/open-spaced-repetition/awesome-fsrs/wiki/The-Algorithm).
The wiki only says implementations should ensure the same-day increase is ≥ 1 for passing grades; that it
actually holds is taken from our library's code, not from the wiki.

### 2. Counting early answers is established practice — confidence: medium

Anki's filtered decks reschedule cards from the learner's answers by default, and early reviews get a
calculation that depends on how early they are (Anki manual, *Filtered Decks*,
https://docs.ankiweb.net/filtered-decks.html). We could not find an official statement of exactly how FSRS
treats elapsed time there; claim 1 is what we rely on.

### 3. "Mature" at 21 days is a convention, not a finding — confidence: high

"A card is defined as 'mature' if its interval is ≥21 days" (Anki manual, *Statistics*,
https://docs.ankiweb.net/stats.html). Our `MatureDays = 21` is that convention; `ProgressFullDays = 180`
is a second convention of ours and has no source.

### 4. A strength meter derived from a memory model has prior art — confidence: medium

Settles & Meeder (2016), "A Trainable Spaced Repetition Model for Language Learning", ACL, pp. 1848–1858
(https://aclanthology.org/P16-1174/) introduced half-life regression. Duolingo showed strength meters
computed from such per-word predictions, aggregated per skill
(https://blog.duolingo.com/how-we-learn-how-you-learn). The meters there were per skill, not per word; we
did not verify the paper's own wording on meters.

### 5. Visible progress changes effort, and a bar that crawls costs motivation — confidence: medium, by analogy

- Kivetz, Urminsky & Zheng (2006), "The Goal-Gradient Hypothesis Resurrected", *Journal of Marketing
  Research* 43(1), 39–58: effort accelerates as a goal gets closer, and an illusory head start speeds
  completion. Consumer reward programmes, not learning.
- Conrad, Couper, Tourangeau & Peytchev (2010), "The impact of progress indicators on task completion",
  *Interacting with Computers* 22(5), 417–427 (https://pmc.ncbi.nlm.nih.gov/articles/PMC2910434): feedback
  that moves slowly early on raises abandonment; fast early feedback improves the experience but not
  necessarily completion. Web surveys, not learning.

Both are transfers from other settings. They argue for a bar that moves visibly in the first days (our
0–20 learning band and the 20–70 band on a linear scale) and for explaining a "+0" rather than hiding it.

### 6. Repetitions close together add less than spaced ones — confidence: high for the effect

Cepeda, Pashler, Vul, Wixted & Rohrer (2006), "Distributed Practice in Verbal Recall Tasks: A Review and
Quantitative Synthesis", *Psychological Bulletin* 132(3), 354–380 (https://pubmed.ncbi.nlm.nih.gov/16719566/).
See also [R2]. This is why "practice needs more answers than a review on time" is true without any rule of
ours: the model gives early answers less.

## What this changes in our code

- `internal/api/handlers_practice.go`: a practice answer goes through the same `e.Answer` path as a review
  (ADR-064). No discount, no "every second answer" rule, no counter — claim 1 does the pricing. Words in
  «Знаю» and new cards stay log-only.
- `internal/srs/progress.go`: `progressOf` maps stage and stability to 0–100 for display only (ADR-070).
  Marks at 20 and 70 follow the stage boundaries (claim 3); the 20–70 band is linear in stability so early
  days move visibly (claim 5); the top band is logarithmic up to `ProgressFullDays`.
- UI copy: a same-day answer on a settled word shows "+0" with the reason (claims 1 and 6). The number is
  called progress, never a score, points or level ([R14], [R16]).
- `internal/srs/replay_test.go`: the fuzz covers second, minute and same-day gaps, because practice makes
  them common.

## Caveats

- Claims 4 and 5 come from other products and settings; nobody has measured a per-word bar in this app.
- `ProgressFullDays = 180` is a product choice. Changing it changes what "100" means and nothing else.
- Interval fuzz can flip a card between young and mature around 21 days, so a correct answer can show −1.

## Related

[R2] spacing; [R14], [R16] motivation surface; ADR-003, ADR-019 (superseded), ADR-039 (amended), ADR-064, ADR-070.
