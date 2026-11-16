-- +goose Up

-- ─────────────────────────────────────────────────────────────────────────
-- A sentence translation the learner chose to keep INSTEAD of the pack's or
-- the machine's: the AI Teacher's version, saved by an explicit press.
--
-- Learner data, not a cache (compare translation_cache, 00008): it is the
-- learner's own decision, it is exported and imported with the profile, and
-- nothing regenerates it. It never feeds scheduling, a gloss or a card answer;
-- it only replaces the line printed under a sentence (the crib).
--
-- `model` is shown with the text, as everywhere a model wrote something.
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE sentence_translation_override (
    user_id     INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    source_hash TEXT    NOT NULL,
    source      TEXT    NOT NULL,
    target_lang TEXT    NOT NULL,
    text        TEXT    NOT NULL,
    model       TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (user_id, source_hash, target_lang)
) WITHOUT ROWID;

-- +goose Down
DROP TABLE sentence_translation_override;
