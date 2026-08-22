# R5 — Interleaving: weaker than folklore, and harmful before a competence floor

**Confidence: High.** This note **overrides the source plan** and is the single most
consequential correction in the project.

## The claim the source plan makes (§2.3)

> Стратегия: вводить рано (месяц 3), затем **интерливинг** — не блоками «сегодня определённое»,
> а вперемешку, чтобы мозг каждый раз делал выбор. Это классическая «желательная трудность».

## What the evidence actually says

### Brunmair & Richter (2019), *Psychological Bulletin* — "Similarity matters"

- **59 studies, 238 effect sizes, 158 samples** (cut-off June 2018).
- Overall interleaving effect: **g = 0.42** — medium, not the transformative effect the popular
  literature implies.
- **Critical moderator:** effects are **attenuated for explicit, rule-based tasks and for
  language-learning tasks (grammar, vocabulary)**. The large effects come from visual category
  learning — paintings scored **g = 0.67**.
- Mechanism: interleaving helps when categories are **hard to discriminate** (low
  between-category discriminability). When categories are already discriminable, **blocking can
  be better**, because noticing *within*-category regularities is what matters.

### Hwang (2025), *Language Learning* 75(1) — "Undesirable Difficulty of Interleaved Practice"

Title says it. Findings:

- Interleaving's difficulty **prevents low achievers from building declarative knowledge**
  (form–meaning associations) in the **early stage** of learning.
- Below a threshold of prior achievement or in-practice accuracy, learners **cannot absorb the
  cognitive load** interleaving imposes.
- **Initial blocked practice is important** for declarative knowledge development.

### Nakata & Suzuki (2019), *Modern Language Journal* — "Effects of Blocking, Interleaving, and Increasing Practice"

L2-specific treatment of the same question, reaching compatible conclusions.

- Brunmair & Richter 2019 — https://www.psychologie.uni-wuerzburg.de/fileadmin/06020400/2019/Brunmair_Richter_in_press__2019_META-ANALYSIS_OF_INTERLEAVED_LEARNING.pdf
- Hwang 2025 — https://onlinelibrary.wiley.com/doi/10.1111/lang.12659
- Nakata & Suzuki 2019 — https://yuichisuzuki.net/wp-content/uploads/2023/04/Nakata-Suzuki-2019-MLJ.pdf
- Firth et al. (2021) systematic review — https://bera-journals.onlinelibrary.wiley.com/doi/10.1002/rev3.3266

## Why the source plan's version would actively hurt

Definite vs indefinite conjugation is:

1. **rule-based and explicit** — the category where Brunmair & Richter found attenuation;
2. **being introduced at month 3**, i.e. exactly the "early stage / low prior knowledge"
   condition where Hwang found interleaving produces *undesirable* difficulty;
3. **a distinction with no L1 analogue** for a Russian or English speaker — so the learner has
   no scaffold to absorb the extra load.

Interleaving from introduction is the worst case the literature describes.

## What this changes in our code

**`internal/drill/schedule.go` implements a blocked → interleaved gate.**

```
phase(topic) :=
    locked        if prerequisites unmet
    blocked       if rolling_accuracy(topic, window=20) < gate_threshold
    interleaved   if rolling_accuracy(topic, window=20) >= gate_threshold
    mastered      if interleaved_accuracy(window=50) >= mastery_threshold
```

- `gate_threshold` default **0.80**, `mastery_threshold` default **0.90**, window sizes and
  both thresholds live in `config.yaml` under `drill:` — tunable without a rebuild.
- Once interleaved, a topic can **fall back to blocked** if rolling accuracy drops below
  `gate_threshold - hysteresis` (default 0.10). The gate is not a one-way door.
- **The phase is visible in the UI.** `topic_progress.phase` is rendered on the Grammar view
  with a plain-language explanation of why you are in it. Learners are told that the app is
  deliberately holding back interleaving until they are ready, and why — this is honest, and it
  also prevents "why is this so easy" confusion.
- Interleaving, when it does happen, mixes **similar contrasting items** (definite vs
  indefinite of the *same* verb, `-ban` vs `-ba` of the *same* noun) rather than random topics —
  because Brunmair & Richter's mechanism is discriminability, not variety for its own sake.

## Caveats

- Brunmair & Richter's language-task moderator is based on a subset of their studies; the
  attenuation is real but the sub-analysis is less powered than the headline.
- Hwang's sample is low-achieving adolescents. Generalising to a motivated adult professional
  is an extrapolation — which is exactly why our gate is a **tunable threshold** rather than a
  hardcoded prohibition, and why an advanced user can lower it.

## Related

[[R4 explicit instruction]](04-explicit-instruction.md) · [[R15 Hungarian difficulty]](15-hungarian-difficulty.md) · [[R1 retrieval practice]](01-retrieval-practice.md)
