-- +goose Up

-- A chat about one WORD, from the word inspector.
--
-- The review chat (00019) is keyed on `note_id`, one per word per local day,
-- and that shape is exactly right for a card. But the inspector opens on a
-- surface form the learner clicked in a text — which very often has no note
-- at all, and sometimes no lemma either, because the word is one the pack
-- does not carry. `note_id` cannot name it.
--
-- So: the form itself, lowercased by the caller the same way `user_gloss.form`
-- is. One thread per form per local day, found or created, so «ещё примеры»
-- on the same word twice in an afternoon continues the conversation instead
-- of starting a third one.
ALTER TABLE tutor_thread ADD COLUMN word TEXT;

CREATE INDEX idx_tutor_thread_word ON tutor_thread(user_id, word, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_tutor_thread_word;
ALTER TABLE tutor_thread DROP COLUMN word;
