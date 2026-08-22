-- +goose Up

-- The reading library.
--
-- Texts are tokenised once on ingest and stored analysed, for two reasons:
-- re-analysing a news article on every page view is wasteful, and coverage over
-- a learner's OWN corpus becomes a meaningful, improving number only if the
-- corpus persists.
CREATE TABLE text (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    title      TEXT    NOT NULL,
    source_url TEXT    NOT NULL DEFAULT '',
    kind       TEXT    NOT NULL DEFAULT 'paste'
               CHECK (kind IN ('paste','url','srt','epub','generated')),
    body       TEXT    NOT NULL,
    -- Denormalised so the library list does not have to re-count every text.
    word_count      INTEGER NOT NULL DEFAULT 0,
    lemma_coverage  REAL    NOT NULL DEFAULT 0,
    element_coverage REAL   NOT NULL DEFAULT 0,
    added_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    read_at    TEXT
);
CREATE INDEX idx_text_user ON text(user_id, added_at DESC);

-- One row per token, pre-analysed. `feats` is the UD feature string of THIS
-- occurrence, which is what n+1 scoring needs: the same lemma in two sentences
-- can carry entirely different unknown features.
CREATE TABLE text_token (
    text_id  INTEGER NOT NULL REFERENCES text(id) ON DELETE CASCADE,
    idx      INTEGER NOT NULL,
    sent_idx INTEGER NOT NULL,
    form     TEXT    NOT NULL,
    start_b  INTEGER NOT NULL,
    end_b    INTEGER NOT NULL,
    is_word  INTEGER NOT NULL DEFAULT 1 CHECK (is_word IN (0,1)),
    lemma_id INTEGER,
    feats    TEXT    NOT NULL DEFAULT '',
    certainty TEXT   NOT NULL DEFAULT 'unknown',
    PRIMARY KEY (text_id, idx)
);
CREATE INDEX idx_text_token_lemma ON text_token(lemma_id);
CREATE INDEX idx_text_token_sent  ON text_token(text_id, sent_idx);

-- +goose Down
DROP TABLE text_token;
DROP TABLE text;
