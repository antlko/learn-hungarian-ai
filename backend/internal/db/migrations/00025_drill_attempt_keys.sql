-- +goose Up

-- Repeated drills must vary: the generator now asks the attempt log which items,
-- sentences and lemmas the learner met recently (internal/drill/recent.go).
-- That needs two new columns, and two new exercise kinds (`person`, `qword`)
-- need the kind CHECK gone — it listed the kinds by name, so every new kind
-- would have needed another table rebuild. Kind validity is checked in Go
-- (drill.Kind.Valid at AppendAttempt) instead.
--
-- SQLite cannot drop a CHECK in place, so this is the documented table rebuild.
-- Nothing references drill_attempt (no foreign key points at it, no view or
-- trigger reads it), and goose runs the file in one transaction, so a failure
-- leaves the old table untouched. Every existing row is copied with its id.
--
--   item_key    = kind|lemmaID|headline|sentenceID, '' on rows written before
--                 this migration (the reader falls back to `prompt` for those).
--   corpus_sentence = pack sentence the frame came from, 0 when there was none.
--                 Deliberately NOT called sentence_id: that name is how
--                 lang.IDRefs finds columns a pack install must keep
--                 compatible, and a stale anti-repeat hint after a pack swap is
--                 harmless — refusing the install over it would not be.
CREATE TABLE drill_attempt_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    topic_code  TEXT    NOT NULL,
    kind        TEXT    NOT NULL,
    phase       TEXT    NOT NULL CHECK (phase IN ('blocked','interleaved','mastered')),
    prompt      TEXT    NOT NULL,
    expected    TEXT    NOT NULL,
    answer      TEXT    NOT NULL DEFAULT '',
    correct     INTEGER NOT NULL CHECK (correct IN (0,1)),
    duration_ms INTEGER NOT NULL DEFAULT 0,
    feature_ids TEXT    NOT NULL DEFAULT '',
    error_class TEXT    NOT NULL DEFAULT '',
    origin      TEXT    NOT NULL DEFAULT 'drill'
                CHECK (origin IN ('drill','placement','from_error')),
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    item_key    TEXT    NOT NULL DEFAULT '',
    corpus_sentence INTEGER NOT NULL DEFAULT 0
);

INSERT INTO drill_attempt_new (id, user_id, topic_code, kind, phase, prompt, expected,
                               answer, correct, duration_ms, feature_ids, error_class,
                               origin, created_at)
SELECT id, user_id, topic_code, kind, phase, prompt, expected, answer, correct,
       duration_ms, feature_ids, error_class, origin, created_at
  FROM drill_attempt;

DROP TABLE drill_attempt;
ALTER TABLE drill_attempt_new RENAME TO drill_attempt;

CREATE INDEX idx_drill_attempt_user  ON drill_attempt(user_id, created_at);
CREATE INDEX idx_drill_attempt_topic ON drill_attempt(user_id, topic_code, created_at);
CREATE INDEX idx_drill_attempt_recent ON drill_attempt(user_id, topic_code, id);

-- +goose Down

-- Not reversible: the old table's kind CHECK would reject the new kinds already
-- logged. Left in place; older binaries never run against a newer database.
SELECT 1;
