---
id: correct-writing
version: 2
model_hint: strongest available; small local models (< 7B) often break the format, so an external chat is recommended for them
cites: [R9, R1, R4, R15, R24]
output: line-tagged text (TANULOM-CHECK 1)
---

# Writing check — one format, one parser (v2)

Supersedes v1 (JSON, three-stage reveal, model-written drills): [ADR-085](../DECISIONS.md#adr-085),
[R24](../research/24-written-corrective-feedback.md).

## Why

[R9](../research/09-corrective-feedback.md) and [R4](../research/04-explicit-instruction.md): feedback
that names the place, the right form, the reason and the rule works; a hint question first lets the
learner attempt the repair ([R1](../research/01-retrieval-practice.md)). v2 keeps that as the **UI default**
(hint-first, a "show all" switch) and does **not** enforce staging on the server. The model writes **no
practice items**: model-written Hungarian never becomes practice. The check marks; it mints nothing.

## One template, two transports

The same text is sent to the local model (`services.llm`, as one user message) and shown to the learner
to copy into an external chat. It is built by `internal/writing.Build` (`PromptVersion = 1`, the
"TANULOM-CHECK n" number) and pinned by golden tests for both explanation languages. Parameters:
`LEVEL` (the piece's CEFR level), `TOPIC` (title and brief in the learner's locale, or "free topic
chosen by the learner."), the optional suggested length, `EXPLAIN_LANG` (`Russian` | `English`),
`TEXT` (the essay, trimmed, between `<<<` and `>>>`). Instructions are English; the worked example stays
Russian in both locales (it shows the format, not the language). **The word `ONLY` never appears in it.**
A test checks the example's Hungarian forms (*boltba, kenyeret, tejet, parkba, levelet*) against the pack.

## Template (generated from `prompt.go`; the code is the source of truth)

~~~text
You are an experienced teacher of Hungarian as a foreign language. Below is a text written by a learner. Review it the way a careful teacher marks a written assignment.

ABOUT THE LEARNER AND THE TASK
- The learner works at CEFR level {{LEVEL}}.
- Task: {{TOPIC}}
{{LENGTH}}- The learner's own language is {{EXPLAIN_LANG}}.

WHAT TO DO
1. Find every objective error: spelling and accent marks (vowel length), vowel harmony, case endings, possessive endings, definite versus indefinite conjugation, agreement, verb forms, preverbs, word order, articles, wrong word choice, missing or extra words, punctuation.
2. Correct each error with the smallest change that fixes it. Keep the learner's words and ideas. Do not rewrite correct sentences to make them more elegant. Something correct but unnatural may be reported with TYPE: style, at most three times.
3. For each error explain, for a learner at this level: why it is wrong, what the right form is, why it is right, and which grammar rule applies. Be detailed: this explanation is the point of the exercise.
4. Report every error in its own [ERROR] block, in the order the errors appear in the text.
5. Write HINT, WHY, RULE, SUMMARY, STRENGTH and VOCAB notes in {{EXPLAIN_LANG}}. Hungarian words, QUOTE, FIX, EXAMPLE sentences and the corrected text stay in Hungarian.

ANSWER FORMAT
A program reads your answer, so follow this format exactly. Put the whole answer inside one code block that starts with ```text and ends with ```. Write nothing before or after the code block. Field names stay in English capitals, each at the start of its own line, followed by a colon. A value may continue onto the next lines.

TANULOM-CHECK 1
LEVEL: <the CEFR level this text shows: A1, A2, B1, B2, C1 or C2>
SUMMARY: <two to four sentences: the overall impression and the most important thing to work on>
STRENGTH: <one thing done well; repeat this line for each, at most three>

[ERROR]
QUOTE: <the wrong words copied character for character from the learner's text, one to six words, as short as possible>
NTH: <which occurrence of QUOTE in the text this is; 1 for the first>
TYPE: <one of: spelling, vowel_length, harmony, case, possessive, definiteness, agreement, verb_form, preverb, word_order, article, word_choice, missing_word, extra_word, punctuation, style>
FIX: <the corrected words that replace QUOTE>
HINT: <a short question that points at the problem without giving the answer>
WHY: <why QUOTE is wrong here and why FIX is right>
RULE: <the name of the rule, then the rule itself in one or two sentences>
EXAMPLE: <one more correct Hungarian sentence that uses the same rule> — <its translation into {{EXPLAIN_LANG}}>

For a missing word, QUOTE is the word just before the gap and FIX is that word followed by the missing one. For an extra word, FIX is QUOTE without it.

[VOCAB]
<one line per note: a more natural or more precise word — why>

[CORRECTED]
<the whole text with all corrections applied and nothing else changed>

[END]

If the text has no errors, write no [ERROR] block and say so in SUMMARY.

AN EXAMPLE OF THE FORMAT (for a different, shorter text; explanations in Russian)
```text
TANULOM-CHECK 1
LEVEL: A1
SUMMARY: Текст понятный. Главное — окончания: направление «куда» и прямое дополнение.
STRENGTH: Правильный порядок слов с «tegnap» в начале.

[ERROR]
QUOTE: boltban mentem
NTH: 1
TYPE: case
FIX: boltba mentem
HINT: Глагол «menni» отвечает на вопрос «где?» или «куда?»?
WHY: «-ban» значит «в чём» (место). С глаголом движения нужно «куда» — «-ba».
RULE: Иллатив -ba/-be: движение внутрь. Инессив -ban/-ben: нахождение внутри.
EXAMPLE: A parkba megyek. — Я иду в парк.

[ERROR]
QUOTE: egy kenyér
NTH: 1
TYPE: case
FIX: egy kenyeret
HINT: Что ты купил? Какое окончание у прямого дополнения?
WHY: «kenyér» — прямое дополнение глагола «vettem», ему нужен аккузатив -t; в этом слове основа меняется: kenyér → kenyeret.
RULE: Аккузатив -t отмечает прямое дополнение; у части слов долгое é в основе сокращается.
EXAMPLE: Levelet írok. — Я пишу письмо.

[CORRECTED]
Tegnap a boltba mentem. Vettem egy kenyeret és tejet.

[END]
```

THE LEARNER'S TEXT
<<<
{{TEXT}}
>>>
~~~

## Parser contract (`writing.Parse`, run on read)

Fences with any info string, chatter before/after, `**bold**` keys, bullets, CRLF, BOM, `<think>` blocks,
a missing header and a missing closing fence are tolerated. An error without QUOTE or FIX (FIX may be
empty only for `extra_word`) is dropped and counted. Caps: 60 errors, 2000 runes per field, 5 strengths,
20 vocab notes. Status: `failed` (nothing recognised) → HTTP 422 for a paste and nothing stored; `partial`
(truncated, dropped, or no header); else `ok`. A header version above 1 is still parsed and flagged.

## Failure modes

| Failure | Mitigation |
|---|---|
| Small model breaks the format or stops early | forgiving parser; partial results kept and marked; the UI recommends an external chat when `modelParamsB < 7` |
| QUOTE not found in the text | anchored by exact → folded → unique accent-folded match; otherwise listed as «не нашли это место в тексте», not highlighted |
| Invented Hungarian in FIX / EXAMPLE | words absent from the dictionary are flagged «Нет в словаре»; the whole check is labelled with the model |
| Over-correcting correct text | the prompt forbids rewriting; `style` notes are capped at three; the check is advice, never a score |
| Model recites the prompt | the answer is parsed, not shown raw (raw only behind «Показать исходный ответ», as plain text) |
