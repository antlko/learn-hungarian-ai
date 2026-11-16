package db

import (
	"database/sql"
	"errors"
	"fmt"
)

// The machine-translation cache. See migration 00008 for why it is a cache and
// not a record, and why it is scoped per user.

// CachedTranslation returns a stored translation and the model that produced
// it. A miss is ("", "", nil) — absence is the normal case here, not an error.
func (d *DB) CachedTranslation(userID int64, sourceHash, targetLang string) (string, string, error) {
	var text, model string
	err := d.QueryRow(`
		SELECT text, model FROM translation_cache
		WHERE user_id = ? AND source_hash = ? AND target_lang = ?`,
		userID, sourceHash, targetLang).Scan(&text, &model)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("read translation cache: %w", err)
	}
	return text, model, nil
}

// PutTranslation stores one translation, replacing any earlier answer for the
// same sentence. Replacing rather than keeping the first is deliberate: a newer
// answer came from whatever model is configured NOW, and the model is displayed
// with the text.
func (d *DB) PutTranslation(userID int64, sourceHash, source, targetLang, text, model string) error {
	_, err := d.Exec(`
		INSERT INTO translation_cache (user_id, source_hash, source, target_lang, text, model)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, source_hash, target_lang)
		DO UPDATE SET text = excluded.text, model = excluded.model`,
		userID, sourceHash, source, targetLang, text, model)
	if err != nil {
		return fmt.Errorf("write translation cache: %w", err)
	}
	return nil
}

// The learner's own kept translation of a sentence (migration 00022, ADR-078).
// Learner data, unlike the cache above: written only on an explicit press,
// exported with the profile.

// SentenceOverrides returns the kept translations of one sentence by target
// language, each with the model that wrote it. Missing languages are absent.
func (d *DB) SentenceOverrides(userID int64, sourceHash string) (map[string][2]string, error) {
	rows, err := d.Query(`
		SELECT target_lang, text, model FROM sentence_translation_override
		WHERE user_id = ? AND source_hash = ?`, userID, sourceHash)
	if err != nil {
		return nil, fmt.Errorf("read sentence override: %w", err)
	}
	defer rows.Close()
	out := map[string][2]string{}
	for rows.Next() {
		var lang, text, model string
		if err := rows.Scan(&lang, &text, &model); err != nil {
			return nil, err
		}
		out[lang] = [2]string{text, model}
	}
	return out, rows.Err()
}

// PutSentenceOverride keeps text as the sentence's translation, replacing any
// earlier one for the same language.
func (d *DB) PutSentenceOverride(userID int64, sourceHash, source, targetLang, text, model string) error {
	_, err := d.Exec(`
		INSERT INTO sentence_translation_override (user_id, source_hash, source, target_lang, text, model)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, source_hash, target_lang)
		DO UPDATE SET text = excluded.text, model = excluded.model,
		              created_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`,
		userID, sourceHash, source, targetLang, text, model)
	if err != nil {
		return fmt.Errorf("write sentence override: %w", err)
	}
	return nil
}

// DeleteSentenceOverride forgets the kept translation, so the original shows
// again. Deleting nothing is not an error.
func (d *DB) DeleteSentenceOverride(userID int64, sourceHash, targetLang string) error {
	_, err := d.Exec(`
		DELETE FROM sentence_translation_override
		WHERE user_id = ? AND source_hash = ? AND target_lang = ?`,
		userID, sourceHash, targetLang)
	if err != nil {
		return fmt.Errorf("delete sentence override: %w", err)
	}
	return nil
}
