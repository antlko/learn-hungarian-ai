# Research library

Every design decision in this app traces to a citation. This directory is the source of truth
for those citations.

## The rule

> **No design decision may cite a paper that has no note here.**
> **No note may exist without a "What this changes in our code" section.**

If you are about to write code that implements a pedagogical claim, find the note first. If the
note does not exist, write it first. If the evidence turns out to be weaker than assumed, the
code changes — not the note.

## How to cite from code

Reference notes by their ID in a comment above the relevant logic:

```go
// [R5] Blocked practice must precede interleaving for rule-based material.
// Brunmair & Richter 2019 (g=0.42, attenuated for language tasks); Hwang 2025.
// See .claude/research/05-interleaving.md
func (s *Scheduler) phaseFor(topic Topic, progress Progress) Phase { ... }
```

In Vue, the same convention in a `<!-- -->` or `//` comment above the component or composable.

## Index

| ID | Note | One-line claim | Confidence |
|----|------|----------------|------------|
| — | [00-source-plan.md](00-source-plan.md) | The user's original B2 study plan, verbatim — the brief this app implements | n/a |
| R1 | [01-retrieval-practice.md](01-retrieval-practice.md) | Retrieval beats restudy; feedback amplifies it | **High** |
| R2 | [02-spacing.md](02-spacing.md) | Distributed beats massed; optimal gap scales with retention interval | **High** |
| R3 | [03-fsrs.md](03-fsrs.md) | FSRS-6 as the scheduler; ~20–30% fewer reviews at equal retention | Medium |
| R4 | [04-explicit-instruction.md](04-explicit-instruction.md) | Explicit grammar instruction beats implicit | **High** |
| R5 | [05-interleaving.md](05-interleaving.md) | Interleaving is weaker than folklore and harmful before a competence floor | **High** |
| R6 | [06-lexical-coverage.md](06-lexical-coverage.md) | 95% coverage = minimal comprehension, 98% = unassisted | **High** (for English) |
| R7 | [07-hvpt.md](07-hvpt.md) | Multi-talker phonetic training improves L2 perception; it decays | **High** |
| R8 | [08-capt-asr.md](08-capt-asr.md) | ASR pronunciation training works *only* with explicit corrective feedback | Medium-High |
| R9 | [09-corrective-feedback.md](09-corrective-feedback.md) | Corrective feedback works; prompts beat recasts | **High** |
| R10 | [10-llm-tutors.md](10-llm-tutors.md) | LLM chatbots help; voice input is a significant positive moderator | Medium-Low |
| R11 | [11-extensive-reading.md](11-extensive-reading.md) | Extensive reading works, best for adults, better on screen | **High** |
| R12 | [12-retrieval-direction.md](12-retrieval-direction.md) | You get back the direction you trained | **High** |
| R13 | [13-shadowing.md](13-shadowing.md) | Shadowing helps prosody and fluency; segmental evidence inconclusive | Medium |
| R14 | [14-motivation.md](14-motivation.md) | Gamification raises autonomy, not competence | Medium-Low |
| R15 | [15-hungarian-difficulty.md](15-hungarian-difficulty.md) | What is actually hard about Hungarian, with error distributions | Medium |
| R16 | [16-debunked.md](16-debunked.md) | Things we deliberately do NOT build | **High** |
| R17 | [17-hungarian-nlp-stack.md](17-hungarian-nlp-stack.md) | The Hungarian NLP/speech resources this app is built on | n/a (engineering) |
| R18 | [18-sessions-and-progress.md](18-sessions-and-progress.md) | Relearn to one correct recall per sitting; near goals sustain effort; measured progress is feedback, points are rewards | Medium |
| R19 | [19-grounded-tutor.md](19-grounded-tutor.md) | A small model explains grammar acceptably only when grounded, short-context and labelled | Medium-Low |
| R21 | [21-graded-authored-texts.md](21-graded-authored-texts.md) | A connected, graded, authored text per unit beats a bag of corpus sentences at A1–A2; labelled AI-written | Medium |

## Confidence scale

- **High** — multiple independent meta-analyses, large k, consistent direction.
- **Medium** — one meta-analysis or several primary studies; known heterogeneity.
- **Medium-Low** — recent meta-analyses with short interventions or weak primary studies.
- **Low** — single study, or simulation rather than experiment. Flagged inline where used.

## PDFs

`pdf/` holds only papers whose licence permits redistribution (open-access, author-hosted
preprints where the author permits, government/institution copies). Everything else is
link-only. See [pdf/README.md](pdf/README.md) for the per-file provenance ledger.

## Honest summary of the evidence base

Three things worth stating plainly, because the app's credibility depends on not overselling:

1. **The memory findings are solid.** Spacing, retrieval practice, and feedback are among the
   most replicated results in cognitive psychology. Build on them freely.
2. **The SLA findings are good but noisier.** Effect sizes for instruction, feedback and
   reading are consistent in direction but the underlying studies are smaller, shorter, and
   overwhelmingly about English as L2. Hungarian-specific evidence is thin.
3. **The AI findings are the weakest.** LLM-tutor meta-analyses aggregate interventions
   often lasting 1–7 days. Treat "probably helps" as the ceiling, and never let a model's
   output become study material without validation ([R10](10-llm-tutors.md), and the
   attestation guardrail in `backend/internal/corpus/`).
