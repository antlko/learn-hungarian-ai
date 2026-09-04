-- +goose Up
-- The tutor becomes a chat (ADR-029): a thread is a conversation with a
-- title, an optional pinned topic, and its own life — new, listed, deleted —
-- instead of one fixed thread per curriculum topic.
--
-- `tutor_message.topic_code` changes meaning with this: on an ASSISTANT turn it
-- is the lesson that answer was grounded in — pinned, or inferred from the
-- question — and the panel prints it («по уроку: …»). Blank means the answer
-- was built from the reference corpus alone, and the panel says that too.
-- On a user turn it is blank. Existing rows were all grounded in their
-- thread's topic, so they keep it.
--
-- Nothing here reaches a note, a card, a gloss or either state space; that
-- absence is still the design.
CREATE TABLE tutor_thread (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    -- The first question, cut short; blank until it is asked. A pinned
    -- thread with no title is shown under its topic's name.
    title      TEXT    NOT NULL DEFAULT '',
    -- The topic every question in this thread is grounded in; blank means
    -- the app infers one per question.
    topic_code TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_tutor_thread_user ON tutor_thread(user_id, updated_at);

-- SQLite refuses a REFERENCES clause on an added column with a non-NULL
-- default; the store deletes a thread's messages itself.
ALTER TABLE tutor_message ADD COLUMN thread_id INTEGER NOT NULL DEFAULT 0;

-- One pinned thread per existing (learner, topic) conversation, so nothing
-- anyone asked is lost.
INSERT INTO tutor_thread (user_id, topic_code, created_at, updated_at)
SELECT user_id, topic_code, MIN(created_at), MAX(created_at)
  FROM tutor_message
 GROUP BY user_id, topic_code;

UPDATE tutor_message
   SET thread_id = (SELECT t.id FROM tutor_thread t
                     WHERE t.user_id = tutor_message.user_id
                       AND t.topic_code = tutor_message.topic_code);

DROP INDEX IF EXISTS idx_tutor_message_thread;
CREATE INDEX idx_tutor_message_thread_id ON tutor_message(thread_id, id);

-- +goose Down
DROP INDEX IF EXISTS idx_tutor_message_thread_id;
ALTER TABLE tutor_message DROP COLUMN thread_id;
CREATE INDEX idx_tutor_message_thread ON tutor_message(user_id, topic_code, id);
DROP INDEX IF EXISTS idx_tutor_thread_user;
DROP TABLE IF EXISTS tutor_thread;
