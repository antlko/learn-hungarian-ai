package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CreateSession stores a session by the hash of its token. The plaintext token is
// only ever held by the client, so a database leak does not yield live sessions.
func (d *DB) CreateSession(tokenHash string, userID int64, expiresAt time.Time, userAgent string) error {
	_, err := d.Exec(
		`INSERT INTO session (token_hash, user_id, expires_at, user_agent) VALUES (?, ?, ?, ?)`,
		tokenHash, userID, expiresAt.UTC().Format(time.RFC3339), userAgent,
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// UserBySession resolves a session token hash to its user, rejecting expired
// sessions. Returns ErrNotFound for unknown or expired tokens alike — callers
// must not be able to distinguish the two.
func (d *DB) UserBySession(tokenHash string) (User, error) {
	var u User
	var created, expires string
	err := d.QueryRow(`
		SELECT u.id, u.email, u.password_hash, u.role, u.created_at, s.expires_at
		  FROM session s
		  JOIN user u ON u.id = s.user_id
		 WHERE s.token_hash = ?`, tokenHash,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &created, &expires)

	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("lookup session: %w", err)
	}

	exp, err := time.Parse(time.RFC3339, expires)
	if err != nil || !time.Now().UTC().Before(exp) {
		_ = d.DeleteSession(tokenHash)
		return User{}, ErrNotFound
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return u, nil
}

// DeleteSession removes one session (logout).
func (d *DB) DeleteSession(tokenHash string) error {
	if _, err := d.Exec(`DELETE FROM session WHERE token_hash = ?`, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteOtherSessions ends every session a user has except the one identified by
// keepTokenHash.
//
// This is what a password change does: the point of changing a password is to
// evict whoever else is signed in, but signing the person out of the browser they
// just typed the new password into is a bug, not security. An empty keepTokenHash
// keeps nothing — an admin resetting someone else's password wants that account
// signed out everywhere.
func (d *DB) DeleteOtherSessions(userID int64, keepTokenHash string) (int64, error) {
	res, err := d.Exec(`DELETE FROM session WHERE user_id = ? AND token_hash <> ?`,
		userID, keepTokenHash)
	if err != nil {
		return 0, fmt.Errorf("delete other sessions: %w", err)
	}
	return res.RowsAffected()
}

// PurgeExpiredSessions drops sessions past their expiry. Called on startup and
// periodically; it is housekeeping, not a security boundary — UserBySession
// already rejects expired tokens.
func (d *DB) PurgeExpiredSessions() (int64, error) {
	res, err := d.Exec(`DELETE FROM session WHERE expires_at < ?`,
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("purge sessions: %w", err)
	}
	return res.RowsAffected()
}
