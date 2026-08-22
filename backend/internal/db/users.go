package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Roles.
const (
	RoleAdmin   = "admin"
	RoleLearner = "learner"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("not found")

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

// CreateUser inserts an account and returns its id.
func (d *DB) CreateUser(email, passwordHash, role string) (int64, error) {
	res, err := d.Exec(
		`INSERT INTO user (email, password_hash, role) VALUES (?, ?, ?)`,
		email, passwordHash, role,
	)
	if err != nil {
		return 0, fmt.Errorf("create user: %w", err)
	}
	return res.LastInsertId()
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
