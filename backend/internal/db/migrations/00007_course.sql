-- +goose Up

-- ─────────────────────────────────────────────────────────────────────────
-- The learner's position in the guided course.
--
-- The syllabus itself lives in the language pack (`course_unit`,
-- `course_word`), because an ordering of Hungarian vocabulary is language
-- content and not learner state. What lives here is which units have been
-- taken up and when.
--
-- Note what is NOT here: a "completed" flag the learner can set. A unit is
-- done when its words are actually in play in the review deck — a fact the app
-- can check — not when someone presses a button. See internal/course.
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE course_progress (
    user_id    INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
    unit_code  TEXT    NOT NULL,
    -- When the learner enrolled the unit's vocabulary into the review deck.
    -- NULL means the unit has been opened but not taken up.
    enrolled_at TEXT,
    -- How many notes the enrolment actually created. Kept because it can be
    -- LOWER than the unit's word count — a word the pack has no attested
    -- sentence for is skipped rather than turned into a bare-word card, and
    -- the plan screen says so instead of silently showing a short unit.
    notes_made INTEGER NOT NULL DEFAULT 0,
    skipped    INTEGER NOT NULL DEFAULT 0,
    first_seen TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (user_id, unit_code)
);

-- +goose Down
DROP TABLE course_progress;
