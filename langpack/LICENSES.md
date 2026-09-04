# Language pack — source attribution ledger

The Hungarian language pack is a **derived work** of the sources below. Several are CC BY-SA,
which is share-alike, so **the pack as a whole is released under CC BY-SA 4.0** — even though the
application code is MIT.

This is a legal obligation, not a courtesy. This file is surfaced in the app under
**Settings → Data → Sources**, and shipped inside the pack as `pack_meta.attribution`.

## Sources

### Lexicon

| Source | Used for | Licence | Attribution |
|---|---|---|---|
| [English Wiktionary](https://en.wiktionary.org) via [wiktextract](https://github.com/tatuylonen/wiktextract) / [kaikki.org](https://kaikki.org/dictionary/Hungarian/) | EN glosses, IPA, full inflection tables | **CC BY-SA 3.0 / GFDL** | "Wiktionary contributors, English Wiktionary" |
| [Russian Wiktionary](https://ru.wiktionary.org) via wiktextract | RU glosses | **CC BY-SA 3.0 / GFDL** | "Wiktionary contributors, Russian Wiktionary" |
| [Hungarian WordNet](https://github.com/mmihaltz/huwn) | sense relations | see repo | Miháltz Márton et al. |

### Corpora & frequency

| Source | Used for | Licence | Attribution |
|---|---|---|---|
| [Tatoeba](https://tatoeba.org) | example sentences, HU↔EN/RU links | **CC BY 2.0 FR**; some sentences CC0 | "Tatoeba Project contributors" |
| Tatoeba **audio** | HVPT + sentence audio | **per-clip — set by the contributor** | recorded per clip in `pair_audio.license`; clips without a permissive licence are excluded |
| [hermitdave/FrequencyWords](https://github.com/hermitdave/FrequencyWords) (`hu_50k`, OpenSubtitles) | frequency ranks | **CC BY-SA 4.0** | Hermit Dave; derived from OpenSubtitles via OPUS |
| [OPUS OpenSubtitles](https://opus.nlpl.eu/OpenSubtitles/) v2018, `hu-ru` and `en-hu` | conversational example sentences and their translations | **free for research use**; the corpus is derived from opensubtitles.org and OPUS asks that the paper be cited rather than granting an explicit content licence | P. Lison and J. Tiedemann, *OpenSubtitles2016: Extracting Large Parallel Corpora from Movie and TV Subtitles*, LREC 2016 |
| [Hungarian Webcorpus 2.0](https://hlt.bme.hu/en/resources/webcorpus2) | n-gram attestation index, lemma frequency | **CC BY-SA 4.0** (Wikipedia subcorpus); Common Crawl terms for the CC subcorpus | SZTAKI-HLT / BME |
| [UD Hungarian](https://github.com/UniversalDependencies/UD_Hungarian) | feature inventory, golden test data | **CC BY-SA 4.0** | Universal Dependencies contributors |

#### OpenSubtitles and redistribution — the line this project draws

OPUS OpenSubtitles is the standard free source of conversational Hungarian and the frequency
list above is already derived from it. Its licensing is weaker than Tatoeba's, though: OPUS
distributes it for research and asks for a citation, which is not the same as a licence to
embed film dialogue in a binary we publish. So:

- The **full pack** is built on the learner's own machine, by them, from sources they fetch.
  It may contain these sentences, marked `sentence.source = 'opensubtitles'`.
- The **starter pack**, which is compiled into the distributed executable, takes
  `source = 'tatoeba'` only. `pipeline/starter.py` enforces this with an allow-list rather than
  a deny-list, so a future source stays out until someone decides otherwise.

Nothing in the selection is generated: every sentence is one a person wrote and every
translation is one a human translator produced. The pipeline only chooses — see
`pipeline/subtitles.py`.

### Tooling (build-time only — not redistributed in the pack)

| Source | Used for | Licence |
|---|---|---|
| [HuSpaCy](https://github.com/huspacy/huspacy) + `hu_core_news_lg` | morphological analysis and lemmatisation at build time | Apache-2.0 (library); models CC BY-SA |
| [emMorph](https://github.com/dlt-rilmta/emMorph) / [morphdb.hu](http://mokk.bme.hu/resources/morphdb-hu/) | fallback suffix rules | see repos |

### Evaluation only — never shipped

| Source | Used for | Licence | Attribution |
|---|---|---|---|
| [UD Hungarian-Szeged](https://github.com/UniversalDependencies/UD_Hungarian-Szeged) | scoring the morphology against human annotation (`backend/internal/morph/golden_ud_test.go`) | **CC BY-NC-SA 3.0** | Szeged Dependency Treebank, converted to UD (Vincze et al.) |

The **NC** matters and is why this sits in its own section: nothing derived from this treebank
enters the pack, no text from it reaches a learner, and the file is fetched into
`langpack/cache/` rather than committed. It is a measuring stick held against the pack from
outside — held *inside*, it would put a non-commercial restriction on a pack released CC BY-SA.

### Audio

| Source | Used for | Licence | Attribution |
|---|---|---|---|
| [Piper](https://github.com/rhasspy/piper) voices `hu_HU-anna`, `hu_HU-berta`, `hu_HU-imre` | synthetic talkers, card audio | **MIT** (models per voice card — verify each) | Rhasspy / Piper contributors |
| [Mozilla Common Voice — Hungarian](https://commonvoice.mozilla.org/hu/datasets) | **human talkers for HVPT** | **CC0 1.0** | Mozilla Common Voice contributors |

## Obligations this creates

1. **Attribution.** Every CC BY / CC BY-SA source is named here, in the app, and in
   `pack_meta.attribution` inside the pack itself.
2. **Share-alike.** The pack is CC BY-SA 4.0. Anyone redistributing a modified pack must do the
   same. This does **not** extend to the application code (MIT) — the pack is data the program
   reads, not a derivative of the program.
3. **Per-clip audio filtering.** Tatoeba audio licences are chosen by the individual contributor.
   `pipeline/sentences.py` **must** check each clip and record the licence in `pair_audio.license`;
   clips whose licence cannot be resolved are excluded. This is a pack quality gate.
4. **No paywalled or scraped content.** The pipeline fetches only the pinned, publicly licensed
   sources listed above, verified by SHA256.

## Verifying

```bash
cd langpack && python -m pipeline.verify_licenses --pack ../dist/langpack-hu-v1.sqlite
```

Checks that every `pair_audio` and `sentence` row carries a resolvable licence, and that
`pack_meta.attribution` matches this file. It runs as a release gate in
`.github/workflows/langpack.yml`.
