-- +goose Up

-- ─────────────────────────────────────────────────────────────────────────
-- The ear trainer (HVPT) and the listening loop.
--
-- HVPT state follows [R7] — .claude/research/07-hvpt.md — whose two
-- non-negotiables shape the schema:
--
--   * Talker variability is the ACTIVE INGREDIENT. Every trial records which
--     talker spoke, so the app can prove to the learner that a session used
--     real variety rather than asserting it.
--   * NO CONTRAST IS EVER COMPLETE. Effects decay; contrasts get FSRS
--     maintenance forever. There is deliberately no 'completed' column, and
--     adding one would be the bug.
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE hvpt_contrast_state (
    user_id  INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    -- The pack's minimal_pair.contrast key: 'a/á', 'ö/ő', 's/sz', …
    contrast TEXT    NOT NULL,
    -- FSRS-6 state, same shape as every other scheduled thing in the app.
    due            TEXT,
    stability      REAL    NOT NULL DEFAULT 0,
    difficulty     REAL    NOT NULL DEFAULT 0,
    scheduled_days INTEGER NOT NULL DEFAULT 0,
    reps           INTEGER NOT NULL DEFAULT 0,
    lapses         INTEGER NOT NULL DEFAULT 0,
    state          INTEGER NOT NULL DEFAULT 0,
    last_review    TEXT,
    -- Rolling accuracy, EWMA for the same reason as morph_state's.
    recent_accuracy REAL    NOT NULL DEFAULT 0,
    recent_total    INTEGER NOT NULL DEFAULT 0,
    first_seen      TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (user_id, contrast)
);

-- One row per 2AFC trial. Kept whole: the adaptive selector reads error
-- history from here, and "your ö/ő is weak" must always be traceable to the
-- trials that said so.
CREATE TABLE hvpt_trial (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    pair_id     INTEGER NOT NULL,
    contrast    TEXT    NOT NULL,
    -- Which member was played, which was chosen, who spoke, at what rate.
    played      TEXT    NOT NULL CHECK (played IN ('a','b')),
    chosen      TEXT    NOT NULL CHECK (chosen IN ('a','b')),
    correct     INTEGER NOT NULL CHECK (correct IN (0,1)),
    talker_id   TEXT    NOT NULL,
    rate        REAL    NOT NULL DEFAULT 1.0,
    rt_ms       INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_hvpt_trial_user     ON hvpt_trial(user_id, created_at);
CREATE INDEX idx_hvpt_trial_contrast ON hvpt_trial(user_id, contrast, created_at);

-- ─────────────────────────────────────────────────────────────────────────
-- Listening. A material is a TEXT (the transcript, ingested through the normal
-- reader pipeline — which is what makes the coverage pre-check free) plus an
-- audio file. The three-pass protocol [R13] is UI state persisted per learner.
--
-- listen_log is append-only seconds actually listened. It is the source for
-- the listening-hours gate on focus/word-order topics [R15]: the curriculum
-- reads SUM(seconds), so hours cannot be claimed, only accumulated.
-- ─────────────────────────────────────────────────────────────────────────
ALTER TABLE text ADD COLUMN audio_path TEXT NOT NULL DEFAULT '';

CREATE TABLE listen_state (
    user_id    INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    text_id    INTEGER NOT NULL REFERENCES text(id) ON DELETE CASCADE,
    -- 1 gist (no transcript) · 2 with transcript · 3 dictation/shadowing
    pass       INTEGER NOT NULL DEFAULT 1 CHECK (pass IN (1,2,3)),
    position_s REAL    NOT NULL DEFAULT 0,
    updated_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (user_id, text_id)
);

CREATE TABLE listen_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    text_id    INTEGER REFERENCES text(id) ON DELETE SET NULL,
    seconds    REAL    NOT NULL CHECK (seconds > 0),
    pass       INTEGER NOT NULL DEFAULT 1,
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_listen_log_user ON listen_log(user_id, created_at);

-- +goose Down
DROP TABLE listen_log;
DROP TABLE listen_state;
DROP TABLE hvpt_trial;
DROP TABLE hvpt_contrast_state;
-- SQLite cannot drop a column before 3.35 in all builds; recreate on the way
-- down is not worth it for a dev-only rollback. The column is harmless.
