# R17 — The Hungarian NLP and speech stack

Engineering reference rather than pedagogy. These are the resources the language pack and the
optional sidecars are built from, with licences, because licence compliance is a shipping
requirement.

Curated upstream index: **[awesome-hungarian-nlp](https://github.com/oroszgy/awesome-hungarian-nlp)**
(Orosz György) — the single best starting point for Hungarian NLP resources.

## Morphology and analysis

| Resource | What it gives | Licence | Where used |
|---|---|---|---|
| [HuSpaCy](https://github.com/huspacy/huspacy) `hu_core_news_lg` | tokenisation, sentence split, POS (UD), **morphological features**, lemmatisation, dependency parse, NER, vectors. **Lemma 97.6%, POS 96.7%** | Apache-2.0 (models CC BY-SA) | `langpack/` build; optional `services/nlp` |
| [UD Hungarian](https://github.com/UniversalDependencies/UD_Hungarian) | the feature inventory; gold data for golden tests | CC BY-SA | `langpack/`, test fixtures |
| [emMorph](https://github.com/dlt-rilmta/emMorph) / [emmorphpy](https://github.com/ppke-nlpg/emmorphpy) | finite-state analysis, better on rare forms | see repo | unknown-form fallback rules |
| [morphdb.hu](http://mokk.bme.hu/resources/morphdb-hu/) | morphological database | see repo | fallback suffix rules |
| [hunspell](http://hunspell.github.io/) `hu_HU` | spell/affix data | LGPL/MPL/GPL tri-licence | validation |

**UD Hungarian feature inventory** — the atoms of `morph_state`:

- **Case** (22): `Nom Gen Dat Acc Ins Abl Abs Ade All Cau Del Dis Ela Ess Ill Ine Loc Sbl Sup Tem Ter Tra`
- **Number**: `Sing Plur` · **Person**: `1 2 3`
- **Number[psor]**, **Person[psor]**, **Number[psed]** — possessive marking
- **Mood**: `Ind Imp Cnd Pot` · **Tense**: `Past Pres`
- **VerbForm**: `Inf Fin PartFut PartPres PartPast Conv`
- **Voice**: `Act Cau` · **Aspect**: `Iter` · **Degree**: `Pos Cmp Sup`
- **Definite**, **PronType**, **NumType**, **Poss**, **Reflex**
- POS: all 17 universal tags; `SYM` unattested.

Source: https://universaldependencies.org/hu/index.html

## Lexicon and glosses

| Resource | What | Licence |
|---|---|---|
| [kaikki.org Hungarian](https://kaikki.org/dictionary/Hungarian/) via [wiktextract](https://github.com/tatuylonen/wiktextract) | glosses, IPA, **full inflection tables** in the `forms` field. 72,781 word forms; 49,653 noun senses; 17,730 verb senses | CC BY-SA / GFDL |
| Russian Wiktionary edition (same toolchain) | RU glosses — **materially thinner than EN**, hence the EN fallback | CC BY-SA |
| [Hungarian WordNet](https://github.com/mmihaltz/huwn) | sense relations | see repo |

Note: kaikki's per-language pages are being deprecated in favour of the
[raw data downloads](https://kaikki.org/dictionary/rawdata.html) — pin the raw dump, not the
per-language file.

## Corpora and frequency

| Resource | What | Licence |
|---|---|---|
| [Hungarian Webcorpus 2.0](https://hlt.bme.hu/en/resources/webcorpus2) | large corpus → **n-gram attestation index**, lemma frequency | CC BY-SA 4.0 (Wikipedia subcorpus); Common Crawl terms for that subcorpus |
| [hermitdave/FrequencyWords](https://github.com/hermitdave/FrequencyWords) `content/2018/hu/hu_50k.txt` | OpenSubtitles frequency ranks — **spoken-register weighted**, which suits a learner | CC BY-SA 4.0 |
| [Tatoeba](https://tatoeba.org/en/downloads) | example sentences, HU↔EN/RU links, some audio. 479,026 HU translation links | CC BY 2.0 FR; some CC0. **Audio licence is per-clip — must be filtered** |
| [OPUS / OpenSubtitles](https://opus.nlpl.eu/OpenSubtitles/) | parallel data | per-corpus |
| [Szeged Treebank](http://rgai.inf.u-szeged.hu/), [NerKor](https://github.com/dlt-rilmta/NerKor) | annotated data | see repo |

## Speech

| Resource | What | Licence |
|---|---|---|
| [Piper](https://github.com/rhasspy/piper) + [piper-voices `hu_HU`](https://huggingface.co/rhasspy/piper-voices/tree/main/hu/hu_HU) | **3 voices: `anna`, `berta`, `imre`** (medium). VITS-style, ~15M params, ONNX, CPU real-time | MIT |
| [Common Voice Hungarian](https://commonvoice.mozilla.org/hu/datasets) | **human talkers** — the fix for Piper's 3-voice shortfall ([R7](07-hvpt.md)) | **CC0** |
| [Trendency/whisper-large-v3-hu](https://huggingface.co/Trendency/whisper-large-v3-hu) | Hungarian ASR. **WER ≈ 11.26%** on Common Voice 19/20/21 deltas | see model card |
| [CSS10](https://github.com/Kyubyong/css10), [Hungarian single-speaker set](https://www.kaggle.com/bryanpark/hungarian-single-speaker-speech-dataset) | TTS training data | see source |

## Hungarian LLMs (optional `llm` profile)

| Model | Note |
|---|---|
| [Racka-4B](https://huggingface.co/elte-nlp/Racka-4B) (ELTE NLP) | Qwen3-4B adaptation; reported to beat PULI-8B on HuLU |
| [PULI-GPTrio](https://huggingface.co/NYTK/PULI-GPTrio), PULI-LlumiX-Llama-3.1-8B (NYTK) | long-standing Hungarian SOTA |
| [SambaLingo-Hungarian](https://huggingface.co/sambanovasystems/SambaLingo-Hungarian-Base) | alternative |
| `qwen3`, `gemma3` via Ollama | general-purpose, acceptable Hungarian |

**Hungarian LLM benchmarks** (why the attestation guardrail exists — [R10](10-llm-tutors.md)):
**HuLU** (understanding, https://github.com/nytud/HuLU), **OpenHuEval** (Hungarian-specific
knowledge), **HuGME** (hallucination, relevance, bias).

## What this changes in our code

**Runtime morphology must not require Python.** The core is a static Go binary
([`docs/ARCHITECTURE.md`](../../docs/ARCHITECTURE.md)). Resolution:

1. **Build time** — HuSpaCy analyses corpora and expands Wiktionary paradigms into a
   `wordform → (lemma, upos, feats)` table shipped in the language pack.
2. **Runtime** — the Go binary does an **indexed lookup**, not analysis. O(1), no dependencies.
3. **Unknown forms** — a suffix-stripper built from `morph_feature` allomorph tables.
4. **Ambiguity** — when the optional `nlp` sidecar is running, HuSpaCy disambiguates in context
   (`vár` = castle vs. waits). Without it, the pack's frequency prior is used and the UI marks
   the analysis as uncertain.

See [`docs/LANGPACK.md`](../../docs/LANGPACK.md) for the pipeline and
[`langpack/LICENSES.md`](../../langpack/LICENSES.md) for the attribution ledger.
