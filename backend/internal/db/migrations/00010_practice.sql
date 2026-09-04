-- +goose Up
-- Free practice: retrieval without scheduling.
--
-- A separate table, and deliberately NOT review_log. Undo is delete-and-replay
-- over that log and it must reproduce stored FSRS state bit for bit (see
-- internal/srs/replay.go), so a row that never fed the scheduler cannot live in
-- it without breaking the property the whole undo mechanism rests on.
--
-- Nothing here is scheduling. Practice never moves a card's due date, never
-- touches stability, and never advances a lemma toward `known`: closing a word
-- is what the review queue is for. This records that the work happened and how
-- it went, which is a competence signal with something measured behind it.
-- See ADR-019.
CREATE TABLE practice_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    card_id     INTEGER NOT NULL REFERENCES card(id) ON DELETE CASCADE,
    -- The direction is copied rather than joined: a card deleted later should
    -- not silently rewrite what the learner practised.
    direction   TEXT    NOT NULL,
    correct     INTEGER NOT NULL CHECK (correct IN (0,1)),
    duration_ms INTEGER NOT NULL DEFAULT 0,
    practised_at TEXT   NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX idx_practice_log_user ON practice_log(user_id, practised_at);


-- +goose Down
DROP INDEX IF EXISTS idx_practice_log_user;
DROP TABLE IF EXISTS practice_log;
