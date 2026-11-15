# langpack/talk

Colloquial material for the reading library (ADR-077): per basic-tier unit one **talk text** (a 4th text in real
spoken Hungarian) and **5-10 mini dialogues**. One file per unit, named by the unit code
(`m1-u1-koszones.json`). Everything here is AI-written (`"checked": "ai"`) until a native speaker has read it.

```json
{"talk": {"genre": "dialogue", "checked": "ai", "title": {"ru": "...", "en": "..."},
          "names": ["Kata"], "paras": [[{"hu": "...", "ru": "...", "en": "..."}]]},
 "dialogues": [{"slug": "ajtonal", "title": {"ru": "...", "en": "..."},
                "scene": {"ru": "one line: where, who", "en": "..."}, "names": [], "checked": "ai",
                "turns": [{"who": "A", "hu": "...", "ru": "...", "en": "..."}]}]}
```

- `genre` of the talk text: `dialogue`, `phone`, `chat` (turn-based; optional `who` A|B|C on every entry or
  none) or `voicemsg` (prose, 1-4 paragraphs).
- A mini is always a `dialogue`; `slug` is kebab-case and unique in the unit; the order in the file is the order
  in the library.
- Sizes, the word rules (V1'), the colloquial lexicon `pipeline/texts_colloquial.tsv` and the banned list:
  `docs/LANGPACK.md` "Colloquial talk texts and mini dialogues". Spoken words must come from the lexicon (or the
  pack); vulgar words are banned everywhere.

Author: `cd langpack && .venv/bin/python -m pipeline.texts PACK --brief UNIT --tier talk` (or `mini`), write the
file, then `make texts PACK=...` (validates, rewrites `backend/internal/readsets/catalog.json`), commit.
`--require-talk` checks that every unit has a talk text and at least 5 minis.
