---
id: explain-form
version: 1
model_hint: any instruct model
cites: [R4, R15]
output: json
---

# Morphological debugger

Decomposes a Hungarian wordform and justifies **why each allomorph** was selected.

## Why

[R4](../research/04-explicit-instruction.md) — explicit rule statement beats inference
(g ≈ 1.29 vs 0.77). [R15](../research/15-hungarian-difficulty.md) — vowel harmony is the
first thing that must be automated, and automation starts from an explicit rule.

**Important:** this template is a *fallback and enrichment* path. The primary decomposition comes
from `internal/morph` using the language pack, which is deterministic and offline. The LLM adds
prose explanation and extra examples — it does **not** own the analysis.

## Template

```
Разбери венгерскую форму «{{form}}» по морфемам.

Известный разбор (из морфологической базы, считай его истиной):
  лемма: {{lemma}}
  признаки: {{feats}}
  сегментация: {{segments}}

Для КАЖДОЙ морфемы дай:
  - её форму,
  - функцию одним предложением,
  - если у неё есть алломорфы — почему выбран именно этот
    (гармония гласных: задний / передний неогубленный / передний огубленный;
     или иное морфонологическое правило).

Затем дай 3 ДРУГИХ глагола или существительных в той же самой форме,
с разными классами гармонии, чтобы был виден контраст.

ВЕРНИ СТРОГО JSON:
{
  "morphemes": [{"form":"...","function_ru":"...","function_en":"...",
                 "allomorph_rule_ru":"...","allomorph_rule_en":"..."}],
  "contrast_examples": [{"form":"...","lemma":"...","harmony_class":"back|front_unrounded|front_rounded",
                         "gloss_ru":"...","gloss_en":"..."}]
}
```

## Failure modes

| Failure | Mitigation |
|---|---|
| Invents a morpheme boundary that contradicts the pack | the pack's segmentation is passed in as ground truth and the response is checked against it; mismatch → discard the LLM output, show the deterministic breakdown alone |
| Contrast examples are not real words | each is looked up in `wordform`; unknown ones are dropped |
| Harmony class mislabelled | recomputed from the vowels in Go, not trusted from the model |
