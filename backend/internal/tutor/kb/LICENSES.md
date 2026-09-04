# The tutor's reference corpus — attribution ledger

Everything in this directory is read by the grammar tutor (`internal/tutor`, ADR-027) and
shown to a language model as reference material. None of it is shown to a learner as-is; the
model paraphrases it, and the answer is labelled with the model's name. Each file carries its
source, URL and licence in its front matter; this file is the ledger.

Like the language pack ([`langpack/LICENSES.md`](../../../../langpack/LICENSES.md)), the corpus
is a derived work of share-alike sources, so **the `src-*.md` files are CC BY-SA 4.0** even
though the application code is MIT. The hand-written `ref-*.md` files are original work
released under the same licence so the directory has one answer.

| Files | Source | Licence | Attribution |
|---|---|---|---|
| `src-wikipedia-*.md` | [English Wikipedia](https://en.wikipedia.org): *Hungarian grammar*, *Hungarian verbs*, *Hungarian noun phrase*, *Hungarian phonology*, *Hungarian orthography* | **CC BY-SA 4.0** | "Wikipedia contributors" — converted from wikitext by `langpack/pipeline/tutor_kb.py` |
| `src-wiktionary-*.md` | [English Wiktionary](https://en.wiktionary.org) appendices on Hungarian: suffixes, possessive suffixes, postpositions, pronouns, verbal prefixes, pronunciation, inessive case usage, antiharmonic words, v-stem words, fleeting-vowel words, third-person possessive forms, words of time, compound verbs, verbs used only with a prefix | **CC BY-SA 4.0** | "Wiktionary contributors, English Wiktionary" — converted as above |
| `src-fsi-*.md` | [*Hungarian Basic Course*](https://archive.org/details/Fsi-HungarianBasicCourse-StudentText), US Foreign Service Institute, 1962 — the "Notes on Grammar" of each unit | **public domain** (US government work) | Foreign Service Institute. OCR text from archive.org; diacritics the scan lost are restored from the language pack where exactly one attested form matches, and other tokens are left as scanned |
| `ref-*.md` | written for this app | **CC BY-SA 4.0** | tanulom contributors |

## Evaluated and excluded

- **Ignácz Singer, *Simplified Grammar of the Hungarian Language* (1882), Wikisource.** Public
  domain and a real grammar, but its orthography predates the reforms (`cz` for `c`, and older
  forms throughout). A small model reads reference text as normative; this one would teach it
  a spelling nobody uses.
- **Copyrighted modern grammars** (Rounds, *Hungarian: An Essential Grammar*; Törkenczy;
  Szita & Görbe). Not licensed for redistribution, so not here. The pack's own lessons are the
  modern reference the tutor leans on first.

## Rebuilding

```bash
make tutor-kb        # regenerates src-*.md from the pinned sources; ref-*.md are untouched
```

The outputs are committed: `internal/tutor/kb.go` embeds this directory with `//go:embed`, and
an embed of a missing file is a compile error. The download cache lives in
`langpack/cache/tutor/` and is gitignored.
