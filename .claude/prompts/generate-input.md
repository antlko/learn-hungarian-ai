---
id: generate-input
version: 1
model_hint: any instruct model; Hungarian-adapted preferred (Racka-4B, PULI)
cites: [R6, R11, R4]
output: json
---

# Comprehensible input generator

Produces a short text using **only lemmas the learner already knows**, plus a bounded number of
new ones. This is the single most valuable thing an LLM does here, because no textbook knows
which 1,400 lemmas *this* learner has.

## Why

[R6](../research/06-lexical-coverage.md) — comprehension collapses below ~95% coverage, so
generated input must be built from a known-lemma set rather than guessed at a "level".
[R11](../research/11-extensive-reading.md) — volume of comprehensible reading drives gains, and
adults benefit most from real, non-infantilised content.
[R4](../research/04-explicit-instruction.md) — new items are marked explicitly, not left to be
inferred.

## Template

```
Ты — преподаватель венгерского как иностранного.

ЗАДАЧА: написать связный текст на венгерском, {{word_count}} слов, тема: «{{topic}}».

ЖЁСТКИЕ ОГРАНИЧЕНИЯ:
1. Используй ТОЛЬКО леммы из списка ИЗВЕСТНЫЕ_ЛЕММЫ, плюс максимум {{new_limit}} новых.
2. Грамматика — только из списка РАЗРЕШЁННАЯ_ГРАММАТИКА. Ничего сверх него.
3. Целевой признак «{{target_feature}}» должен встретиться не менее {{target_min}} раз
   в естественных контекстах. Не выделяй его — он должен быть незаметной частью текста.
4. Каждое предложение — 6–14 слов. Без списков, без заголовков.
5. Пиши так, как пишут взрослому человеку. Не упрощай содержание — упрощай только язык.

ИЗВЕСТНЫЕ_ЛЕММЫ: {{known_lemmas}}
РАЗРЕШЁННАЯ_ГРАММАТИКА: {{allowed_features}}

ВЕРНИ СТРОГО JSON:
{
  "text": "...",
  "new_lemmas": [{"lemma":"...","gloss_ru":"...","gloss_en":"...","why":"почему без него нельзя"}],
  "target_feature_occurrences": [{"form":"...","sentence_index":0}]
}
```

## Failure modes and what the code does

| Failure | Frequency | Mitigation in code |
|---|---|---|
| Exceeds the new-lemma budget | common | `internal/llm/validate.go` re-analyses the text with `internal/morph`; over budget → reject and retry once with the offenders named |
| Uses forbidden grammar (typically conditional or imperative slipping in) | common | same re-analysis checks every token's feats against `allowed_features` |
| Grammatical but unidiomatic Hungarian | **common — this is the dangerous one** | `internal/corpus/attest.go` n-gram check; unattested spans flagged, text cannot become cards without override |
| Returns prose around the JSON | occasional | strict JSON extraction, one retry, then fail visibly |
| Silently translates the topic rather than writing about it | rare | length + topic-keyword check |

**Never auto-accept.** A generated text lands in the Library marked `unverified` until it passes
attestation, and the badge stays visible to the learner.
