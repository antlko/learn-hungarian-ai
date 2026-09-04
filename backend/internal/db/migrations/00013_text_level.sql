-- +goose Up
-- Estimated CEFR level of a library text (ADR-023).
--
-- `level_source` says WHO estimated, because the honesty rules require the UI
-- to label it: 'heuristic' (pack frequency bands + grammar topics),
-- 'llm' (level_detail carries the model name), 'manual' (the learner said so —
-- always wins, never overwritten by a recompute). '' means not yet estimated;
-- an estimate the algorithm refuses to make (too little of the text resolves)
-- stays '' with source 'heuristic', which the UI renders as «уровень не
-- определён» rather than inventing a band.
ALTER TABLE text ADD COLUMN level        TEXT NOT NULL DEFAULT '';
ALTER TABLE text ADD COLUMN level_source TEXT NOT NULL DEFAULT '';
ALTER TABLE text ADD COLUMN level_detail TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE text DROP COLUMN level_detail;
ALTER TABLE text DROP COLUMN level_source;
ALTER TABLE text DROP COLUMN level;
