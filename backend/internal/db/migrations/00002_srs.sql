-- +goose Up

-- ─────────────────────────────────────────────────────────────────────────
-- Notes and cards.
--
-- A NOTE is a thing to learn — normally a sentence, because the source brief is
-- explicit that cards must be sentences and never bare words: a lemma on its own
-- teaches nothing about the morphology that carries the meaning.
--
-- A CARD is one DIRECTION of that note, each with its own independent FSRS
-- state. Transfer-appropriate processing means you get back the direction you
-- trained, so recognising `ház` and producing it are genuinely different pieces
-- of knowledge and are scheduled separately. See [R12].
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE note (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    kind        TEXT    NOT NULL CHECK (kind IN ('sentence', 'vocab', 'drill')),
    -- Pointers into the language pack. Not foreign keys: the pack is a separate
    -- database, and a pack upgrade must never orphan a learner's notes.
    lemma_id    INTEGER,
    sentence_id INTEGER,
    -- The target form this note is about, and the span to blank for cloze.
    hu          TEXT    NOT NULL,
    target_form TEXT    NOT NULL DEFAULT '',
    target_start INTEGER NOT NULL DEFAULT -1,
    target_end   INTEGER NOT NULL DEFAULT -1,
    -- Features the target form carries. These are what a cloze answer grades.
    target_feats TEXT   NOT NULL DEFAULT '',
    gloss_ru    TEXT    NOT NULL DEFAULT '',
    gloss_en    TEXT    NOT NULL DEFAULT '',
    notes       TEXT    NOT NULL DEFAULT '',
    source      TEXT    NOT NULL DEFAULT 'manual',
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_note_user ON note(user_id);
CREATE INDEX idx_note_lemma ON note(user_id, lemma_id);

CREATE TABLE card (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    note_id   INTEGER NOT NULL REFERENCES note(id) ON DELETE CASCADE,
    direction TEXT    NOT NULL CHECK (
                  direction IN ('recognize','produce','listen','cloze','spell')),
    -- FSRS-6 state, stored exactly as the library models it so that replaying a
    -- review log reproduces it bit for bit.
    due            TEXT    NOT NULL,
    stability      REAL    NOT NULL DEFAULT 0,
    difficulty     REAL    NOT NULL DEFAULT 0,
    scheduled_days INTEGER NOT NULL DEFAULT 0,
    reps           INTEGER NOT NULL DEFAULT 0,
    lapses         INTEGER NOT NULL DEFAULT 0,
    state          INTEGER NOT NULL DEFAULT 0,   -- 0 New 1 Learning 2 Review 3 Relearning
    last_review    TEXT,
    suspended      INTEGER NOT NULL DEFAULT 0 CHECK (suspended IN (0,1)),
    UNIQUE (note_id, direction)
);
CREATE INDEX idx_card_due ON card(due) WHERE suspended = 0;
CREATE INDEX idx_card_note ON card(note_id);

-- The full FSRS log shape. Two reasons this is complete rather than summarised:
-- replaying it must reproduce stored state exactly (there is a property test for
-- that), and a future per-user optimiser consumes exactly these columns.
CREATE TABLE review_log (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    card_id           INTEGER NOT NULL REFERENCES card(id) ON DELETE CASCADE,
    rating            INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 4),
    state             INTEGER NOT NULL,
    due               TEXT    NOT NULL,
    stability         REAL    NOT NULL,
    difficulty        REAL    NOT NULL,
    elapsed_days      INTEGER NOT NULL,
    last_elapsed_days INTEGER NOT NULL,
    scheduled_days    INTEGER NOT NULL,
    review_time       TEXT    NOT NULL,
    duration_ms       INTEGER NOT NULL DEFAULT 0,
    -- What the learner actually typed, for productive directions. Kept so the
    -- morpheme diff can be re-derived and so error patterns can be mined.
    answer            TEXT    NOT NULL DEFAULT '',
    correct           INTEGER NOT NULL DEFAULT 1 CHECK (correct IN (0,1))
);
CREATE INDEX idx_review_card ON review_log(card_id, review_time);
CREATE INDEX idx_review_time ON review_log(review_time);

-- ─────────────────────────────────────────────────────────────────────────
-- The two knowledge state spaces.
--
-- lemma_state answers "do you know this word".
-- morph_state answers "do you know this grammatical fact".
--
-- Keeping them apart is the entire product thesis: it is what makes real n+1
-- selection possible (`elmentem volna` has zero unknown lemmas and two unknown
-- features) and what makes a per-suffix weakness map expressible at all.
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE lemma_state (
    user_id  INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    lemma_id INTEGER NOT NULL,
    -- `known` and `ignored` are learner declarations that bypass scheduling:
    -- marking a word known in the reader should not create review load.
    status   TEXT    NOT NULL DEFAULT 'learning'
             CHECK (status IN ('new','learning','known','ignored')),
    due            TEXT,
    stability      REAL    NOT NULL DEFAULT 0,
    difficulty     REAL    NOT NULL DEFAULT 0,
    scheduled_days INTEGER NOT NULL DEFAULT 0,
    reps           INTEGER NOT NULL DEFAULT 0,
    lapses         INTEGER NOT NULL DEFAULT 0,
    state          INTEGER NOT NULL DEFAULT 0,
    last_review    TEXT,
    first_seen     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (user_id, lemma_id)
);
CREATE INDEX idx_lemma_state_status ON lemma_state(user_id, status);

CREATE TABLE morph_state (
    user_id    INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    -- A UD feature id such as 'Case=Ine'. Stable across pack versions by
    -- contract; renaming one is a breaking change needing a state remap.
    feature_id TEXT    NOT NULL,
    due            TEXT,
    stability      REAL    NOT NULL DEFAULT 0,
    difficulty     REAL    NOT NULL DEFAULT 0,
    scheduled_days INTEGER NOT NULL DEFAULT 0,
    reps           INTEGER NOT NULL DEFAULT 0,
    lapses         INTEGER NOT NULL DEFAULT 0,
    state          INTEGER NOT NULL DEFAULT 0,
    last_review    TEXT,
    -- Rolling accuracy for the blocked -> interleaved gate, and the latency
    -- estimate the vowel-harmony mastery rule needs.
    --
    -- Accuracy is an exponentially weighted average rather than a correct/total
    -- pair: a pair of integers cannot be aged without truncating (a 1/20 ratio
    -- decays straight to 0/19 and can never recover), and keeping the full
    -- outcome history just to compute a windowed mean is not worth a table.
    -- `recent_total` counts observations so the gate can refuse to act on an
    -- estimate built from three answers.
    recent_accuracy REAL    NOT NULL DEFAULT 0,
    recent_total    INTEGER NOT NULL DEFAULT 0,
    median_ms       INTEGER NOT NULL DEFAULT 0,
    first_seen     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (user_id, feature_id)
);

-- One row per feature graded by one answer. A single cloze review emits one card
-- review plus N of these — only for the features the item actually required,
-- which is what keeps the mastery heatmap meaningful rather than noise.
CREATE TABLE feature_review (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    feature_id  TEXT    NOT NULL,
    rating      INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 4),
    correct     INTEGER NOT NULL CHECK (correct IN (0,1)),
    duration_ms INTEGER NOT NULL DEFAULT 0,
    review_time TEXT    NOT NULL,
    card_id     INTEGER REFERENCES card(id) ON DELETE SET NULL,
    origin      TEXT    NOT NULL DEFAULT 'review'
);
CREATE INDEX idx_feature_review ON feature_review(user_id, feature_id, review_time);

-- Learner corrections to a bad pack analysis. The pack is read-only, so an
-- override lives here and is applied as an overlay at lookup time.
CREATE TABLE morph_correction (
    user_id    INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    form       TEXT    NOT NULL,
    lemma      TEXT    NOT NULL,
    feats      TEXT    NOT NULL DEFAULT '',
    note       TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (user_id, form)
);

-- +goose Down
DROP TABLE morph_correction;
DROP TABLE feature_review;
DROP TABLE morph_state;
DROP TABLE lemma_state;
DROP TABLE review_log;
DROP TABLE card;
DROP TABLE note;
