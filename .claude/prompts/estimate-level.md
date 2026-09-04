---
id: estimate-level
version: 1
model_hint: any OpenAI-compatible chat model; short completion, temperature 0
cites: R6 (lexical coverage — why the heuristic exists and stays primary)
---

# Estimate a text's CEFR level

The reader's second opinion. The PRIMARY estimate is the offline heuristic in
`internal/mining/level.go` (pack frequency bands + grammar-topic ceilings + sentence length);
this prompt refines it when a model is configured, and the result is stored with
`level_source='llm'` and the model's name — the UI labels it every time (ADR-023, the ADR-014
labelling rule). The learner's own `manual` verdict is never overwritten.

## Template

System:

```
You judge the CEFR difficulty of Hungarian texts for learners. Answer with ONLY a JSON object:
{"level":"A1|A2|B1|B2|C1|C2","confidence":0.0-1.0,"rationale":"one short sentence"}. No prose
around it. Judge the language, not the topic.
```

User:

```
Title: {title}

Text:
{first ~2000 bytes of the body, cut on a UTF-8 boundary}

A frequency-based heuristic estimated: {heuristic level, when present}
```

The Go constant `levelSystemPrompt` in `internal/api/handlers_level.go` must stay in step with
the system block above; bump `version` here on any semantic edit.

## Why

Lexical coverage drives comprehensibility ([R6]), and the heuristic measures exactly that — but
it is blind to idiom, register and syntax beyond sentence length. A chat model sees those. It
also hallucinates, which is why it gets the last word only over the heuristic, never over the
learner, and why its answer is a constrained enum rather than free text.

## Known failure modes

- **Prose around the JSON, or code fences.** `parseLevelVerdict` extracts the outermost
  `{...}` before parsing; anything unparseable → 502 and the stored estimate stays untouched.
- **A level outside the enum** ("B1-B2", "intermediate"). Refused by `mining.ValidCEFR`;
  stored estimate untouched.
- **Judging the topic, not the language** (a simply-written text about quantum physics rated
  C1). The system prompt names the failure; the confidence field gives the UI something to
  show; the heuristic estimate in the user message anchors the model.
- **The model parrots the heuristic.** Acceptable — agreement is information too — but the UI
  labels the source as the model, so a learner can still see who said what in `level_detail`.
