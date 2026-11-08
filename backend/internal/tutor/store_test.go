package tutor

import (
	"path/filepath"
	"testing"
	"time"

	"tanulom/internal/db"
)

// Clearing chats in bulk. ADR-045.
//
// A review chat is created once per WORD per local day, so a month of
// reviewing leaves hundreds of them and the only way out was deleting them one
// at a time. These tests exist because the operation is destructive and has no
// undo: the thing that must never happen is "clear one tab" taking the others
// with it.

func storeFor(t *testing.T) (*Store, int64, int64) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })

	me, err := d.CreateUser("me@example.com", "hash", db.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	other, err := d.CreateUser("other@example.com", "hash", db.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	return NewStore(d.DB), me, other
}

// seedThreads makes one thread of each kind for a user, each with a message,
// and returns how many it made.
func seedThreads(t *testing.T, s *Store, userID int64) {
	t.Helper()
	now := time.Now()
	for _, kind := range []string{"", "", "review", "review", "review", "word"} {
		th, err := s.CreateThread(userID, NewThread{Kind: kind, Title: "t"}, now)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Append(th.ID, Message{Role: "user", Content: "q"}, now); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCountThreadsByKindNamesTheOrdinaryChatChat(t *testing.T) {
	s, me, _ := storeFor(t)
	seedThreads(t, s, me)

	got, err := s.CountThreadsByKind(me)
	if err != nil {
		t.Fatal(err)
	}
	// The stored kind for an ordinary chat is '', which a query string cannot
	// carry — so the count keys it as `chat`.
	want := map[string]int{"chat": 2, "review": 3, "word": 1}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("counts[%q] = %d, want %d (got %v)", k, got[k], v, got)
		}
	}
	if _, ok := got[""]; ok {
		t.Error(`the empty kind must be keyed as "chat", never as ""`)
	}
}

// Clearing one kind leaves the others alone. This is the whole point.
func TestDeletingOneKindLeavesTheOtherTabs(t *testing.T) {
	s, me, _ := storeFor(t)
	seedThreads(t, s, me)

	n, err := s.DeleteThreads(me, "review")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("deleted %d review chats, want 3", n)
	}

	got, err := s.CountThreadsByKind(me)
	if err != nil {
		t.Fatal(err)
	}
	if got["review"] != 0 {
		t.Errorf("review chats survived: %d", got["review"])
	}
	if got["chat"] != 2 || got["word"] != 1 {
		t.Errorf("clearing one tab took the others with it: %v", got)
	}
}

// `DeleteThreads(user, "")` means EVERY kind, and `DeleteThreadsOfChatKind`
// means the one whose stored value is ''. Confusing them would wipe a
// learner's whole tutor history while meaning to clear one tab, which is why
// they are separate methods.
func TestTheTwoEmptiesAreNotTheSameOperation(t *testing.T) {
	s, me, _ := storeFor(t)
	seedThreads(t, s, me)

	n, err := s.DeleteThreadsOfChatKind(me)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("DeleteThreadsOfChatKind removed %d, want 2 (the '' rows only)", n)
	}
	got, _ := s.CountThreadsByKind(me)
	if got["review"] != 3 || got["word"] != 1 {
		t.Fatalf("DeleteThreadsOfChatKind behaved like delete-all: %v", got)
	}

	all, err := s.DeleteThreads(me, "")
	if err != nil {
		t.Fatal(err)
	}
	if all != 4 {
		t.Errorf(`DeleteThreads(user, "") removed %d, want the remaining 4 — "" means every kind`, all)
	}
}

// Messages have no foreign key onto threads (migration 00016 added
// `thread_id` as a plain INTEGER), so nothing cascades and a delete that
// forgot them would orphan every row.
func TestDeletingThreadsTakesTheirMessages(t *testing.T) {
	s, me, _ := storeFor(t)
	seedThreads(t, s, me)

	var before int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tutor_message WHERE user_id = ?`, me).Scan(&before); err != nil {
		// user_id is on tutor_message too; if the column moves, fail loudly.
		t.Fatal(err)
	}
	if before == 0 {
		t.Fatal("the fixture stored no messages")
	}
	if _, err := s.DeleteThreads(me, ""); err != nil {
		t.Fatal(err)
	}
	var orphans int
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM tutor_message m
		 WHERE NOT EXISTS (SELECT 1 FROM tutor_thread t WHERE t.id = m.thread_id)`).Scan(&orphans); err != nil {
		t.Fatal(err)
	}
	if orphans != 0 {
		t.Errorf("%d messages were orphaned by the delete", orphans)
	}
}

// One learner's housekeeping is not another's.
func TestClearingChatsLeavesOtherLearnersAlone(t *testing.T) {
	s, me, other := storeFor(t)
	seedThreads(t, s, me)
	seedThreads(t, s, other)

	if _, err := s.DeleteThreads(me, ""); err != nil {
		t.Fatal(err)
	}
	got, err := s.CountThreadsByKind(other)
	if err != nil {
		t.Fatal(err)
	}
	if got["chat"] != 2 || got["review"] != 3 || got["word"] != 1 {
		t.Errorf("the other learner's chats changed: %v", got)
	}
}
