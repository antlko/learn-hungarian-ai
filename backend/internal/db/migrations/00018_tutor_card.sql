-- +goose Up
-- The tutor reads the card (ADR-034). A thread can be the day's REVIEW chat
-- (`kind = 'review'`): one per learner per local day, found or created when
-- the review card first asks, titled by the client from its date. A question
-- asked about a card records which card, in which phase, from which template
-- — on the tutor's side only. Nothing on the review path reads these columns:
-- a question asked before the answer counts as a hint through `usedHint`, the
-- same channel «Подсказка» uses, and ADR-027's line — no code path from
-- tutor_message to any state space — stays where it is.
ALTER TABLE tutor_thread ADD COLUMN kind TEXT NOT NULL DEFAULT '';
ALTER TABLE tutor_message ADD COLUMN card_id    INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tutor_message ADD COLUMN card_phase TEXT    NOT NULL DEFAULT '';
ALTER TABLE tutor_message ADD COLUMN template   TEXT    NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE tutor_message DROP COLUMN template;
ALTER TABLE tutor_message DROP COLUMN card_phase;
ALTER TABLE tutor_message DROP COLUMN card_id;
ALTER TABLE tutor_thread DROP COLUMN kind;
