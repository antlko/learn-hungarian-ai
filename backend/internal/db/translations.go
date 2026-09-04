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
