# LLM prompt templates

Every prompt the app sends to a language model lives here, versioned, with its rationale and
known failure modes. Nothing is constructed ad hoc in Go string literals.

## Why this directory exists

1. **Prompts are behaviour.** A change to a prompt changes what the learner is taught. It
   deserves the same review as a code change.
2. **Hungarian is where models are weakest.** [R10](../research/10-llm-tutors.md) — the reason
   the attestation guardrail exists. Prompt design is part of the mitigation.
3. **The pedagogy is encoded in the prompt.** "Correct with prompts, not recasts"
   ([R9](../research/09-corrective-feedback.md)) is enforced by the template, not by hoping the
   model behaves.

## Rules

- **One file per template.** Front-matter carries `id`, `version`, `model_hint`, `cites`.
- **Every template states its research basis** in a `## Why` section, linking the note.
- **Every template lists known failure modes** and what the code does about them.
- **Output must be machine-checkable.** Templates that produce study material must emit
  structured output so `internal/corpus/attest.go` can validate the Hungarian before the
  learner ever sees it as a card.
- **Bump `version` on any semantic edit.** `llm_log` records the version used, so a bad
  generation can be traced to the template that produced it.

## Templates

| File | Purpose | Cites |
|---|---|---|
| `generate-input.md` | Comprehensible text from the learner's actual known-lemma set | [R6](../research/06-lexical-coverage.md), [R11](../research/11-extensive-reading.md) |
| `explain-form.md` | Morphological debugger — decompose a form and justify each allomorph | [R4](../research/04-explicit-instruction.md) |
| `correct-writing.md` | Prompt-then-reveal correction + generated drills | [R9](../research/09-corrective-feedback.md) |
| `roleplay-system.md` | Budapest scenario partner, deliberately imperfect | [R10](../research/10-llm-tutors.md) |
| `generate-drill.md` | Contrastive items targeting one weak feature | [R1](../research/01-retrieval-practice.md), [R5](../research/05-interleaving.md) |

## The hard rule

> **The model generates volume. The corpus and the human validate the standard.**
> — [`research/00-source-plan.md`](../research/00-source-plan.md) §6.2

No LLM-produced Hungarian becomes a card without passing `internal/corpus/attest.go`.
Unattested spans are labelled and require an explicit override.
