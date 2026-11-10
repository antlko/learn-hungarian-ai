// Package db wraps the SQLite database holding all learner data: users, sessions,
// secrets, notes, cards, review logs and both knowledge state spaces.
//
// The language pack is a separate, read-only file — see internal/lang. Keeping
// them apart is what lets a pack be swapped without risking learner data.
//
// Schema changes are goose migrations in migrations/*.sql. They are embedded and
// applied inside Open on every start: there is no migrate command, and the
// distroless runtime image has no shell to run one.
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // pure-Go driver: keeps CGO_ENABLED=0
)

// DB is a thin wrapper around *sql.DB carrying the domain queries.
type DB struct {
	*sql.DB
}

//go:embed migrations/*.sql
var embedMigrations embed.FS

// FileName is the database file inside the data directory.
const FileName = "tanulom.db"

// Path returns the database location inside a data directory.
func Path(dataDir string) string { return filepath.Join(dataDir, FileName) }

// Open opens (creating if needed) the database and runs migrations.
func Open(path string) (*DB, error) {
	dsn := path + "?_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=synchronous(NORMAL)"

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// SQLite has a single writer. Serializing connections avoids lock contention
	// entirely, which matters more than concurrency at this scale.
	sqlDB.SetMaxOpenConns(1)

	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	if err := migrate(sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return &DB{sqlDB}, nil
}

// gooseMu serialises migrate: goose keeps its base FS, logger and dialect in
// package globals, so two databases opening at once (parallel tests) race on
// them. Production opens one database, so the lock is free there.
var gooseMu sync.Mutex

// migrate applies the embedded goose migrations. goose skips already-applied ones.
func migrate(sqlDB *sql.DB) error {
	gooseMu.Lock()
	defer gooseMu.Unlock()
	goose.SetBaseFS(embedMigrations)
	goose.SetLogger(goose.NopLogger()) // keep goose off stdout; we log via slog
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set migrations dialect: %w", err)
	}
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
