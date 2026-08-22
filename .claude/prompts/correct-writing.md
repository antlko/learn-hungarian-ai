---
id: correct-writing
version: 1
model_hint: strongest available; this is the highest-stakes template
cites: [R9, R1, R15]
output: json
---

# Writing correction — prompt first, reveal second

## Why

[R9](../research/09-corrective-feedback.md) — corrective feedback works (d = 0.64), explicit
correction has the larger immediate effect (d = 0.81), and **prompts beat recasts**: feedback
that makes the learner produce the repair is encoded better than feedback that hands it over.
[R1](../research/01-retrieval-practice.md) — self-repair is retrieval; being told is not.

This mirrors the source plan's own §6.1 #3: *«Не "исправь", а "исправь и дай дриллы"»*.

## The three-stage contract

The model must produce all three stages at once; **the server releases them one at a time** and
requires a learner attempt before stage 3. Enforced in `internal/api/handlers_write.go`, not in
the UI — so it survives a page reload.

| Stage | Content |
|---|---|
| 1 · Locate | which clause, what *kind* of problem — never the answer |
| 2 · Hint | a question that points at the rule |
| 3 · Reveal | correction + one-sentence rule + 3 new practice items |

## Template

```
Ты — преподаватель венгерского. Ниже текст ученика.

ДЛЯ КАЖДОЙ ОШИБКИ верни три стадии:
  (1) locate  — укажи предложение и ТИП проблемы, НЕ давая ответа.
                Пример: «В предложении 2 проблема с согласованием глагола и объекта.»
  (2) hint    — наводящий ВОПРОС, который указывает на правило.
                Пример: «Посмотри на объект — он определённый?»
  (3) reveal  — исправленная форма, правило ОДНИМ предложением,
                и 3 НОВЫХ предложения на то же правило, где ученик выбирает форму.

ЖЁСТКО:
- НЕ комментируй то, что написано верно. Вообще.
- НЕ переписывай текст целиком. Только точечные ошибки.
- Один пункт = одна ошибка. Не объединяй.
- Стиль/вкусовщину не трогай, только объективные ошибки.

ТЕКСТ УЧЕНИКА:
{{text}}

УРОВЕНЬ: {{level}}   ИЗУЧЕННЫЕ ПРИЗНАКИ: {{known_features}}

ВЕРНИ СТРОГО JSON:
{
  "errors": [{
    "sentence_index": 0,
    "span": "...",
    "error_class": "harmony|case|definiteness|possessor|vowel_length|agreement|word_order|lexis|spelling",
    "feature_id": "Case=Ine",
    "locate_ru": "...", "locate_en": "...",
    "hint_ru": "...",   "hint_en": "...",
    "correction": "...",
    "rule_ru": "...",   "rule_en": "...",
    "drills": [{"prompt":"...","options":["...","..."],"answer":"..."}]
  }]
}
```

## Failure modes

| Failure | Mitigation |
|---|---|
| "Corrects" something that was already right | `error_class` + span re-checked with `internal/morph/diff.go`; a correction that the deterministic analyser agrees was fine is dropped |
| Gives the answer inside `locate` or `hint` | server-side check: if `correction` appears as a substring of `locate`/`hint`, the item is regenerated |
| Drill items are unidiomatic | every drill sentence passes `internal/corpus/attest.go` before display |
| `feature_id` not in the UD inventory | validated against `morph_feature`; unknown → the error is kept but generates no drill |
| Stylistic nagging | prompt forbids it; long error lists with `error_class: lexis` are truncated |
