-- +goose Up
-- The grammar tutor's threads: one conversation per learner per topic.
--
-- Stored so a thread survives a reload and a learner can re-read an
-- explanation, not so anything here feeds the app: nothing in this table
-- reaches a note, a card, a gloss or either state space. The model that
-- wrote each answer is recorded with it, because the label is the entire
-- defence for machine output (ADR-014, ADR-027), and `unattested` keeps the
-- Hungarian words the pack could not find so the UI can keep flagging them.
CREATE TABLE tutor_message (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id        INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    topic_code     TEXT    NOT NULL,
    role           TEXT    NOT NULL CHECK (role IN ('user', 'assistant')),
    content        TEXT    NOT NULL,
    -- The language the answer was asked for, ru or en.
    lang           TEXT    NOT NULL DEFAULT '',
    model          TEXT    NOT NULL DEFAULT '',
    prompt_version INTEGER NOT NULL DEFAULT 0,
    -- JSON array of backticked Hungarian words the dictionary does not carry.
    unattested     TEXT    NOT NULL DEFAULT '[]',
    -- 1 when the stream was cut off (timeout, model error) and the text is
    -- partial. Kept rather than dropped: half an explanation is still one.
    interrupted    INTEGER NOT NULL DEFAULT 0 CHECK (interrupted IN (0,1)),
    created_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_tutor_message_thread ON tutor_message(user_id, topic_code, id);

-- +goose Down
DROP INDEX IF EXISTS idx_tutor_message_thread;
DROP TABLE IF EXISTS tutor_message;
