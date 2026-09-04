package db

import (
	"database/sql"
	"errors"
	"fmt"
)

// app_meta holds one-time maintenance markers — facts about the database file,
// not about a user (which is what `settings` is for). Example: the FSRS steps
// reconciliation that replays every review log once after migration 00011.

// Meta returns one marker value. Missing keys yield ("", false, nil): absence
// is the normal state of a marker that has not been earned yet.
func (d *DB) Meta(key string) (string, bool, error) {
	var v string
	err := d.QueryRow(`SELECT value FROM app_meta WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read app_meta %q: %w", key, err)
	}
	return v, true, nil
}

// SetMeta records a marker.
func (d *DB) SetMeta(key, value string) error {
	_, err := d.Exec(`
		INSERT INTO app_meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	if err != nil {
		return fmt.Errorf("write app_meta %q: %w", key, err)
	}
	return nil
}
