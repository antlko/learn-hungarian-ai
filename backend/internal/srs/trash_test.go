package srs

import (
	"testing"
	"time"
)

// The trash. Three promises, one test each: the word leaves every queue and
// every count, it is still practiseable, and restoring it brings back the state
// it left with rather than a fresh card.

// trashableNote makes one reviewable note and returns its id.
func trashableNote(t *testing.T, s *Store, userID int64, source, target string, now time.Time) int64 {
	t.Helper()
	id, _, err := s.CreateNote(Note{
		UserID: userID, Kind: "sentence", HU: "A " + target + " házban vagyok.",
		TargetForm: target, TargetStart: -1, TargetEnd: -1,
		GlossEN: "in the house", Source: source,
	}, []Direction{Recognize}, now)
	if err != nil {
		t.Fatalf("note %s: %v", target, err)
	}
	return id
}

func TestTrashedWordsLeaveEveryQueueAndEveryCount(t *testing.T) {
	e, userID := newEngine(t)
	s, now := e.store, time.Now()

	trashableNote(t, s, userID, "course:m1-u1", "a", now)
	drop := trashableNote(t, s, userID, "manual", "b", now)

	if moved, err := s.Trash(userID, drop, now); err != nil || !moved {
		t.Fatalf("trash: moved=%v err=%v", moved, err)
	}

	for _, scope := range []Scope{ScopeAll, ScopeReader} {
		items, _, err := s.Queue(userID, scope, 0, 0, true, now)
		if err != nil {
			t.Fatalf("%s: %v", scope, err)
		}
		for _, it := range items {
			if it.NoteID == drop {
				t.Errorf("%s served a trashed word", scope)
			}
		}
	}

	// The counts too: a number that still includes the trashed word is the
	// screen owing a debt the queue will never present. ADR-021.
	bds, err := s.Breakdowns(userID, true, now)
	if err != nil {
		t.Fatalf("breakdowns: %v", err)
	}
	all := breakdownFor(bds, ScopeAll)
	if all.NewNotes != 1 {
		t.Errorf("all counts %d new words, want 1 — the trashed one is still in it", all.NewNotes)
	}
	// And `all == course + reader` must survive a scope that is outside it.
	course, reader := breakdownFor(bds, ScopeCourse), breakdownFor(bds, ScopeReader)
	if all.NewNotes != course.NewNotes+reader.NewNotes {
		t.Errorf("all != course + reader after a trashing: %+v %+v %+v", all, course, reader)
	}
	if reader.NewNotes != 0 {
		t.Errorf("the reader pile still counts the trashed word: %d", reader.NewNotes)
	}
}

// The trash is PRACTISEABLE and never SCHEDULABLE. Practice schedules nothing
// (ADR-019), so a word you set aside can still be looked at without anything
// about it moving.
func TestTrashIsPractiseable(t *testing.T) {
	e, userID := newEngine(t)
	s, now := e.store, time.Now()

	trashableNote(t, s, userID, "manual", "a", now)
	drop := trashableNote(t, s, userID, "manual", "b", now)
	if _, err := s.Trash(userID, drop, now); err != nil {
		t.Fatal(err)
	}

	items, err := s.PracticeQueue(userID, ScopeTrash, 10)
	if err != nil {
		t.Fatalf("practice trash: %v", err)
	}
	if len(items) != 1 || items[0].NoteID != drop {
		t.Fatalf("practising the trash served %d items, want exactly the trashed word", len(items))
	}
	// ...and the normal piles do not.
	items, err = s.PracticeQueue(userID, ScopeAll, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.NoteID == drop {
			t.Error("free practice over the whole deck served a trashed word")
		}
	}
	if n, err := s.PracticeCount(userID, ScopeTrash); err != nil || n != 1 {
		t.Errorf("trash pool = %d (err %v), want 1", n, err)
	}
}

// Restore is not re-learning. The card comes back with the FSRS state it left
// with — a word set aside for a fortnight returns OVERDUE, which is the truth
// about it.
func TestRestoreKeepsScheduledState(t *testing.T) {
	e, userID := newEngine(t)
	s, now := e.store, time.Now()

	id := trashableNote(t, s, userID, "manual", "a", now)
	items, _, err := s.Queue(userID, ScopeAll, 0, 0, true, now)
	if err != nil || len(items) != 1 {
		t.Fatalf("queue: %d items, %v", len(items), err)
	}
	card, err := s.Card(userID, items[0].CardID)
	if err != nil {
		t.Fatal(err)
	}
	note, err := s.Note(userID, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Answer(userID, card, note, Outcome{Correct: true}, "", now); err != nil {
		t.Fatal(err)
	}
	before, err := s.Card(userID, card.ID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Trash(userID, id, now); err != nil {
		t.Fatal(err)
	}
	if restored, err := s.Restore(userID, id); err != nil || !restored {
		t.Fatalf("restore: %v %v", restored, err)
	}
	after, err := s.Card(userID, card.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.State.Stability != before.State.Stability ||
		after.State.Reps != before.State.Reps ||
		!after.State.Due.Equal(before.State.Due) {
		t.Errorf("restore rescheduled the card:\nbefore %+v\nafter  %+v", before.State, after.State)
	}
	if n, err := s.TrashCount(userID); err != nil || n != 0 {
		t.Errorf("trash count = %d (err %v) after a restore, want 0", n, err)
	}
}

// A second click is not an error. Both directions report "nothing changed"
// rather than failing, so a double tap cannot look like a broken button.
func TestTrashAndRestoreAreIdempotent(t *testing.T) {
	e, userID := newEngine(t)
	s, now := e.store, time.Now()
	id := trashableNote(t, s, userID, "manual", "a", now)

	if moved, err := s.Trash(userID, id, now); err != nil || !moved {
		t.Fatalf("first trash: %v %v", moved, err)
	}
	if moved, err := s.Trash(userID, id, now); err != nil || moved {
		t.Errorf("second trash reported a move: %v %v", moved, err)
	}
	if moved, err := s.Restore(userID, id); err != nil || !moved {
		t.Fatalf("first restore: %v %v", moved, err)
	}
	if moved, err := s.Restore(userID, id); err != nil || moved {
		t.Errorf("second restore reported a move: %v %v", moved, err)
	}
}

// Trashing is not deleting: review_log survives, which is what keeps undo
// (delete-and-replay over that log) working across one.
func TestTrashKeepsTheReviewLog(t *testing.T) {
	e, userID := newEngine(t)
	s, now := e.store, time.Now()
	id := trashableNote(t, s, userID, "manual", "a", now)

	items, _, err := s.Queue(userID, ScopeAll, 0, 0, true, now)
	if err != nil || len(items) == 0 {
		t.Fatalf("queue: %v", err)
	}
	card, _ := s.Card(userID, items[0].CardID)
	note, _ := s.Note(userID, id)
	if _, err := e.Answer(userID, card, note, Outcome{Correct: true}, "", now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Trash(userID, id, now); err != nil {
		t.Fatal(err)
	}

	var logs int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM review_log WHERE card_id = ?`, card.ID).Scan(&logs); err != nil {
		t.Fatal(err)
	}
	if logs != 1 {
		t.Errorf("trashing removed %d review_log rows; it must remove none", 1-logs)
	}
	// The trash screen can still name the word.
	rows, err := s.TrashedNotes(userID, 10)
	if err != nil || len(rows) != 1 || rows[0].NoteID != id {
		t.Fatalf("trash listing: %+v (%v)", rows, err)
	}
	if rows[0].Cards != 1 {
		t.Errorf("trash row says %d cards, want 1", rows[0].Cards)
	}
}
