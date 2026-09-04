-- +goose Up
-- go-fsrs v4 Card.RemainingSteps was never persisted. The scheduler's
-- learningState therefore always saw 0 remaining steps, so Again/Hard on a
-- Learning or Relearning card graduated it a day or more out instead of
-- re-showing it in minutes; only the Good->Good path happened to coincide,
-- which is why the bug survived the replay property test (replay ran through
-- the same lossy mapping and agreed with the wrong answer). Storing the field
-- completes the FSRS state so that replaying the log reproduces it bit for bit
-- again. See ADR-020.
--
-- lemma_state also carries FSRS columns but is deliberately skipped: only its
-- `status` column is ever written, the scheduler never advances it, and adding
-- a column nothing reads would imply otherwise.
ALTER TABLE card                ADD COLUMN remaining_steps INTEGER NOT NULL DEFAULT 0;
ALTER TABLE morph_state         ADD COLUMN remaining_steps INTEGER NOT NULL DEFAULT 0;
ALTER TABLE hvpt_contrast_state ADD COLUMN remaining_steps INTEGER NOT NULL DEFAULT 0;

-- App-level one-time maintenance markers (e.g. the steps reconciliation that
-- replays every log with the fixed scheduler). The work itself cannot run in a
-- migration: it needs config.yaml's desiredRetention, and migrations run inside
-- db.Open before any config has been read. Distinct from `settings`, which is
-- per-user; these are facts about the database file.
CREATE TABLE app_meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS app_meta;
ALTER TABLE hvpt_contrast_state DROP COLUMN remaining_steps;
ALTER TABLE morph_state         DROP COLUMN remaining_steps;
ALTER TABLE card                DROP COLUMN remaining_steps;
