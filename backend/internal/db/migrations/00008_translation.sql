-- +goose Up

-- ─────────────────────────────────────────────────────────────────────────
-- Machine translations the learner has already asked for.
--
-- A cache, not a record: nothing here is learner state, nothing replays from
-- it, and deleting the whole table costs one round trip to the sidecar. It
-- exists because the reader's `t` key is pressed repeatedly on the same
-- sentence and a 600M-parameter model on a CPU is not free.
--
-- Scoped to the user even though a translation is a fact about a sentence and
-- not about a person: the SOURCE text is the learner's own paste, and in a
-- multi-user install one account's reading material has no business sitting in
-- a row another account can reach.
--
-- `model` is stored with the text because it is shown with the text. Every
-- translation in this app is labelled as machine output and names the model
-- that produced it (ADR-014); a cache that dropped the attribution would make
-- that label impossible to render honestly on the second view.
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE translation_cache (
    user_id     INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    -- SHA-256 of the source text, so the key stays a fixed width whatever the
    -- learner pasted. The source is kept beside it for debugging and so a
    -- collision would be visible rather than silently served.
    source_hash TEXT    NOT NULL,
    source      TEXT    NOT NULL,
    target_lang TEXT    NOT NULL,
    text        TEXT    NOT NULL,
    model       TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (user_id, source_hash, target_lang)
) WITHOUT ROWID;

-- +goose Down
DROP TABLE translation_cache;
