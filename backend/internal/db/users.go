package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Roles.
const (
	RoleAdmin   = "admin"
	RoleLearner = "learner"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("not found")

// ErrDuplicateEmail is returned when an address is already taken. The uniqueness
// index is COLLATE NOCASE, so `Anna@…` and `anna@…` collide.
var ErrDuplicateEmail = errors.New("email already in use")

// ErrLastAdmin is returned when a change would leave the install with no
// administrator. Nobody could then reach settings or account management again,
// and there is no recovery path short of editing the database by hand.
var ErrLastAdmin = errors.New("last administrator")

// User is an account. PasswordHash never leaves the db and auth layers.
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
}

// CountUsers reports how many accounts exist. Zero means setup has not run.
func (d *DB) CountUsers() (int, error) {
	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM user`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}

// ValidRole reports whether a role is one the schema's CHECK constraint accepts.
// Callers validate before writing so a bad value is a 400, not a constraint error.
func ValidRole(role string) bool { return role == RoleAdmin || role == RoleLearner }

// CreateUser inserts an account and returns its id. A taken address yields
// ErrDuplicateEmail so the caller can answer 409 rather than 500.
func (d *DB) CreateUser(email, passwordHash, role string) (int64, error) {
	res, err := d.Exec(
		`INSERT INTO user (email, password_hash, role) VALUES (?, ?, ?)`,
		email, passwordHash, role,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrDuplicateEmail
		}
		return 0, fmt.Errorf("create user: %w", err)
	}
	return res.LastInsertId()
}

// ListUsers returns every account, oldest first. PasswordHash is left zero: the
// list feeds an API response, and a hash that was never loaded cannot leak.
func (d *DB) ListUsers() ([]User, error) {
	rows, err := d.Query(`SELECT id, email, role, created_at FROM user ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		var created string
		if err := rows.Scan(&u.ID, &u.Email, &u.Role, &created); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		u.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, u)
	}
	return out, rows.Err()
}

// CountAdmins reports how many accounts can still administer the install.
func (d *DB) CountAdmins() (int, error) {
	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM user WHERE role = ?`, RoleAdmin).Scan(&n); err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	return n, nil
}

// SetUserRole changes an account's role, refusing to demote the last admin.
//
// The write happens first and the invariant is checked afterwards, inside the
// same transaction: a count-then-write would let two concurrent demotions both
// observe two admins and both succeed. Rolling back a violation cannot.
func (d *DB) SetUserRole(id int64, role string) error {
	return d.inTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE user SET role = ? WHERE id = ?`, role, id)
		if err != nil {
			return fmt.Errorf("set role: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return errIfNoAdminLeft(tx)
	})
}

// SetUserPassword replaces a stored hash. Hashing is the auth package's job;
// this layer never sees a plaintext password.
func (d *DB) SetUserPassword(id int64, passwordHash string) error {
	res, err := d.Exec(`UPDATE user SET password_hash = ? WHERE id = ?`, passwordHash, id)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser removes an account and everything that hangs off it, refusing to
// remove the last admin.
//
// Sessions are deleted explicitly rather than left to ON DELETE CASCADE. The
// cascade does fire — every user_id foreign key declares it and Open sets
// foreign_keys(1) — but a live cookie outliving its account is the one failure
// here that is a security hole rather than stale data, so it does not depend on
// a pragma set in a connection string.
func (d *DB) DeleteUser(id int64) error {
	return d.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM session WHERE user_id = ?`, id); err != nil {
			return fmt.Errorf("delete sessions: %w", err)
		}
		res, err := tx.Exec(`DELETE FROM user WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("delete user: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return errIfNoAdminLeft(tx)
	})
}

func errIfNoAdminLeft(tx *sql.Tx) error {
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM user WHERE role = ?`, RoleAdmin).Scan(&n); err != nil {
		return fmt.Errorf("count admins: %w", err)
	}
	if n == 0 {
		return ErrLastAdmin
	}
	return nil
}

func (d *DB) inTx(fn func(*sql.Tx) error) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after a successful commit
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// isUniqueViolation recognises a UNIQUE constraint failure. modernc.org/sqlite
// reports it as a plain error carrying the SQLite message, so the message is
// what there is to match on.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// UserByEmail looks up an account by address. Email comparison is case-insensitive.
func (d *DB) UserByEmail(email string) (User, error) {
	return d.scanUser(d.QueryRow(
		`SELECT id, email, password_hash, role, created_at FROM user WHERE email = ?`, email))
}

// UserByID looks up an account by id.
func (d *DB) UserByID(id int64) (User, error) {
	return d.scanUser(d.QueryRow(
		`SELECT id, email, password_hash, role, created_at FROM user WHERE id = ?`, id))
}

func (d *DB) scanUser(row *sql.Row) (User, error) {
	var u User
	var created string
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("scan user: %w", err)
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return u, nil
}
