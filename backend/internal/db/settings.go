package db

import "fmt"

// Setting is one per-user key/value pair. Secret values are never rendered in a
// DTO — only their presence is exposed.
type Setting struct {
	Key    string
	Value  string
	Secret bool
}

// Settings returns every setting for a user.
func (d *DB) Settings(userID int64) ([]Setting, error) {
	rows, err := d.Query(`SELECT key, value, secret FROM settings WHERE user_id = ? ORDER BY key`, userID)
	if err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}
	defer rows.Close()

	var out []Setting
	for rows.Next() {
		var s Setting
		if err := rows.Scan(&s.Key, &s.Value, &s.Secret); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SetSetting inserts or replaces one setting.
func (d *DB) SetSetting(userID int64, key, value string, secret bool) error {
	_, err := d.Exec(`
		INSERT INTO settings (user_id, key, value, secret) VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, key) DO UPDATE SET value = excluded.value, secret = excluded.secret`,
		userID, key, value, secret)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

// Setting returns one value. Missing keys yield ("", false, nil) — absence is not
// an error for settings.
func (d *DB) Setting(userID int64, key string) (string, bool, error) {
	var v string
	err := d.QueryRow(`SELECT value FROM settings WHERE user_id = ? AND key = ?`, userID, key).Scan(&v)
	if err != nil {
		return "", false, nil
	}
	return v, true, nil
}
