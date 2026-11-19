# R24 — Written corrective feedback: what a marked essay should show

Written 2026-11-19 for ADR-085 («Письмо», the writing check). It adds no new study to the library and **cites no figure of its own**: it applies [R9](09-corrective-feedback.md), [R1](01-retrieval-practice.md), [R4](04-explicit-instruction.md), [R10](10-llm-tutors.md), [R15](15-hungarian-difficulty.md) and [R16](16-debunked.md) to one concrete screen, and says where the design departs from [R9](09-corrective-feedback.md)'s first reading. Confidence: **Low to medium** — written (not oral) feedback and a language model as the marker are both outside what [R9](09-corrective-feedback.md) measured.

## Claims and how far they carry

### 1. Feedback on a text should say that, where and why — confidence: medium

[R9](09-corrective-feedback.md) finds corrective feedback works and that explicit correction has the larger immediate effect; [R4](04-explicit-instruction.md) finds explicit rule statements beat implicit exposure. A marked essay therefore names the place, the right form, the reason and the rule — "maximally detailed" is the learner's own requirement and fits both notes. Nothing here says a longer explanation is always better; the card shows the short question first and the rest on request.

### 2. Making the learner produce the repair is the safer default — confidence: medium

[R9](09-corrective-feedback.md): prompts (the learner repairs) beat recasts (the teacher hands over the form) in oral classroom studies, and [R1](01-retrieval-practice.md) explains why retrieval encodes better. Our default screen is therefore hint-first: the error is located, the HINT question is shown, the fix is one click away. **Departure from the first reading of R9:** the v1 design (`prompts/correct-writing.md` v1, [R9](09-corrective-feedback.md) item 1) made the server release three stages one at a time and demand an attempt before the reveal. For a learner's own essay that is a lock on information the learner already holds — they can open the external chat and read the answer there. We keep the pedagogy as the default and drop the enforcement: a global «Показать все исправления» toggle is the learner's choice (autonomy; compare [R14](14-motivation.md)).

### 3. A model's correction can be wrong, so every claim is labelled — confidence: medium

[R10](10-llm-tutors.md) is the weakest-evidence note in the library; small local models misjudge Hungarian ([R15](15-hungarian-difficulty.md) lists how much of the language is ending choice). So the screen says which model wrote the check, says it may be wrong, flags Hungarian words the dictionary does not carry, and keeps the raw answer. The check is never an authority for scheduling.

### 4. Nothing from the check becomes a card — confidence: high (as a design rule)

[R16](16-debunked.md) lists unvalidated generated drills. v1 of the prompt had the model write three practice items per error; those would be model-written Hungarian entering practice. For writing we do not do that: the check marks a text and creates nothing (the invariant «The writing check marks; it mints nothing»). A learner who wants the word learns it through the normal paths.

## What this changes in our code

- `internal/writing` (prompt `Build`, forgiving `Parse`, `Anchor`): one line-tagged answer format with fields WHY / RULE / EXAMPLE and a HINT question; no model-written drills.
- `internal/api/handlers_writing.go`: no staged-reveal enforcement; the UI default is hint-first with a remembered "show all" switch. The superseded parts of [R9](09-corrective-feedback.md)'s "What this changes" (three-stage server release, LLM-written drills, `handlers_write.go`) do not apply to writing.
- `docs/PEDAGOGY.md`: the "prompt-then-reveal, enforced server-side" row and §2.5 point to this note.
- Open: a native speaker should review the worked example in the prompt and a sample of real checks; whether hint-first improves repair rates is not measured here.
