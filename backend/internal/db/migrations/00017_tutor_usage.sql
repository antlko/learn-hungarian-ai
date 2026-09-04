-- +goose Up
-- The context meter (ADR-031). What the server reported spending on an
-- answer, in its own tokens, and the window the model was running with: an
-- Ollama server says (GET /api/ps), the OpenAI stream says (usage), and a
-- prompt longer than the window is cut from the HEAD without a word — so the
-- figures are kept with the answer and printed under the composer. Zero means
-- "not reported", and the UI says that instead of printing 0.
ALTER TABLE tutor_message ADD COLUMN prompt_tokens  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tutor_message ADD COLUMN answer_tokens  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tutor_message ADD COLUMN context_tokens INTEGER NOT NULL DEFAULT 0;
-- Why an interrupted answer stopped: budget, timeout, stop, connection — so
-- the label under it says what happened instead of guessing at two causes.
-- Blank on a complete answer and on rows from before this column.
ALTER TABLE tutor_message ADD COLUMN cut_reason TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE tutor_message DROP COLUMN cut_reason;
ALTER TABLE tutor_message DROP COLUMN context_tokens;
ALTER TABLE tutor_message DROP COLUMN answer_tokens;
ALTER TABLE tutor_message DROP COLUMN prompt_tokens;
