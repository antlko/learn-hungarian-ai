---
id: generate-drill
version: 1
model_hint: any instruct model
cites: [R1, R5, R15]
output: json
---

# Contrastive drill generator

Produces practice items targeting **one** morphological feature, with distractors that are
minimally different.

## Why

[R1](../research/01-retrieval-practice.md) — effortful retrieval with feedback is the mechanism.
[R5](../research/05-interleaving.md) — when items *are* interleaved, the benefit comes from **low
between-category discriminability**: contrasting items must be genuinely confusable, not merely
different. Random variety does nothing.
[R15](../research/15-hungarian-difficulty.md) — for definiteness, item distribution mirrors the
documented error distribution.

## Template

```
Сгенерируй {{count}} упражнений на признак: {{feature_label}} ({{feature_id}}).

ФОРМАТ КАЖДОГО ПУНКТА:
  - предложение с ПРОПУСКОМ на целевой форме,
  - ровно {{n_options}} варианта,
  - неправильные варианты должны быть МИНИМАЛЬНО отличными от правильного
    (другой алломорф гармонии, другой падеж того же ряда, другое спряжение),
    а НЕ случайными словами. Ученик должен реально выбирать, а не угадывать.

ОГРАНИЧЕНИЯ:
- Лексика только из: {{known_lemmas}}
- Грамматика только из: {{allowed_features}} + сам целевой признак
- Длина предложения 6–14 слов
- Контексты бытовые и разные: {{contexts}}
{{#if definiteness}}
- РАСПРЕДЕЛЕНИЕ (по данным корпуса ошибок HunLearner):
  {{def_object_pct}}% пунктов — определённый объект,
  {{demonstrative_pct}}% — объект-указательное местоимение (это самый частый провал),
  остальное — неопределённый объект.
{{/if}}

ВЕРНИ СТРОГО JSON:
{
  "items": [{
    "sentence": "... ___ ...",
    "options": ["...","..."],
    "answer": "...",
    "distractor_rationale": ["почему этот вариант правдоподобен", "..."],
    "rule_ru": "...", "rule_en": "..."
  }]
}
```

## Failure modes

| Failure | Mitigation |
|---|---|
| Distractors are obviously wrong → item measures nothing | `internal/drill/validate.go` checks each distractor is a real form of a plausible lemma and differs from the answer by ≤2 morphological features; failures are regenerated |
| The blank does not actually require the target feature | the sentence is re-analysed; if the blanked span's feats lack `feature_id`, the item is discarded |
| Vocabulary drift outside the known set | full re-analysis against `known_lemmas` |
| The "correct" answer is wrong | attestation check on the completed sentence; unattested → discard |

**Generated drills are cached and reused.** Regenerating identical items every session wastes
tokens and, worse, introduces variance in item quality. Items live in `drill_item` with the
template version that produced them.
