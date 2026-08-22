# R9 — Corrective feedback: prompts beat recasts

**Confidence: High.** Two well-known meta-analyses, consistent direction.

## Claim

Corrective feedback measurably improves L2 accuracy. **How** you correct matters more than
whether you correct: feedback that **pushes the learner to produce the repair themselves**
(prompts, elicitation, clarification requests) outperforms feedback that simply supplies the
correct form (recasts).

## Evidence

**Li (2010), *Language Learning* 60(2):309–365** — "The effectiveness of corrective feedback in
SLA: A meta-analysis"

| Feedback type | Effect |
|---|---|
| Corrective feedback overall | **d = 0.64**, maintained over time |
| Explicit correction | **d = 0.81** (larger *immediate* effect) |
| Recasts | **d = 0.70** |
| Implicit feedback | smaller immediately, but **better maintained** |

**Lyster & Saito (2010)** — oral CF in classrooms: CF significantly aids development,
**particularly with prompts rather than recasts**.

- Li 2010 — https://onlinelibrary.wiley.com/doi/abs/10.1111/j.1467-9922.2010.00561.x
- Lyster & Saito 2010 — https://www.researchgate.net/publication/259427636

Note the tension: explicit feedback wins immediately, implicit feedback is retained better.
Prompts are the useful synthesis — they are explicit that *something* is wrong (so the learner
notices) while requiring the learner to retrieve the repair (so it is encoded, cf. [R1](01-retrieval-practice.md)).

## What this changes in our code

1. **The writing checker is prompt-first, reveal-second.** `internal/llm/correct.go` produces a
   three-stage response for each error:

   | Stage | Content | Shown |
   |---|---|---|
   | 1 · Locate | "There is an agreement error in clause 2." | immediately |
   | 2 · Hint | "Look at the object — is it definite?" | on request |
   | 3 · Reveal | corrected form + one-sentence rule + 3 new practice items | on request or after a failed retry |

   The learner must attempt the repair before stage 3 unlocks. This is enforced server-side, not
   just in the UI, so it survives a page reload.

2. **This mirrors the source plan's own §6.1 prompt #3** — *"Не «исправь», а «исправь и дай
   дриллы»"*. Our template lives at `.claude/prompts/correct-writing.md` and always demands
   `(а) исправление, (б) правило одним предложением, (в) три НОВЫХ предложения`.

3. **Every confirmed error becomes a drill.** `internal/drill/from_error.go` maps the error to
   its underlying `morph_feature` and enqueues targeted items — feedback that does not change
   what you practise next is wasted feedback.

4. **Review feedback follows the same shape.** In `ReviewView.vue`, a wrong typed answer first
   highlights *which morpheme* diverged ([`internal/morph/diff.go`](../../backend/internal/morph/))
   before showing the full correct form. Locate → then reveal.

5. **Never correct what was right.** The prompt template explicitly forbids commentary on
   correct output — the source plan asks for this and it also keeps token cost and reading load
   down.

## Caveats

- These meta-analyses study *oral, interactional* feedback in classrooms. Extending them to
  asynchronous written feedback from a machine is an extrapolation.
- The immediate/durable trade-off between explicit and implicit is not fully resolved; our
  staged design is a reasoned compromise, not a directly evidenced protocol.

## Related

[[R1]](01-retrieval-practice.md) · [[R4]](04-explicit-instruction.md) · [[R8]](08-capt-asr.md) · [[R10]](10-llm-tutors.md)
