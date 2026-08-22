-- Language pack schema. Read-only at runtime; ATTACHed beside the learner
-- database so a pack upgrade is a file swap that cannot touch learner data.
-- See ../../docs/LANGPACK.md.

PRAGMA journal_mode = OFF;
PRAGMA synchronous = OFF;

CREATE TABLE pack_meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- ─── lexicon ──────────────────────────────────────────────────────────────
CREATE TABLE lemma (
    id        INTEGER PRIMARY KEY,
    lemma     TEXT NOT NULL,
    upos      TEXT NOT NULL,
    freq_rank INTEGER,          -- NULL = not in the frequency list
    freq_pm   REAL,             -- occurrences per million
    cefr      TEXT,             -- estimated from frequency band
    ipa       TEXT
);
CREATE INDEX idx_lemma_lemma ON lemma(lemma);
CREATE INDEX idx_lemma_rank  ON lemma(freq_rank);
CREATE UNIQUE INDEX idx_lemma_key ON lemma(lemma, upos);

CREATE TABLE lemma_gloss (
    lemma_id  INTEGER NOT NULL REFERENCES lemma(id),
    lang      TEXT NOT NULL CHECK (lang IN ('ru','en')),
    sense_idx INTEGER NOT NULL,
    gloss     TEXT NOT NULL,
    PRIMARY KEY (lemma_id, lang, sense_idx)
);

-- The hot table. `source` records provenance and `confidence` records how much
-- a generated form is trusted -- both matter to the answer diff, which must be
-- able to say "you wrote X; the attested form is Y".
CREATE TABLE wordform (
    form       TEXT NOT NULL,
    lemma_id   INTEGER NOT NULL REFERENCES lemma(id),
    upos       TEXT NOT NULL,
    feats      TEXT NOT NULL DEFAULT '',
    source     TEXT NOT NULL CHECK (source IN ('wiktionary','generated','corpus')),
    confidence TEXT NOT NULL CHECK (confidence IN ('high','medium','low'))
);
CREATE INDEX idx_wordform_form  ON wordform(form);
CREATE INDEX idx_wordform_lemma ON wordform(lemma_id);

-- Display-only: high-confidence forms, one per slot.
CREATE TABLE paradigm (
    lemma_id INTEGER NOT NULL REFERENCES lemma(id),
    slot     TEXT NOT NULL,
    form     TEXT NOT NULL,
    PRIMARY KEY (lemma_id, slot)
);

-- ─── the second state space ───────────────────────────────────────────────
CREATE TABLE morph_feature (
    id          TEXT PRIMARY KEY,      -- UD "Case=Ine"; stable across pack versions
    key         TEXT NOT NULL,
    value       TEXT NOT NULL,
    label_ru    TEXT NOT NULL,
    label_en    TEXT NOT NULL,
    allomorphs  TEXT NOT NULL,         -- JSON {harmony_class: suffix}
    explain_ru  TEXT NOT NULL,
    explain_en  TEXT NOT NULL,
    teach_order INTEGER NOT NULL,
    prereq_ids  TEXT NOT NULL DEFAULT '',
    grid        TEXT                   -- "interior/where" etc., for the 3x3 UI
);
CREATE INDEX idx_feature_order ON morph_feature(teach_order);

-- ─── input material ───────────────────────────────────────────────────────
CREATE TABLE sentence (
    id         INTEGER PRIMARY KEY,
    hu         TEXT NOT NULL,
    source     TEXT NOT NULL,
    license    TEXT NOT NULL,
    len_tokens INTEGER NOT NULL,
    difficulty REAL
);
CREATE INDEX idx_sentence_len ON sentence(len_tokens);

CREATE TABLE sentence_trans (
    sentence_id INTEGER NOT NULL REFERENCES sentence(id),
    lang        TEXT NOT NULL,
    text        TEXT NOT NULL,
    PRIMARY KEY (sentence_id, lang)
);

-- Pre-analysed so n+1 scoring is a SQL join rather than a parse.
CREATE TABLE sentence_token (
    sentence_id INTEGER NOT NULL REFERENCES sentence(id),
    idx         INTEGER NOT NULL,
    form        TEXT NOT NULL,
    lemma_id    INTEGER REFERENCES lemma(id),
    feats       TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (sentence_id, idx)
);
CREATE INDEX idx_stoken_lemma ON sentence_token(lemma_id);

-- ─── phonetics ────────────────────────────────────────────────────────────
CREATE TABLE minimal_pair (
    id         INTEGER PRIMARY KEY,
    a          TEXT NOT NULL,
    b          TEXT NOT NULL,
    contrast   TEXT NOT NULL,
    priority   INTEGER NOT NULL,
    gloss_a_ru TEXT NOT NULL, gloss_b_ru TEXT NOT NULL,
    gloss_a_en TEXT NOT NULL, gloss_b_en TEXT NOT NULL,
    context_a  TEXT NOT NULL DEFAULT '',
    context_b  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_pair_contrast ON minimal_pair(contrast);

-- Audio is added by the rendering stage. `talker_id` is recorded on every clip
-- because talker variability is the active ingredient of HVPT, and the Ear view
-- reports how many distinct talkers a session actually used.
CREATE TABLE pair_audio (
    pair_id   INTEGER NOT NULL REFERENCES minimal_pair(id),
    member    TEXT NOT NULL CHECK (member IN ('a','b')),
    talker_id TEXT NOT NULL,
    talker_kind TEXT NOT NULL CHECK (talker_kind IN ('piper','human')),
    rate      REAL NOT NULL DEFAULT 1.0,
    path      TEXT NOT NULL,
    license   TEXT NOT NULL,
    PRIMARY KEY (pair_id, member, talker_id, rate)
);

-- ─── curriculum ───────────────────────────────────────────────────────────
CREATE TABLE grammar_topic (
    id            INTEGER PRIMARY KEY,
    code          TEXT NOT NULL UNIQUE,
    level         TEXT NOT NULL,
    title_ru      TEXT NOT NULL, title_en      TEXT NOT NULL,
    explain_md_ru TEXT NOT NULL, explain_md_en TEXT NOT NULL,
    prereq_codes  TEXT NOT NULL DEFAULT '',
    feature_ids   TEXT NOT NULL DEFAULT '',
    teach_order   INTEGER NOT NULL
);
CREATE INDEX idx_topic_order ON grammar_topic(teach_order);

-- ─── the attestation guardrail ────────────────────────────────────────────
CREATE TABLE ngram (
    gram  TEXT NOT NULL,
    n     INTEGER NOT NULL,
    count INTEGER NOT NULL,
    PRIMARY KEY (gram, n)
);
