package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

// Migration 00025 rebuilds drill_attempt (dropping the kind CHECK, adding the
// anti-repeat columns). Every row written before it must survive, with its id,
// and the new kinds must be insertable afterwards.
func TestMigration00025KeepsOldDrillAttempts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")

	raw, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	raw.SetMaxOpenConns(1)
	gooseMu.Lock()
	goose.SetBaseFS(embedMigrations)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	err = goose.UpTo(raw, "migrations", 24)
	gooseMu.Unlock()
	if err != nil {
		t.Fatalf("migrate to 24: %v", err)
	}

	if _, err := raw.Exec(`INSERT INTO user (id, email, password_hash, role)
		VALUES (1, 'a@example.com', 'x', 'admin')`); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	for i, kind := range []string{"allomorph", "cloze", "mcq", "case", "conjugation"} {
		if _, err := raw.Exec(`INSERT INTO drill_attempt
			(user_id, topic_code, kind, phase, prompt, expected, answer, correct,
			 duration_ms, feature_ids, error_class, origin)
			VALUES (1, 'accusative', ?, 'blocked', ?, 'házat', 'házat', 1, 900,
			        'Case=Acc', '', 'drill')`, kind, "p"+kind); err != nil {
			t.Fatalf("seed attempt %d: %v", i, err)
		}
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open (applies 00025): %v", err)
	}
	defer d.Close()

	var n int
	var firstID, lastID int64
	if err := d.QueryRow(`SELECT COUNT(*), MIN(id), MAX(id) FROM drill_attempt`).
		Scan(&n, &firstID, &lastID); err != nil {
		t.Fatal(err)
	}
	if n != 5 || firstID != 1 || lastID != 5 {
		t.Fatalf("rows after rebuild: n=%d ids %d..%d, want 5 rows with ids 1..5", n, firstID, lastID)
	}
	var key string
	var sent int64
	var prompt, feats string
	if err := d.QueryRow(`SELECT item_key, corpus_sentence, prompt, feature_ids
		FROM drill_attempt WHERE kind = 'cloze'`).Scan(&key, &sent, &prompt, &feats); err != nil {
		t.Fatal(err)
	}
	if key != "" || sent != 0 || prompt != "pcloze" || feats != "Case=Acc" {
		t.Errorf("old row changed: key=%q sentence=%d prompt=%q feats=%q", key, sent, prompt, feats)
	}

	// The kind CHECK is gone, so the new kinds insert; the other CHECKs remain.
	if _, err := d.Exec(`INSERT INTO drill_attempt (user_id, topic_code, kind, phase, prompt,
		expected, correct) VALUES (1, 'x', 'person', 'blocked', 'p', 'e', 0)`); err != nil {
		t.Errorf("a new kind must insert: %v", err)
	}
	if _, err := d.Exec(`INSERT INTO drill_attempt (user_id, topic_code, kind, phase, prompt,
		expected, correct) VALUES (1, 'x', 'qword', 'nonsense', 'p', 'e', 0)`); err == nil {
		t.Error("the phase CHECK must survive the rebuild")
	}

	// The index the recent-items query relies on, and the old ones.
	for _, idx := range []string{"idx_drill_attempt_user", "idx_drill_attempt_topic", "idx_drill_attempt_recent"} {
		var c int
		if err := d.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&c); err != nil || c != 1 {
			t.Errorf("index %s missing after the rebuild", idx)
		}
	}

	// Deleting the user still cascades: the foreign key came across.
	if _, err := d.Exec(`DELETE FROM user WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if err := d.QueryRow(`SELECT COUNT(*) FROM drill_attempt`).Scan(&n); err != nil || n != 0 {
		t.Errorf("attempts left after deleting the user: %d (%v)", n, err)
	}
}
