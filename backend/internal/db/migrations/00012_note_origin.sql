-- +goose Up
-- Which text a note was mined from — the linkage that makes «повторить слова
-- из этого текста» possible (ADR-022).
--
-- A link table rather than a note column, because ownership is not exclusive:
-- the same sentence re-added from a second text collides with FindDuplicate's
-- (hu, target_form) key and creates NO new note — with a column, text B's
-- claim on the shared word would silently vanish. Course notes simply have no
-- rows here. Deleting a text unlinks; it never deletes notes — the learner's
-- deck outlives the paragraph it came from.
CREATE TABLE note_origin (
    note_id  INTEGER NOT NULL REFERENCES note(id) ON DELETE CASCADE,
    text_id  INTEGER NOT NULL REFERENCES text(id) ON DELETE CASCADE,
    added_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (note_id, text_id)
);

CREATE INDEX idx_note_origin_text ON note_origin(text_id, note_id);

-- +goose Down
DROP INDEX IF EXISTS idx_note_origin_text;
DROP TABLE IF EXISTS note_origin;
