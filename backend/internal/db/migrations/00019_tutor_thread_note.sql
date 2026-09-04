-- +goose Up
-- A review chat is per WORD, not per day (ADR-034, amended 2026-09-04).
--
-- One chat a day sounded like continuity and read as amnesia's opposite: the
-- panel opened on the previous word's conversation, so a learner looking at
-- `Örülök` was shown an answer about `jöttél` — and every question after it
-- carried that answer into the model's window. The chat is now found by the
-- note the card belongs to, within the local day: the same word ten minutes
-- later (its learning step) continues the conversation, the next word opens
-- an empty one, and the day's questions are still listed together on the
-- AI-teacher page.
--
-- `title` on a review thread holds the WORD, written when the thread is
-- created — the one thing that tells «Повторение · jöttél» from
-- «Повторение · Örülök» in the chat list. Threads created before this
-- migration have note_id = 0 and no title; they are still listed, named by
-- their date.
--
-- Nothing here reaches a note's cards, its FSRS state or either state space:
-- the column is a pointer the tutor reads, and ADR-027's line — no code path
-- from tutor_message to any state space — is where it was.
ALTER TABLE tutor_thread ADD COLUMN note_id INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_tutor_thread_note ON tutor_thread(user_id, note_id, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_tutor_thread_note;
ALTER TABLE tutor_thread DROP COLUMN note_id;
