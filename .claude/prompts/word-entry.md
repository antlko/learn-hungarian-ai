---
id: word-entry
version: 2
model_hint: 7–8B instruct; below that the meanings drift and the examples invent stems
cites: [R10]
output: json
---

# Dictionary entry for a word the pack does not have

Produces a **displayed, labelled** dictionary entry for one Hungarian surface form: dictionary
form, part of speech, meanings in the learner's language, a morpheme breakdown, and a couple of
example sentences.

## Why

The language pack is finite and the reader is not. A learner who clicks `bejelentkezve` in a
pasted text gets «В языковом пакете этого нет.» and a dead end — no meaning, no breakdown, no
way forward, on precisely the word they stopped for. The pack's own entry (the `nincs` case) is
richer than anything a model should be trusted to produce, which is why this template runs
**only** where the pack has nothing.

[R10](../research/10-llm-tutors.md) is the reason for every guardrail below: Hungarian is where
models are weakest, and a confident wrong entry is worse than a dead end because the learner has
no way to check it.

## What this output may and may not become

- It is **displayed**, labelled with the model that wrote it.
- Every Hungarian word in it is looked up in the pack, and the ones the dictionary has never
  seen are listed under the entry — `tutor.Backticked` + `tutor.Unattested`, the same check the
  tutor's answers get.
- Its meaning reaches `user_gloss` **only** on an explicit save, as `source='machine'` with the
  model recorded, and reaches a **card** only after «Принять как своё» re-saves it as `manual`
  (ADR-014, ADR-035). Nothing here is ever a card's back on its own.
- The example sentences are **never** material. They are not attested (`internal/corpus` does
  not exist), they do not become notes, and the UI says so.

## Template

System:

```
You are a Hungarian-to-{{lang_name}} dictionary. Answer ONLY with a JSON object.

Schema:
{
  "lemma":    "the dictionary form of the word",
  "pos":      "one of: NOUN VERB ADJ ADV NUM PRON PROPN DET CONJ ADP PART INTJ",
  "meanings": ["short gloss", "another sense", "..."],
  "morphemes":["stem", "-suffix", "..."],
  "examples": [{"hu": "a short Hungarian sentence using the word",
                "tr": "its translation into {{lang_name}}"}]
}

Rules:
- Wrap every Hungarian word you write in backticks, INSIDE the quotes: "lemma": "`ház`".
  Never put a backtick around a quoted value — that is not JSON and it cannot be read.
- At most 3 meanings, at most 3 morphemes, at most 2 examples.
- Meanings are in {{lang_name}}, short, no sentences, no grammar labels.
- `morphemes` splits the FORM you were given, not the lemma: stem first, then each suffix
  written with its leading hyphen.
- If you do not know the word, return {"unknown": true} and nothing else.
- No prose, no markdown fences, no commentary. JSON only.
```

User:

```
{{form}}
```

## Known failure modes, and what the code does

| Failure | Mitigation |
| --- | --- |
| Not JSON at all (fences, prose preamble) | `handlers_ai_entry.go` strips ``` fences and takes the outermost `{…}`; anything still unparseable is returned as `raw` and rendered as prose, labelled — fence stripped, because the learner reads the words and not the wire format |
| Backticks around the VALUE rather than inside it — ``"lemma": `"feltehetek"`,`` — which no parser takes | version 2 says where the ticks go (rule 1); the handler ALSO retries the decode with every backtick removed, since they are decoration stripped from each field anyway. gemma3:4b did this on `feltehetek` and the learner got a screenful of raw JSON |
| Invented stem or a non-existent form in an example | every backticked token goes through `s.wordExists()`; misses are listed as `unattested` under the entry |
| Confident entry for a word that is not Hungarian | `{"unknown": true}` is honoured and shown as "the model does not know it either" — the dead end, but an honest one |
| Meanings in Hungarian instead of the learner's language | the language is named twice (system rule + schema comment); a wrong-language gloss is visible to the learner immediately and is never saved without their press |
| Backticks omitted, so nothing is checked | the entry is still shown, but `unattested` is then empty — which would be a silent loss of the guardrail, so the handler ALSO checks `lemma` and every `examples[].hu` token whether or not they were backticked |

## What it must never be asked to do

Generate study material. A card's Hungarian must be attested (`internal/corpus`, Phase 9), and
that package does not exist. Examples here are illustration on a panel, and the panel says so.
