-- +goose Up
-- ─────────────────────────────────────────────────────────────────────────
-- The learner's own dictionary.
--
-- The language pack is read-only and it does not have every word. `aludtál`,
-- `Kinyitom` and a long tail behind them resolve to no lemma, and until now the
-- reader could say nothing about them at all: the gloss section was absent, the
-- knowledge buttons were dead, and a card built from one was blank on its back.
--
-- This is the overlay that fixes it, and it is the same shape as
-- `morph_correction`: the pack stays untouched and the learner's own knowledge
-- lives beside it, applied at lookup time.
--
-- `source` is not decoration. A meaning the learner wrote and a meaning a
-- translation model produced are different kinds of thing, and the second stays
-- labelled as machine output for as long as it exists — on the day it is saved
-- and every day after. That is ADR-014's rule and the reason it survives a
-- round trip through storage: `translation_cache` keeps the model beside the
-- text for exactly the same reason, so the second view can be as honest as the
-- first.
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE user_gloss (
    user_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    -- The surface form, LOWERCASED by the caller. Lowercasing on write rather
    -- than comparing with COLLATE NOCASE on read: this is keyed lookup on the
    -- hot path of every word the reader shows, and NOCASE would throw the
    -- primary key away. The same trap `idx_wordform_form` documents in the pack.
    form       TEXT NOT NULL,
    gloss      TEXT NOT NULL,
    -- 'manual' — the learner wrote it. 'machine' — a translation model did.
    source     TEXT NOT NULL,
    -- The model that produced a machine gloss, so the label can name it. Empty
    -- for a manual one.
    model      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (user_id, form)
) WITHOUT ROWID;

-- +goose Down
DROP TABLE user_gloss;
