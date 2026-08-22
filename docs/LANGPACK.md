# The language pack

A versioned, read-only SQLite file containing everything the app knows about Hungarian. It is
built offline by `langpack/` and shipped as a release asset. **The core binary never runs a
morphological analyser — it does indexed lookups.** See
[ADR-002](../.claude/DECISIONS.md#adr-002).

The pack is the ceiling on product quality. If lemmatisation is wrong, `n+1` selection and the
mastery heatmap are wrong. Treat the golden test as a release gate.

---

## 1. Why a pack exists

The best Hungarian analyser (HuSpaCy `hu_core_news_lg`: lemma 97.6%, POS 96.7%) is Python. The
core is a static Go binary. Resolution: run HuSpaCy **at build time** and ship the results.

**Lookup ladder at runtime** — `internal/morph/lookup.go`:

| Step | Condition | Result |
|---|---|---|
| 1 | exact `wordform` hit, single analysis | done, `certain` |
| 2 | exact hit, multiple analyses | frequency prior wins; flagged `uncertain` |
| 3 | no hit | suffix-stripper from `morph_feature` allomorphs; flagged `derived` |
| 4 | `nlp` sidecar reachable | HuSpaCy disambiguates in context (`vár` = castle vs. waits) |

Steps 1–3 are fully offline with zero dependencies. Step 4 is an upgrade, never a requirement.

---

## 2. Schema

```sql
-- ─── lexicon ────────────────────────────────────────────────────────────
lemma(id INTEGER PK, lemma TEXT, upos TEXT, freq_rank INTEGER,
      freq_pm REAL, cefr TEXT, ipa TEXT, flags INTEGER)
lemma_gloss(lemma_id, lang TEXT CHECK(lang IN ('ru','en')), sense_idx INTEGER, gloss TEXT)

-- the hot table: ~3–6 M rows, the single most important index in the product
wordform(form TEXT, lemma_id INTEGER, upos TEXT, feats TEXT, freq INTEGER)
CREATE INDEX idx_wordform_form ON wordform(form);

paradigm(lemma_id INTEGER, slot TEXT, form TEXT)   -- full declension/conjugation tables

-- ─── the second state space's vocabulary ────────────────────────────────
morph_feature(id TEXT PK,               -- e.g. 'Case=Ine'
              key TEXT, value TEXT,
              label_ru TEXT, label_en TEXT,
              allomorphs TEXT,          -- JSON: {"back":"-ban","front_unrounded":"-ben", …}
              explain_md_ru TEXT, explain_md_en TEXT,
              teach_order INTEGER, prereq_ids TEXT)

-- ─── input material ─────────────────────────────────────────────────────
sentence(id, hu TEXT, source TEXT, license TEXT,
         len_tokens INTEGER, difficulty REAL, audio_ref TEXT)
sentence_trans(sentence_id, lang, text)
sentence_token(sentence_id, idx, form, lemma_id, feats)   -- pre-analysed → n+1 is a SQL join

-- ─── phonetics ──────────────────────────────────────────────────────────
minimal_pair(id, a TEXT, b TEXT, contrast TEXT,
             ipa_a TEXT, ipa_b TEXT, context_sentence TEXT)
pair_audio(pair_id, member TEXT CHECK(member IN ('a','b')),
           talker_id TEXT, talker_kind TEXT,   -- 'piper' | 'human'
           rate REAL, path TEXT, license TEXT)

-- ─── curriculum ─────────────────────────────────────────────────────────
grammar_topic(id, code TEXT, level TEXT,
              title_ru, title_en, explain_md_ru, explain_md_en,
              prereq_ids TEXT, feature_ids TEXT, teach_order INTEGER)

-- ─── the guardrail ──────────────────────────────────────────────────────
ngram(gram TEXT, n INTEGER, count INTEGER)
CREATE INDEX idx_ngram_gram ON ngram(gram);

-- ─── metadata ───────────────────────────────────────────────────────────
pack_meta(key TEXT PK, value TEXT)   -- version, schema_version, built_at, source checksums
```

**Compatibility.** `pack_meta.schema_version` is checked at startup by `internal/lang`. A major
mismatch refuses to start with an actionable message rather than failing later in a confusing way.

**User overrides never touch the pack.** A learner who reports a wrong analysis writes to
`morph_correction` in `tanulom.db`, applied as an overlay at lookup time.

---

## 3. Sources and licences

Full attribution ledger: [`../langpack/LICENSES.md`](../langpack/LICENSES.md). Surfaced in-app
under Settings → Data.

| Source | Used for | Licence |
|---|---|---|
| [kaikki.org Hungarian](https://kaikki.org/dictionary/Hungarian/) (wiktextract) | glosses, IPA, **full inflection tables**; 72,781 forms, 49,653 noun + 17,730 verb senses | CC BY-SA / GFDL |
| Russian Wiktionary edition, same toolchain | RU glosses — **thinner than EN**, hence fallback | CC BY-SA |
| [Tatoeba](https://tatoeba.org/en/downloads) | example sentences, HU↔EN/RU links, some audio | CC BY 2.0 FR / CC0; **audio licence is per-clip — filter** |
| [hermitdave/FrequencyWords](https://github.com/hermitdave/FrequencyWords) `hu_50k` | frequency ranks (OpenSubtitles, spoken-register weighted) | CC BY-SA 4.0 |
| [Hungarian Webcorpus 2.0](https://hlt.bme.hu/en/resources/webcorpus2) | n-gram attestation index, lemma frequency | CC BY-SA 4.0 (Wikipedia subcorpus) |
| [UD Hungarian](https://github.com/UniversalDependencies/UD_Hungarian) | feature inventory; **golden test data** | CC BY-SA |
| [HuSpaCy](https://github.com/huspacy/huspacy) `hu_core_news_lg` | build-time analysis | Apache-2.0 (models CC BY-SA) |
| [Common Voice HU](https://commonvoice.mozilla.org/hu/datasets) | **human talkers for HVPT** | CC0 |
| [Piper hu_HU](https://huggingface.co/rhasspy/piper-voices/tree/main/hu/hu_HU) — anna, berta, imre | synthetic talkers, card audio | MIT |
| [morphdb.hu](http://mokk.bme.hu/resources/morphdb-hu/) / [emMorph](https://github.com/dlt-rilmta/emMorph) | fallback suffix rules | see repo |

**Attribution is a shipping requirement, not a nicety.** CC BY-SA obliges attribution and
share-alike on derived data. The pack is therefore released under **CC BY-SA 4.0** even though
the application code is MIT. Both facts are stated in the README.

---

## 3a. Source quality — what we learned by actually parsing it

Findings from the real extract, recorded because they are not obvious and cost
real debugging time.

### Wiktionary noun and adjective tables are excellent; verb tables are unusable

The wiktextract Hungarian extract (~580 MB, 78,730 raw entries) yields **75,224
usable lemmas, all glossed, 70k with IPA**. But its inflection tables split sharply
by part of speech:

| POS | Forms kept | Verdict |
|---|---:|---|
| NOUN | ~971,000 | trustworthy |
| ADJ | ~352,000 | trustworthy |
| NUM / PRON | ~25,000 | trustworthy |
| **VERB** | **0 — rejected** | **unusable** |

Two distinct problems, both real:

1. **`error-unrecognized-form`.** wiktextract sets this tag when it fails to parse
   a table. On Hungarian verb tables the surviving rows are **shifted by one**, so
   `megyek` (1sg) arrives carrying the tags of the next row — `second-person`.
   Importing that would write wrong `Person`/`Number` values into `morph_state`.
   Dropping these rows takes the corpus from 2.75 M forms to 1.44 M.
2. **Unsplit table cells.** Even among rows without the error tag, verb tables
   emit cells like `menve (menvén)`, `mehetve / mehetvén`, and literally
   `intransitive verb` as pseudo-forms, with a `Person` attached to converbs.

**Decision: `TRUSTED_FORM_POS` excludes VERB.** Since the definite/indefinite
conjugation is the single most important thing this app teaches
([R15](../.claude/research/15-hungarian-difficulty.md)), a plausible-looking but
wrong verb paradigm is strictly worse than no paradigm. Verb forms come from the
generator and from corpus attestation instead.

A form must also be a single orthographic token of Hungarian letters
(`_is_plausible_form`) to be imported at all.

### Possessive columns re-tag person and number

Inside a possessive column, `first-person` describes the **possessor**, not the
word. Mapped to `Person[psor]` / `Number[psor]`, or `házam` would land in the same
bucket as a first-person verb.

### Russian glosses are not in this source

The English Wiktionary edition gives EN glosses only. RU glosses require the
Russian edition through the same toolchain. Until that stage lands, lemmas carry
EN glosses and the pack manifest records `gloss_langs: ["en"]`, so the UI shows
its documented fallback badge rather than silently presenting English as Russian
([ADR-004](../.claude/DECISIONS.md#adr-004)).

---

## 4. Pipeline

```
fetch → normalise → analyse → merge → score → emit
```

| Stage | Module | Does |
|---|---|---|
| fetch | `pipeline/fetch.py` | pinned URLs + SHA256 into `cache/`; **refuses to proceed on checksum mismatch** — reproducibility over convenience |
| normalise | `wiktionary.py`, `wiktionary_ru.py`, `frequency.py` | JSONL → `lemma`, `lemma_gloss`, `paradigm`, IPA; frequency → ranks |
| analyse | `analyse.py` | HuSpaCy over Tatoeba + a Webcorpus sample → `sentence_token`; expand `wordform` from **corpus-attested forms ∪ Wiktionary paradigms** |
| merge | `features.py` | resolve lemma identity across sources; derive `morph_feature` incl. allomorph tables from harmony class |
| score | `sentences.py`, `minimal_pairs.py`, `curriculum.py`, `ngram.py` | sentence difficulty; CEFR estimate per lemma; contrast inventory + audio rendering; topic DAG; n-gram counts |
| emit | `emit.py` | write SQLite, create indexes, `VACUUM`, checksum, `manifest.json` |

**Why `wordform` is a union.** Corpus-attested forms give real-world coverage including
irregulars; Wiktionary paradigms give systematic coverage including forms that are rare but
perfectly valid. Neither alone is sufficient for an agglutinative language.

### Running it

```bash
cd langpack
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
python -m spacy download hu_core_news_lg    # ~700 MB
python -m pipeline.build --out ../dist/langpack-hu-v1.sqlite
```

Expect hours and tens of GB of scratch space. This runs in CI on tag
(`.github/workflows/langpack.yml`), not on a developer's laptop, and never inside the app image.

---

## 5. Distribution

| Mode | How | For |
|---|---|---|
| **Download on first run** (default) | setup wizard fetches from GitHub Releases, verifies SHA256, shows progress | most users; keeps the image small |
| **Baked in** | `docker build --build-arg BAKE_LANGPACK=1` | air-gapped installs |
| **Bring your own** | drop the file in the data dir; detected at startup | custom or rebuilt packs |

The pack sits **next to** `tanulom.db`, not inside it. Upgrading is a file swap plus a schema
check — user data is never at risk, which is the whole reason for the split.

---

## 6. Quality gates

These block a pack release.

| Gate | Threshold | Why |
|---|---|---|
| Lemma accuracy vs held-out UD sample | ≥ 95% | below this, `n+1` selection is unreliable |
| Feature-set exact match vs UD | ≥ 90% | below this, the mastery heatmap is noise |
| `wordform` coverage of a Tatoeba holdout | ≥ 98% of tokens | measures whether runtime lookup actually resolves |
| Every `grammar_topic` has non-empty `explain_md` in **both** locales | 100% | [R4](../.claude/research/04-explicit-instruction.md) — a topic without an explanation cannot be taught explicitly |
| Every `morph_feature` has labels in both locales | 100% | [ADR-004](../.claude/DECISIONS.md#adr-004) |
| Every `pair_audio` row has a resolvable licence | 100% | Tatoeba audio licensing is per-clip |
| ≥ 5 distinct talkers per HVPT contrast | 100% of contrasts | [R7](../.claude/research/07-hvpt.md) — talker variability is the active ingredient |
| `manifest.json` checksums match the emitted file | exact | reproducibility |

---

## 7. Versioning

`langpack-hu-v{N}.sqlite`, with `pack_meta` carrying `version`, `schema_version`, `built_at` and
every source checksum.

- **Patch** (v1 → v1.1): data corrections, more sentences. No schema change. Hot-swappable.
- **Minor** (v1 → v2): new tables or columns, additive. The binary tolerates older packs for one
  minor version and warns.
- **Major**: incompatible schema. The binary refuses to start and names the required pack
  version.

`morph_feature.id` values (`Case=Ine`) are **stable identifiers** — a learner's `morph_state`
references them across pack upgrades. Renaming one is a breaking change and requires a migration
that remaps user state.
