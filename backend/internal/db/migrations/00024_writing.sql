-- +goose Up

-- ─────────────────────────────────────────────────────────────────────────
-- «Письмо» (Writing, ADR-085). The learner's own pieces and the checks a model
-- wrote on them. Learner data: exported and imported with the profile.
--
-- A check stores only what cannot be recomputed: the model's raw answer and
-- the exact text that was checked. The parsed errors, highlight segments and
-- links are rebuilt on every read (internal/writing), so a parser fix
-- upgrades old checks. Nothing here feeds FSRS or creates a card.
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE writing_piece (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    topic_code TEXT    NOT NULL DEFAULT '',
    title      TEXT    NOT NULL DEFAULT '',
    level      TEXT    NOT NULL DEFAULT 'A2',
    body       TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_writing_piece_user ON writing_piece(user_id, updated_at);

CREATE TABLE writing_check (
    id             INTEGER PRIMARY KEY,
    piece_id       INTEGER NOT NULL REFERENCES writing_piece(id) ON DELETE CASCADE,
    user_id        INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    source         TEXT    NOT NULL CHECK (source IN ('local', 'external')),
    model          TEXT    NOT NULL,
    lang           TEXT    NOT NULL,
    prompt_version INTEGER NOT NULL,
    checked_text   TEXT    NOT NULL,
    raw            TEXT    NOT NULL,
    interrupted    INTEGER NOT NULL DEFAULT 0,
    created_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_writing_check_piece ON writing_check(piece_id, created_at);

-- +goose Down
DROP TABLE writing_check;
DROP TABLE writing_piece;
