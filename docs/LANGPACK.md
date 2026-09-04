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
-- `cefr` is estimated from frequency bands (900/2000/4500/8000/16000,
-- pipeline/frequency.py) and is the ONE band table: the reader's text-level
-- estimate reads it (ADR-023). internal/drill/generate.go's `bandFor` uses
-- different cutoffs on purpose — it budgets drill VOCABULARY per curriculum
-- level, a different dial — so neither should be "fixed" to match the other,
-- and nothing may fork a third table.
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

### Russian glosses come from a second edition, and from the course

The English Wiktionary edition gives EN glosses only, so RU glosses are a
separate stage: `pipeline/wiktionary_ru.py` reads the **Russian** edition through
the same wiktextract toolchain.

The two editions are not comparable in size, and pretending otherwise would set
the wrong expectation. English carries ~45 900 glossed Hungarian senses; Russian
carries ~12 300, and its shape is lopsided — roughly 10 000 nouns against under
600 verbs. Matching is on `(lemma, upos)` and falls back to lemma alone, because
ruwiktionary tags `piros` an adjective where a frequency-derived lemma may be a
noun, and refusing that pair would drop a colour word to defend a tagging
convention.

**The course's glosses outrank the dictionary's.** All 853 course words carry a
hand-written `gloss_ru` chosen for the unit that teaches them, and that gloss is
promoted to `sense_idx 0`; the imported senses shift down behind it rather than
being discarded. `kér` shows *просить* — the sense the unit means — with
*требовать, заказывать* still reachable in the word's detail view. Showing a
beginner the dictionary's first sense instead would be a small lie told 853
times.

| | full pack | starter pack |
|---|---|---|
| lemmas with an RU gloss | 8 620 | 2 290 of 5 132 |
| top-1000 frequency band | 604 of 1 270 | — |
| course words | 853 of 853 | 853 of 853 |

Where Russian is absent the pack still has English, `pack_meta.gloss_langs` reads
`["en","ru"]`, and the UI shows its documented fallback badge rather than
silently presenting English as Russian
([ADR-004](../.claude/DECISIONS.md#adr-004)). `starter.py` recomputes
`gloss_langs` from the rows it actually copied, so the starter cannot inherit a
claim the full pack could back and it cannot.

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
| score | `sentences.py`, `minimal_pairs.py`, `curriculum.py`, `lessons.py`, `ngram.py` | sentence difficulty; CEFR estimate per lemma; contrast inventory + audio rendering; topic DAG + lesson text; n-gram counts |
| emit | `emit.py` | write SQLite, create indexes, `VACUUM`, checksum, `manifest.json` |

### Editing a lesson without a full rebuild

`curriculum.py` owns the teaching DAG; **`lessons.py` owns the prose**, keyed by topic code, in
both locales. They were one file, and that is why every lesson used to be a single paragraph of
50–450 characters: nobody puts a page of prose in the middle of a graph definition. A lesson
now states the rule, shows the paradigm as a paradigm, works through examples and names the
mistake a learner actually makes — which is [R4] applied rather than cited.

Four tables are computed from Python literals alone and need nothing downloaded —
`morph_feature`, `grammar_topic`, `minimal_pair`, and `course_unit`/`course_word`. Rewriting
them in an existing pack takes seconds:

```bash
make refresh-curriculum          # python -m pipeline.refresh --pack ...
make starter                     # then re-embed, and COMMIT the result
```

`pipeline/refresh.py` validates everything **before** it opens the file for writing — including
every minimal-pair member against the pack's own `wordform` table ([ADR-033](../.claude/DECISIONS.md#adr-033))
— and swaps all five tables in one transaction: a pack left half-rewritten by a content error
is worse than one carrying the old text. The ear trainer's `pair_audio` rows follow their pair
to its new id and the rows of pairs that are gone are dropped, so a refresh never plays one
pair's clips under another. **Stop the server first** — the app treats a pack as read-only, and
this is a build step writing to one. After a refresh that changed the pairs, `make audio`
renders the missing clips (`rm -rf data/media/hvpt` first to drop the old id-named files).

**`make starter` is not optional after either command.** The binary embeds its own pack, so
skipping it ships last month's lessons ([ADR-010](../.claude/DECISIONS.md#adr-010)).

### Conversational sentences, and adding them without a rebuild

Tatoeba is 60,000 sentences of careful *written* Hungarian, and only 3,746 of them carry a
Russian translation. For an app whose stated goal is conversation that is a thin corpus: the
`n+1` rail repeats itself, a course word with no attested sentence gets no card, and the review
screen shows sentences it cannot translate.

`pipeline/subtitles.py` reads the OPUS OpenSubtitles parallel corpora (`hu-ru`, 690 MB;
`en-hu`, 1.3 GB — 19.1M aligned line pairs each) and keeps a fraction of a percent of them.

**The alignment cannot be trusted pair by pair, and that shapes the whole design.**
OpenSubtitles is aligned automatically and it fails predictably at the head of every film, where
one side runs translator credits and opening-song lyrics against the other side's dialogue.
Sampling the first lines gives nonsense; sampling deep in the corpus gives 17 correct
translations out of 18. Structural guards (numerals must survive, a question stays a question,
plausible length ratio, no credit lines) remove the obvious cases and are not enough.

So selection is by **recurrence and majority vote**:

- a sentence must occur at least `MIN_OCCURRENCES` times before it counts as a phrase rather
  than a line from one film — which also means the sort key *is* "how often people say this";
- its translation is the one that wins a vote across all its occurrences, by at least
  `MIN_AGREEMENT`. A true translation recurs; forty independent misalignments do not agree with
  each other. Below the threshold the sentence is kept and the **translation is dropped** —
  real Hungarian with no translation is useful, a wrong translation is not.

Measured on `hu-ru`: 10.0M usable pairs → 8.85M distinct sentences → 132,079 recurring →
26,781 with a translation that carried its vote. The top of that list is
`Mit csinálsz itt?`, `Hogy érzed magad?`, `Mennyi az idő?`, `Hány éves vagy?`.

```bash
make subtitles                   # ~40 min, no full rebuild
```

The sentence stage never calls HuSpaCy — `build.py` links each token through the pack's own
`wordform` index — so adding sentences to a pack that already has a lexicon is minutes of I/O
rather than the hours a full `make langpack` costs. `pipeline/add_subtitles.py` is idempotent:
it owns every `source='opensubtitles'` row and replaces them wholesale, never touching Tatoeba's,
and it rebuilds the n-gram attestation index because the counts in it are what make it a
guardrail.

**These sentences never reach the starter pack.** See
[LICENSES.md](../langpack/LICENSES.md#opensubtitles-and-redistribution--the-line-this-project-draws).

### The ear trainer's clips are citation-form, and why

The first render sent Piper the bare word. Its VITS voices are trained on sentences, and an
isolated three-letter word is out of distribution: the inventory came out with a **median clip
of 0.35 s** (minimum 0.16 s) in which the kor/kór vowel-length contrast — the thing HVPT exists
to teach — was physically absent. Learners heard "different words entirely" and scored at
chance, which is exactly what stimuli carrying no signal produce.

The render now produces **citation forms**: the word as a one-word sentence (`kór.`), slowed by
`CITATION_SCALE` (HVPT stimuli in the literature are clear citation speech, not conversational
tempo), with enough trailing silence that the Opus encode cannot clip a final consonant —
öt/őt is decided in the last hundred milliseconds. Measured effect: the long member is now
consistently 15–25% longer than its short partner, at every voice and rate, which is the
natural Hungarian ratio. Re-render an existing inventory with `make audio` after
`--force`-clearing, or delete `data/media/hvpt` first.

### Verb conjugations, generated

Wiktionary's extraction is spotty for verbs: `hoz` arrived with 24 forms, holding `hoztuk`
(definite) but not `hoztunk` (indefinite), so a learner clicking a word on their own course
card was told it is not in the pack. `pipeline/verbs.py` generates the **present and past
indicative, both definitenesses, plus -lak/-lek and the infinitive** for every regular verb —
the cells months one to four of the course actually produce.

The honesty rules it operates under: rows are `source='generated'` and can never displace an
attested form (`trustRank`) or be presented as a drill answer without a caveat
(`Form.Attested()`); the past stem is **extracted from an attested past form** where one exists
(`hoztuk` → `hozt` → `hoztunk`, MEDIUM) and only built by rule at LOW where none does; and the
dozen suppletive verbs (`megy`, `van`, `eszik`…) are refused outright — a generator that emits
`megytunk` has stopped being a safety net. Conditional, imperative and potential are out of
scope on purpose: their assimilations are where a generator starts inventing wrong Hungarian.

`make verbs` backfills an existing pack in seconds (it also folds in corpus-attested forms the
original build dropped — 7,872 on hu-v1); the full build runs the same code. Insert-if-absent
per `(form, lemma_id)`, so re-runs are no-ops.

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

### Defects found by building drills on hu-v1

Phase 5 generates every exercise from this pack, which put far more weight on the wordform
index than reading did. Three defects surfaced. All three are fixed in the pipeline for future
packs; the runtime guards stay regardless, because a shipped pack cannot be retro-fixed.

| Defect | Effect | Pipeline fix | Runtime guard |
|---|---|---|---|
| Wiktionary writes an **empty declension cell as `-`**, and `-` passed `_is_plausible_form` because the hyphen is legal inside compounds (`van-e`). hu-v1 therefore stores `-` as the essive of several thousand nouns. | `-` is attested, high-confidence and the shortest candidate, so it wins any "best form" tie-break outright — it was being served as the correct answer to a drill. | `_is_plausible_form` now requires at least one letter (`wiktionary.py`) | `plausibleForm` in `internal/morph/forms.go` |
| The **`paradigm` table is generator-only** — `inflect.paradigm()` never consults the attested forms sitting in `wordform` beside it. For `ház` it gives the superessive as `házan`; the dictionary attests `házon`. | The reader's paradigm table was teaching a wrong form, and any drill built from it would mark the right answer wrong. | `build.py` overlays attested cells onto the generated paradigm, using the shared `inflect.slot_for` | nothing that must be right reads `paradigm`; `Analyzer.Paradigm` builds from the lookup index and marks generator-only cells as derived |
| **Frequency ranks are assigned per lemma STRING**, not per (lemma, POS), because the OpenSubtitles list is not POS-tagged. The NOUN reading of `nem` ("no") therefore sits at rank 4, and `hisz` NOUN at 50. | Drills were being built around "no", "we", "up" and the letter H. | not fixable without POS-tagged frequency data — documented, not papered over | `loadVocabulary` in `internal/drill/generate.go` requires an unambiguous POS, a lower-case lemma and at least one attested inflected form |

A fourth is a limitation rather than a defect: **Wiktionary lists derived verb forms with no
features at all** (`olvashat` carries an empty `feats`). The potential, the `-lak/-lek` form,
the imperative, the participles, the causative, the converb and the frequentative therefore have
no attested material to build items from, and the app reports those topics as
`drill.unavailable.noMaterial` rather than inventing answers. Thirteen of the twenty-eight
topics produce drills on hu-v1.

---

### The audio render stage

Every pair member is a word the dictionary carries ([ADR-033](../.claude/DECISIONS.md#adr-033)):
`minimal_pairs.unattested` runs in the build after the lexicon and in `refresh` before it
writes, and either refuses the pack naming the form. Clips are named by the **word**
(`hvpt/kor-ef012f_anna_100.ogg` — diacritics folded, a hash keeping `kor` and `kór` apart),
never by the pair id, which `refresh` reassigns; `refresh` moves `pair_audio` rows to their
pair's new id and drops the rows of pairs that are gone.

`pair_audio` is populated by `make audio` (which fetches the voice models first — see below),
or directly by `python -m pipeline.minimal_pairs --pack … --media … --voices-dir …`. It renders
every pair member × three Piper voices (`hu_HU-{anna,berta,imre}-medium`, dataset licence
**CC0** per the model cards) × two rates (1.0, 0.85) into `<media>/hvpt/` — **342 Opus clips in
an Ogg container, 513 KB** for the 32-pair inventory, transcoded with `ffmpeg` where it is
available and left as WAV where it is not. That size is why the whole inventory rides inside the
binary.

Idempotent in both directions: a re-run skips synthesis for a clip already on disk but still
upserts its `pair_audio` row, so rebuilding the pack does not silently leave the audio
unregistered. That was a real failure — a pack rebuild wiped `pair_audio` and the ear trainer
went quiet while 384 perfectly good files sat in `media/hvpt/`.

Rate variants are recorded as rates and **never counted as talkers**: three synthetic voices are
three talkers and the app says so. The shipped default is `hvpt.minTalkers = 3` — every CC0
Hungarian voice that exists — against the five [R7](../.claude/research/07-hvpt.md) evidences.
The shortfall is stated in the product rather than rounded away; see
[PEDAGOGY §2.7](PEDAGOGY.md). A session below the configured floor is refused **with the
numbers**, never silently degraded.

The models themselves are 60 MB each and are **not** in the repository: `make voices` fetches
them from `rhasspy/piper-voices`, and only the small `.json` voice cards beside them are
tracked, so which three voices are expected — and under what licence — is recorded in the repo
even before anyone downloads anything.

### Two editions: the starter is in the binary

The full pack is 252 MB. That is the right artefact to *download* and the wrong one to ship
inside an executable — but an app that boots into "no language pack is installed" is not a
product, it is a build step wearing a UI.

So there are two editions, distinguished by `pack_meta.edition`:

| | starter | full |
|---|---|---|
| where | embedded in the binary (`internal/lang/starter/`, gzipped) | downloaded or built |
| lemmas | 5 132 — a frequency band, plus every course word and every lemma their sentences need | 35 311 |
| wordforms | 502 480 | 1 980 147 |
| sentences | 12 000, every one fully resolvable inside the starter's lemma set | 60 000 |
| lemmas with a Russian gloss | 2 290 | 8 620 |
| course words | 853 (all of them) | 853 |
| curriculum, features, minimal pairs, course, resources, n-grams | complete | complete |
| size | ~14 MB gzipped + 0.5 MB audio | 252 MB |

`langpack/pipeline/starter.py` derives it from the full pack. The selection rule that matters:
a sentence is kept only when **every** word token resolves to a kept lemma, so the reader and
the drill generator never hand the learner a sentence the starter cannot explain — a
half-known sentence would poison the coverage numbers rather than merely be unhelpful.

`internal/lang.EnsurePack` installs it on first boot, and `lang.Detect` now VERIFIES that a
candidate opens rather than trusting the filename: a truncated download used to leave a
zero-byte `langpack-hu-v1.sqlite` that the app reported as present, then died behind a message
telling the user to fetch a pack that appeared to already be there.

## 6. Quality gates

Two kinds of thing were mixed in this table and are now separated, because a
threshold nothing enforces is not a gate — it is a wish that reads like one.

### Enforced: these block a pack release

Each is checked by a test or by the pipeline itself, and each currently passes.

| Gate | Threshold | Why |
|---|---|---|
| Every `grammar_topic` has non-empty `explain_md` in **both** locales | 100% | [R4](../.claude/research/04-explicit-instruction.md) — a topic without an explanation cannot be taught explicitly |
| Every `grammar_topic` lesson is a lesson, not a blurb | ≥ 800 chars, both locales | a paragraph of context above an exercise is not explicit instruction; see `pipeline/lessons.py` |
| Every `morph_feature` has labels in both locales | 100% | [ADR-004](../.claude/DECISIONS.md#adr-004) |
| Every `pair_audio` row has a resolvable licence | 100% | Tatoeba audio licensing is per-clip |
| Every `course_word.lemma` resolves in `lemma` | 100% | a course word the pack cannot look up is a word the app cannot teach — enforced in `pipeline/course.py` `validate()` |
| Every `course_unit.topic_codes` entry is a real `grammar_topic.code` | 100% | the plan screen links units to the grammar DAG |
| No lemma is taught by two units | 100% | a duplicate wastes a slot in a fixed syllabus and makes "words left in this unit" wrong |
| Every `resource.url` resolves | 100% | a dead link is worse than an omitted one at the exact moment the course hands over |
| ≥ 3 distinct talkers per HVPT contrast | 100% of contrasts (measured: 3, on every one) | [R7](../.claude/research/07-hvpt.md) — talker variability is the active ingredient. **Three, not the five R7 evidences**: three is every CC0 Hungarian voice that exists |
| `manifest.json` checksums match the emitted file | exact | reproducibility |
| No `wordform.form` without a letter in it | 100% | Wiktionary's empty-cell marker `-` was imported as a real form and won every shortest-form tie-break |
| Every `paradigm` cell prefers the attested form where one exists | 100% | the table is shown to a learner; a generated cell that contradicts the dictionary teaches the wrong form |

Note the talker row. The **evidence** figure in [R7](../.claude/research/07-hvpt.md) is five
talkers; three is what exists under a free licence for Hungarian. The pack is gated at what it
ships and the shortfall is stated in the product, not rounded up here — see
[PEDAGOGY §2.7](PEDAGOGY.md).

### Measured, not gated: what the analysis is actually worth

These were written as ≥ 95% lemma and ≥ 90% features before the analyser existed, on the
assumption of a contextual tagger. What ships is a dictionary lookup handed one word with no
sentence around it ([ADR-002](../.claude/DECISIONS.md#adr-002)), and these are its real numbers
against **UD Hungarian-Szeged**, 8 873 word tokens the pack never saw
(`backend/internal/morph/golden_ud_test.go`):

| | measured on hu-v1 | regression floor |
|---|---|---|
| coverage — an analysis was produced at all | 80.7% | 75% |
| lemma — gold lemma among the candidates | 76.9% | 72% |
| UPOS | 72.3% | 67% |
| feature set, exact, over the 14 modelled keys | 52.2% | 49% |

**The feature figure fell from 56.2% and the analyser did not change.** The test reads the keys
it scores from the pack, so extending the inventory automatically widens the comparison — and
the inventory gained the verb's `Person`, which the app has always graded (every conjugated form
carries one, so it has a `morph_state` row) but could not name, because it had no row in
`morph_feature`. The progress heatmap printed `Person=1` at the learner. Scoring a feature we
were already grading is the metric becoming more honest, not the parser becoming worse.

**Of the tokens it does analyse, the lemma is right 96% of the time.** Coverage is the binding
constraint, not correctness: proper nouns (60% of 745 unanalysed) and verbs (45% of 927) are
most of the gap — names are not in a dictionary, and the conjugation generator covers fewer
forms than the noun one.

Three notes on method, so the figures can be trusted rather than taken:

- Punctuation is excluded by the **tokeniser's** own `IsWord` verdict, not by a POS whitelist,
  which would be circular. Leaving UD's 1 479 punctuation tokens in would drop every figure by
  fourteen points and measure nothing.
- The unmarked forms are normalised on **both** sides before comparison. UD writes
  `Case=Nom|Number=Sing` on a bare noun and `Mood=Ind|Tense=Pres|Person=3|Number=Sing` on a
  bare verb; this pack treats an absent suffix as an absent feature, because that is what it
  schedules. Both conventions are defensible and the mapping only ever adds a default where the
  analysis is silent — it cannot turn a wrong answer into a right one.
- One convention difference is left **counted as an error** rather than normalised away: on a
  plural possessive UD writes `Number=Plur` where this pack writes
  `Number=Sing|Number[psed]=Plur`. Ours is the more explicit analysis and it still fails the
  comparison. Normalising it would be tuning the metric to the system it measures.

The certainty badges were checked against the same gold standard and do sort something real:
analyses labelled `certain` carry the gold lemma **93.8%** of the time against `uncertain`'s
**84.3%**. `derived` measures higher than both (98.4%, on 182 tokens) and is deliberately not
ranked against them — the suffix fallback only ever runs on words the dictionary missed, which
in running text are overwhelmingly transparent regulars.

The older `wordform` coverage figure — **88.2% token resolution** over the pack's own 60 000
sentences — is kept because it tracks a different thing (does the runtime lookup resolve), but
it cannot answer whether an analysis is correct: it is measured with the pack's own tables
against the pack's own corpus.

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
