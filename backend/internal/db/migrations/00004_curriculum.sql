-- +goose Up

-- ─────────────────────────────────────────────────────────────────────────
-- The curriculum and its gate.
--
-- The topic DAG itself lives in the language pack (`grammar_topic`), because it
-- is language knowledge and not learner state. What lives here is the learner's
-- position in it — and, above all, the PHASE, which is the visible half of the
-- blocked -> interleaved gate.
--
-- The gate exists because the source brief was wrong about interleaving. It
-- asked for interleaved practice from introduction; the evidence says that is
-- the worst case for an explicit, rule-based, no-L1-analogue distinction taught
-- in the early stage. So: blocked practice until the learner clears a competence
-- floor, then interleaving, with a way back down. See [R5] —
-- .claude/research/05-interleaving.md.
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE topic_progress (
    user_id    INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    -- `grammar_topic.code` in the pack. Not a foreign key: the pack is a
    -- separate, read-only database and a pack upgrade must never orphan
    -- a learner's progress.
    topic_code TEXT    NOT NULL,
    -- `explained` is the honest fifth value. Some topics — the alphabet,
    -- politeness registers, preverb detachment — have a real rule card and no
    -- drill generator behind them, and pretending otherwise would either lock
    -- the whole curriculum behind an exercise that does not exist or fake a
    -- mastery the learner never demonstrated. An explained topic teaches by its
    -- rule card ([R4]), does not gate what comes after it, and says so.
    phase      TEXT    NOT NULL DEFAULT 'locked'
               CHECK (phase IN ('locked','explained','blocked','interleaved','mastered')),

    -- TWO rolling accuracies, not one, because the gate and the mastery test ask
    -- different questions over different windows:
    --
    --   blocked_accuracy      — all answers, window `drill.gateWindow` (20)
    --   interleaved_accuracy  — INTERLEAVED answers only, window
    --                           `drill.masteryWindow` (50)
    --
    -- Mastery must be earned under interleaving; folding blocked answers into it
    -- would let a learner reach "mastered" without ever having had to choose
    -- between the contrasting forms, which is the only thing that matters here.
    --
    -- Both are exponentially weighted rather than correct/total pairs, for the
    -- reason given in 00002 on morph_state: an integer pair cannot be aged
    -- without truncating, and a learner who turns their performance around would
    -- stay locked below the gate forever.
    blocked_accuracy     REAL    NOT NULL DEFAULT 0,
    blocked_total        INTEGER NOT NULL DEFAULT 0,
    interleaved_accuracy REAL    NOT NULL DEFAULT 0,
    interleaved_total    INTEGER NOT NULL DEFAULT 0,

    -- Vowel harmony is a motor skill: correct-but-slow is not mastered. For
    -- topics flagged as motor skills the mastery test additionally requires
    -- median latency at or below `drill.harmonyLatencyMs`. See [R15].
    median_ms  INTEGER NOT NULL DEFAULT 0,

    -- Lifetime counters, for the honest "you have answered N items" line. The
    -- EMAs above deliberately forget; these do not.
    attempts   INTEGER NOT NULL DEFAULT 0,
    correct    INTEGER NOT NULL DEFAULT 0,

    -- `fallbacks` counts interleaved -> blocked demotions. It is shown to the
    -- learner rather than hidden: dropping back is a normal event, and a silent
    -- demotion would read as the app randomly changing its mind.
    fallbacks   INTEGER NOT NULL DEFAULT 0,
    phase_since TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    first_seen  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (user_id, topic_code)
);
CREATE INDEX idx_topic_progress_phase ON topic_progress(user_id, phase);

-- One row per drill item answered.
--
-- Kept in full, like review_log, for three reasons: the phase an item was served
-- in is what makes the mastery window auditable; `error_class` is what
-- `internal/drill/from_error.go` reads to build targeted follow-ups; and a
-- learner who is told "your allomorph choice is weak" is entitled to see the
-- items that produced that claim.
CREATE TABLE drill_attempt (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    topic_code  TEXT    NOT NULL,
    kind        TEXT    NOT NULL CHECK (kind IN (
                    'allomorph','case','conjugation','cloze','transform','mcq')),
    -- The phase the item was SERVED in, recorded at serve time. Deriving it
    -- later from topic_progress would be wrong: the phase moves, and an answer
    -- given under blocked practice must not retroactively count towards the
    -- interleaved mastery window.
    phase       TEXT    NOT NULL CHECK (phase IN ('blocked','interleaved','mastered')),
    prompt      TEXT    NOT NULL,
    expected    TEXT    NOT NULL,
    answer      TEXT    NOT NULL DEFAULT '',
    correct     INTEGER NOT NULL CHECK (correct IN (0,1)),
    duration_ms INTEGER NOT NULL DEFAULT 0,
    -- Pipe-separated feature ids this item actually required — the same
    -- discipline as note.target_feats. Only these are graded.
    feature_ids TEXT    NOT NULL DEFAULT '',
    error_class TEXT    NOT NULL DEFAULT '',
    origin      TEXT    NOT NULL DEFAULT 'drill'
                CHECK (origin IN ('drill','placement','from_error')),
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_drill_attempt_user  ON drill_attempt(user_id, created_at);
CREATE INDEX idx_drill_attempt_topic ON drill_attempt(user_id, topic_code, created_at);

-- +goose Down
DROP TABLE drill_attempt;
DROP TABLE topic_progress;
