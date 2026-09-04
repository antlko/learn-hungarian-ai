-- +goose Up
-- The trash: a word the learner set aside.
--
-- Not every card that reaches the queue is worth answering. A sentence the
-- miner cut badly, a word whose gloss is useless, a construction three months
-- above the learner's head — with no way out, the only options were to fail it
-- forever or to let it rot at the front of every session. «Слишком сложно» is
-- a legitimate verdict on a card and the app has to accept it.
--
-- A NOTE-level fact, not a card-level one. The learner is setting aside a WORD;
-- burying only the direction on screen would hand them the same word back ten
-- minutes later through a different door.
--
-- Deliberately NOT `card.suspended`. That column is a card-level mechanism
-- nothing sets today, and overloading it would make "restore" ambiguous the
-- first time anything else needs to suspend a single direction. The timestamp
-- also gives the trash screen an order to list in, which a flag cannot.
--
-- NULL means "in the deck". Everything that builds a queue, counts a pile or
-- reports a number filters on it — see srs.Scope.filter, which carries the
-- clause for the three normal scopes so that no future query can forget it.
--
-- The review_log is untouched: a trashed word's past answers are past work, and
-- undo is delete-and-replay over that log (ADR-006). Trashing is not deleting.
ALTER TABLE note ADD COLUMN trashed_at TEXT;

-- Partial: the trash is a handful of rows in a deck of thousands, and every
-- other query wants `trashed_at IS NULL`, which the index below cannot serve
-- anyway — that one rides the existing idx_note_user.
CREATE INDEX idx_note_trashed ON note(user_id, trashed_at) WHERE trashed_at IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_note_trashed;
ALTER TABLE note DROP COLUMN trashed_at;
