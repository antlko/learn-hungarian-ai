-- +goose Up

-- Users. The first account created through the setup wizard is the admin;
-- everyone else is a learner and sees only their own data.
CREATE TABLE user (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL CHECK (role IN ('admin', 'learner')),
    created_at    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- Opaque session tokens. Stored hashed so a database leak does not hand over
-- live sessions.
CREATE TABLE session (
    token_hash TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    expires_at TEXT    NOT NULL,
    user_agent TEXT
);
CREATE INDEX idx_session_user    ON session(user_id);
CREATE INDEX idx_session_expires ON session(expires_at);

-- Per-user key/value settings that are not global config.yaml concerns, plus
-- secrets. API keys live HERE and never in YAML: config.yaml is meant to be
-- readable and shareable. `secret` rows are stripped from every DTO.
CREATE TABLE settings (
    user_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    key     TEXT    NOT NULL,
    value   TEXT    NOT NULL,
    secret  INTEGER NOT NULL DEFAULT 0 CHECK (secret IN (0, 1)),
    PRIMARY KEY (user_id, key)
);

-- +goose Down
DROP TABLE settings;
DROP TABLE session;
DROP TABLE user;
