---
id: tutor-system
version: 4
model_hint: any OpenAI-compatible chat model; designed for 7–8B local models on an 8k context; temperature 0.3; `reasoning_effort: none` for local servers
cites: [R19, R4, R10]
output: streamed markdown, Hungarian in backticks
---

# The grammar tutor's prompt

The system text itself lives in `backend/internal/tutor/prompts/system_en.md` and
`system_ru.md` and is `//go:embed`ded, so there is ONE copy the binary actually sends. This
note is the rationale, the assembly order, and the failure modes; bump `version` here and
`PromptVersion` in `prompt.go` on any semantic edit, and `tutor_message.prompt_version`
records which one produced an answer.

## Why

[R19](../research/19-grounded-tutor.md) — a small model explains Hungarian acceptably only
when it is grounded in material placed where it will be read, kept short, and labelled.
[R4](../research/04-explicit-instruction.md) — a rule stated at the moment of need is explicit
instruction; a translation taken apart word by word, with the suffix and the rule named on
each, is that rule stated at the moment of need. [R10](../research/10-llm-tutors.md) — the
model generates volume; the app and the human hold the standard. The tutor therefore explains
and never mints material.

## Assembly order (and why the order)

Position matters more than volume for a small model ([R19 §3]): rules first, question last,
the authoritative lesson before the reference dump.

```
1. system rules (the explanation's language, role, honesty, format)   ~700 tokens
2. "The lesson the app teaches" — the pack's explain_md for the topic, trimmed  ≤1,200
   — or, when no lesson was pinned or inferred (ADR-029), one paragraph saying so
2b. "The card" (ADR-034) — when the question was asked from a review card: the
   sentence with its translation and label, the word and its dictionary form,
   the glosses, the graded features with labels and allomorphs, what was typed
   against what was wanted and the diff's class; before the answer, the answer
   is withheld and the block says so FIRST                                       ≤500
   (the reference below yields to it: 3,600 → 2,000 characters)
3. "Features" — id · label · allomorphs · one-line explanation, from the pack   ≤300
4. "This learner" — accuracy and observations per feature, recent wrong answers ≤150
5. "Reference" — up to three retrieved sections of the embedded corpus         ≤1,500
6. the last six turns of the thread, oldest first                              ≤1,200
7. the question — composed by the server for a prepared template (ADR-034) —
   then, for a prepared question, its SHAPE: the numbered sections the answer
   must have and «Без проверочного вопроса в конце»; then two sentences:
   "Explain in {language}; everything Hungarian in Hungarian, in Latin
   letters, in backticks. A request to translate … is answered with the whole
   Hungarian sentence, then a word-by-word breakdown." — the tail is what
   survives when a 4k-context Ollama truncates the prompt from the head, so
   both rules live there too                                                   —
```

Every block is cut to its budget on a UTF-8 boundary; a block that does not fit is shortened,
never dropped, so the model always sees at least the head of the lesson.

## The rules the system text states

- You are a Hungarian grammar tutor inside a learning app. **Explain in {language}.**
  Everything Hungarian — a word, a form, a suffix, a whole sentence — is written **in
  Hungarian, in Latin letters**: that is the material the learner is asking about, not a
  breach of the language rule. *(v3 — v2 said "Answer ONLY in Russian", first line and last,
  and a 1.7B model asked «напиши на венгерском …» handed the Russian sentence back unchanged.
  The language rule is about the explanation, never the material; `ONLY` does not appear and
  `tutor_test.go` pins that.)*
- The lesson and the feature table below are the app's; treat them as correct and stay
  consistent with them. The reference sections are open grammars; prefer the lesson when they
  differ. When the lesson heading says no lesson is open, answer from the reference and name
  which part of the grammar the question belongs to. *(v2, ADR-029.)*
- Write everything Hungarian in **Latin Hungarian orthography** in backticks — a word, a form,
  a suffix — and never transliterate it: a learner who typed «ем» is answered about `-em`. Do
  not invent forms: if unsure whether a form exists, say so rather than guessing. A wrong form
  is worse than no form. *(v2 — the model had answered «Ем — задний гласный» in Cyrillic.)*
- Name the mechanism: which vowel-harmony class, which suffix, which rule. **Always** give two
  or three examples — whole words with the suffix attached and a translation (`házban` — «в
  доме») — starting from the learner's own word when they quoted one. Not ten. *(v2 — an
  answer with no example at all had been the norm.)*
- **A translation request has a shape** (rule 4). Asked to translate into Hungarian, to write
  something in Hungarian, or how to say something: first the whole Hungarian sentence in
  backticks; then its literal rendering; then **word by word** — what each word or phrase
  means, which suffix or preverb it carries and why that one (case, harmony class,
  definiteness, person, number); and when the learner offered a version of their own, a
  comparison that explains every difference. A Russian sentence without the Hungarian is
  stated not to be an answer. *(v3, ADR-030.)*
- Short: three to eight sentences, or a small table — except the translation breakdown, which
  may run to fifteen lines. No preamble, no praise, no emoji. *(v3 — the eight-sentence cap
  had forbidden the breakdown.)*
- If the question is not about Hungarian grammar or vocabulary, say so in one sentence.
- End with one short check question the learner can answer in a word or a form — unless they
  asked for examples only.
- You cannot create cards, exercises or notes, and nothing you write becomes one.
- **If there is a CARD section, answer about that card and in the shape given at the end of
  the question; if the card says the answer is withheld, the learner is still answering:
  explain the rule and do not name the form, the word or its meaning.** *(v4, ADR-034.)*

## Known failure modes

| Failure | Mitigation |
|---|---|
| Invents a suffix or a stem form | The lesson and the allomorph table are in the prompt; every backticked Hungarian word is looked up in `wordform`, and the ones the dictionary lacks are listed under the answer as *not found in the dictionary*. A backticked **sentence** is split and checked word by word (up to 200 characters), so a stem invented inside a translation is listed too |
| **Echoes the learner's Russian sentence when asked to translate it** | v3: the language rule names the explanation, not the material; rule 4 gives the answer a shape and says the Russian alone is not one; the closing line repeats that Hungarian is written in Hungarian |
| Translates without saying why | Rule 4's word-by-word breakdown, and the lesson's feature table in front of it so the names of the cases are the pack's |
| Answers in English to a Russian learner (or vice versa) | The explanation language is stated twice — first line and last line — and the UI shows the language it asked for |
| Ignores the lesson and answers from parametric memory | Lesson placed second, immediately after the rules; the reference is limited to three sections so the lesson is not buried |
| Long lecture | "three to eight sentences" in the rules, `max_tokens` bounded at 1,000, the panel renders as it streams so an over-long answer is visibly over-long |
| Cut off by the budget mid-breakdown | `finish_reason: length` is surfaced as `llm.ErrTruncated` and the answer is stored `interrupted`, so the panel says it stopped rather than passing it off as complete |
| Praise, emoji, filler | Forbidden in the rules; not enforced by code — a cosmetic failure |
| Wanders off Hungarian | Rules ask for a one-sentence refusal; a pinned thread keeps the context on the rule being studied, and a free thread's inferred lesson is printed under the answer so a wrong grounding is visible |
| Writes Hungarian in Cyrillic, or answers without an example | v2 rules 2 and 3; the dictionary check cannot see a Cyrillic "form", so the rule is the only defence — the label under the answer names the model that broke it |
| Picks the wrong lesson for a free chat | Inference is deterministic (`tutor.InferTopic`) and its verdict is printed («по уроку: …»); pinning a topic on a new chat is the override |
| Claims a card was created, or offers to | Rules state that nothing it says becomes a card; the UI carries the same standing note; there is no code path from the tutor to `note` |
| Stale or empty answer from a model that timed out | 150 s stream timeout (a 1,000-token breakdown at a laptop CPU's seven tokens a second); the partial text is kept and marked as interrupted rather than lost |
| Recites the prompt's own headings («## ПРИЗНАКИ», «## СПРАВОЧНИК») instead of answering | `tutor.EchoAt` cuts the answer at the first such heading, the stream is stopped, and the answer is stored `interrupted` with `cutReason: echo`; the label under it says the model began repeating the prompt's scaffolding |
| Names the form before the answer despite the withheld line | The client counted the question as a hint the moment it was sent (`usedHint`), so the grade is capped whatever the model says; the block's withheld line is printed first so no budget cut removes it; the hint templates ask for the rule, not the form |
| Answers a prepared question in prose instead of the numbered sections | The shape is the last thing before the language line ([R19 §3]); the panel renders whatever came back — a cosmetic failure, not a wrong fact |
| A 1–2B model, fluent and wrong | Nothing in the prompt fixes the model — measured: `qwen3:1.7b` with no prompt at all turned «Я пришёл в эту деревню…» into `Ígép a városba és bejártam a házam`, and with v3 printed the lesson's example table instead of a translation. The page says what it is: a tag that names a size under 7B draws a line under the notice with the size and the 7–8B the tutor was built for |

## Version history

- **v1** (2026-09-02) — ADR-027: the lesson-pinned tutor.
- **v2** (2026-09-03) — ADR-029: Latin orthography always; examples always; the no-lesson case
  stated; `reasoning_effort: none` sent to local servers (thinking models spent the answer budget
  on hidden reasoning).
- **v4** (2026-09-04) — ADR-034: the CARD block after the lesson (the reference yields room
  to it), the answer withheld before the answer with the line printed first, rule 9, the
  answer shape for a prepared question in the tail with the check question switched off;
  the question limit counted in characters.
- **v3** (2026-09-03) — ADR-030: the language rule names the explanation, never the material
  (`ONLY` removed from both lines); a translation request answers with the sentence, its literal
  rendering, a word-by-word breakdown and a comparison with the learner's own attempt; the
  breakdown is exempt from the eight-sentence cap; `max_tokens` 700 → 1,000, timeout 90 → 150 s,
  a budget cut marked `interrupted`.

## What this deliberately does not do

It does not create drills, cards or glosses ([ADR-027](../DECISIONS.md#adr-027)) — a translated
sentence is an explanation on the screen it was written on, labelled and dictionary-checked,
never corpus-attested and therefore never material ([ADR-030](../DECISIONS.md#adr-030)). It does
not correct free writing — that is `correct-writing.md`, prompt-then-reveal, and Phase 9. It is
not a conversation partner and does not touch Loop 4 ([R16 §7](../research/16-debunked.md)).
