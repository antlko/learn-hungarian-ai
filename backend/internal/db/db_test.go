package db

import (
	"path/filepath"
	"testing"
	"time"
)

// open returns a migrated database backed by a temp file. A file (not :memory:)
// because the WAL pragma and the single-writer setting only behave realistically
// on disk.
func open(t *testing.T) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	d1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if _, err := d1.CreateUser("a@example.com", "hash", RoleAdmin); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	d1.Close()

	// Re-opening must re-run migrations harmlessly and preserve data.
	d2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer d2.Close()

	n, err := d2.CountUsers()
	if err != nil {
		t.Fatalf("CountUsers: %v", err)
	}
	if n != 1 {
		t.Errorf("want 1 user preserved across reopen, got %d", n)
	}
}

func TestUserLifecycle(t *testing.T) {
	d := open(t)

	if n, _ := d.CountUsers(); n != 0 {
		t.Fatalf("fresh db should have no users, got %d", n)
	}

	id, err := d.CreateUser("Anna@Example.com", "hash", RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Email lookup must be case-insensitive: people capitalise inconsistently.
	u, err := d.UserByEmail("anna@example.com")
	if err != nil {
		t.Fatalf("UserByEmail: %v", err)
	}
	if u.ID != id || u.Role != RoleAdmin {
		t.Errorf("got %+v, want id=%d role=admin", u, id)
	}

	if _, err := d.UserByEmail("nobody@example.com"); err != ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}

	// Duplicate email must be rejected by the UNIQUE constraint.
	if _, err := d.CreateUser("ANNA@example.com", "hash2", RoleLearner); err == nil {
		t.Error("want duplicate email to fail, got nil")
	}
}

func TestSessionExpiry(t *testing.T) {
	d := open(t)
	uid, err := d.CreateUser("a@example.com", "hash", RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}

	if err := d.CreateSession("live-hash", uid, time.Now().Add(time.Hour), "test"); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := d.CreateSession("dead-hash", uid, time.Now().Add(-time.Hour), "test"); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if u, err := d.UserBySession("live-hash"); err != nil || u.ID != uid {
		t.Errorf("live session: got (%+v, %v), want user %d", u, err, uid)
	}
	// An expired session must be indistinguishable from an unknown one.
	if _, err := d.UserBySession("dead-hash"); err != ErrNotFound {
		t.Errorf("expired session: want ErrNotFound, got %v", err)
	}
	if _, err := d.UserBySession("never-existed"); err != ErrNotFound {
		t.Errorf("unknown session: want ErrNotFound, got %v", err)
	}

	if err := d.DeleteSession("live-hash"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := d.UserBySession("live-hash"); err != ErrNotFound {
		t.Errorf("after delete: want ErrNotFound, got %v", err)
	}
}

func TestSessionsCascadeOnUserDelete(t *testing.T) {
	d := open(t)
	uid, _ := d.CreateUser("a@example.com", "hash", RoleAdmin)
	if err := d.CreateSession("h", uid, time.Now().Add(time.Hour), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`DELETE FROM user WHERE id = ?`, uid); err != nil {
		t.Fatal(err)
	}
	// Requires foreign_keys(1) to be on in the DSN.
	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM session`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("sessions should cascade on user delete, %d left", n)
	}
}

func TestSettingsUpsertAndSecrets(t *testing.T) {
	d := open(t)
	uid, _ := d.CreateUser("a@example.com", "hash", RoleAdmin)

	if err := d.SetSetting(uid, "llmKey", "sk-first", true); err != nil {
		t.Fatal(err)
	}
	if err := d.SetSetting(uid, "llmKey", "sk-second", true); err != nil {
		t.Fatal(err)
	}
	if err := d.SetSetting(uid, "theme", "dark", false); err != nil {
		t.Fatal(err)
	}

	got, err := d.Settings(uid)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 settings after upsert, got %d: %+v", len(got), got)
	}

	byKey := map[string]Setting{}
	for _, s := range got {
		byKey[s.Key] = s
	}
	if k := byKey["llmKey"]; k.Value != "sk-second" || !k.Secret {
		t.Errorf("llmKey: got %+v, want value=sk-second secret=true", k)
	}
	if th := byKey["theme"]; th.Secret {
		t.Error("theme should not be marked secret")
	}
}

func TestPurgeExpiredSessions(t *testing.T) {
	d := open(t)
	uid, _ := d.CreateUser("a@example.com", "hash", RoleAdmin)
	_ = d.CreateSession("old", uid, time.Now().Add(-time.Hour), "")
	_ = d.CreateSession("new", uid, time.Now().Add(time.Hour), "")

	n, err := d.PurgeExpiredSessions()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("want 1 purged, got %d", n)
	}
	if _, err := d.UserBySession("new"); err != nil {
		t.Errorf("live session was purged: %v", err)
	}
}
