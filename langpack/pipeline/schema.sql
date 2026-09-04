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
-- The tagger's lemma_id and the dictionary's headword disagree often enough
-- that the course also has to find a sentence by SURFACE FORM: `nagyon` is
-- lemmatised to `nagy`, so the lemma join returns nothing for 820 sentences
-- that plainly contain the word. BINARY-collated like every other index here —
-- callers enumerate the casings (morph.CaseVariants) rather than folding.
CREATE INDEX idx_stoken_form ON sentence_token(form);

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

-- ─── the guided course ────────────────────────────────────────────────────
--
-- The grammar DAG says what order to learn RULES in. It says nothing about
-- which WORDS, and a learner who is told "mine your own sentences" on day one
-- has been handed a research project instead of a course. So the pack also
-- carries an explicit six-month syllabus: ordered units, each with the concrete
-- situation it buys you, the grammar it leans on, and the vocabulary to own.
--
-- After the last unit the app stops prescribing and points at real sources —
-- that hand-off is the point of the course, not an afterthought. See
-- docs/PEDAGOGY.md and .claude/research/11-extensive-reading.md.
CREATE TABLE course_unit (
    id            INTEGER PRIMARY KEY,
    code          TEXT NOT NULL UNIQUE,
    month         INTEGER NOT NULL,
    ord           INTEGER NOT NULL,
    title_ru      TEXT NOT NULL, title_en      TEXT NOT NULL,
    -- What the learner can DO afterwards, in the first person. A unit whose
    -- goal cannot be written as a real errand is a unit about grammar, not
    -- about Hungary, and does not belong in this table.
    goal_ru       TEXT NOT NULL, goal_en       TEXT NOT NULL,
    -- The situation this unit is for: "at the doctor", "at the ügyfélszolgálat".
    situation_ru  TEXT NOT NULL, situation_en  TEXT NOT NULL,
    -- grammar_topic codes this unit assumes or introduces, comma separated.
    topic_codes   TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_course_unit_ord ON course_unit(ord);

CREATE TABLE course_word (
    unit_code TEXT NOT NULL,
    lemma     TEXT NOT NULL,
    upos      TEXT NOT NULL DEFAULT '',
    gloss_ru  TEXT NOT NULL,
    gloss_en  TEXT NOT NULL,
    -- Anything that will trip the learner: irregular stem, formal register,
    -- a false friend. Empty for the ordinary majority.
    note      TEXT NOT NULL DEFAULT '',
    ord       INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (unit_code, lemma)
);

-- Where to read and listen once the course hands over. Curated, licensed,
-- and levelled — "just read the news" is not a plan for an A2 learner.
CREATE TABLE resource (
    id          INTEGER PRIMARY KEY,
    kind        TEXT NOT NULL CHECK (kind IN ('news','reading','listening','video','reference','practice')),
    title       TEXT NOT NULL,
    url         TEXT NOT NULL,
    -- The level this becomes usable at, not the level it is written at.
    level       TEXT NOT NULL,
    blurb_ru    TEXT NOT NULL, blurb_en TEXT NOT NULL,
    free        INTEGER NOT NULL DEFAULT 1 CHECK (free IN (0,1)),
    ord         INTEGER NOT NULL DEFAULT 0
);

-- ─── the attestation guardrail ────────────────────────────────────────────
CREATE TABLE ngram (
    gram  TEXT NOT NULL,
    n     INTEGER NOT NULL,
    count INTEGER NOT NULL,
    PRIMARY KEY (gram, n)
);
