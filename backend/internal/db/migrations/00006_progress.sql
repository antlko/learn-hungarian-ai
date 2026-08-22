-- +goose Up

-- ─────────────────────────────────────────────────────────────────────────
-- Progress: the daily ledger and the checkpoints.
--
-- daily_log is written AT INTERACTION TIME, one row per user per day, updated
-- as the day goes. The streak is then computed from these rows with one rule
-- from [R14]: a day counts when OBLIGATIONS were met — due reviews cleared and
-- the stated minutes reached — not when the app was opened. A day with no row
-- is a day with no work, and it breaks the streak; that is the design, not an
-- edge case, because "пропущенные дни" is the first of the three documented
-- failure modes.
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE daily_log (
    user_id     INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    day         TEXT    NOT NULL,              -- YYYY-MM-DD, user-local
    reviews     INTEGER NOT NULL DEFAULT 0,
    drills      INTEGER NOT NULL DEFAULT 0,
    trials      INTEGER NOT NULL DEFAULT 0,    -- HVPT
    listen_s    REAL    NOT NULL DEFAULT 0,
    -- Accumulated graded-interaction time, milliseconds. An approximation of
    -- effort by design: idle screen time deliberately does not count.
    active_ms   INTEGER NOT NULL DEFAULT 0,
    -- Cards still due at the LAST write of the day. 0 means the queue was
    -- cleared at that moment; the streak reads this.
    due_left    INTEGER NOT NULL DEFAULT 0,
    -- Loop 4: the learner attests that live practice happened. The app cannot
    -- verify it and does not count it toward the streak — it is signal for the
    -- checkpoint diagnostic, nothing more.
    human       INTEGER NOT NULL DEFAULT 0 CHECK (human IN (0,1)),
    updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (user_id, day)
);

-- Checkpoints are the source plan's M1/M3/M6/M12/M18 assessments — pass/fail
-- gates with measured criteria (lemma counts, adherence) and human-verified
-- criteria the app can only record as the learner's own attestation, clearly
-- labelled as such. A FAILED checkpoint blocks curriculum advance beyond its
-- level until it is retaken; that is the source plan's "правило отката".
CREATE TABLE checkpoint (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    code        TEXT    NOT NULL CHECK (code IN ('m1','m3','m6','m12','m18')),
    status      TEXT    NOT NULL CHECK (status IN ('passed','failed')),
    -- The evidence at decision time, JSON — measured numbers AND the learner's
    -- attestations, so a later reader can see what the verdict rested on.
    evidence    TEXT    NOT NULL DEFAULT '{}',
    notes       TEXT    NOT NULL DEFAULT '',
    decided_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_checkpoint_user ON checkpoint(user_id, decided_at);

-- +goose Down
DROP TABLE checkpoint;
DROP TABLE daily_log;
