package srs

import (
	"testing"
	"time"
)

// Burial spans the DAY, not the page load.
//
// The bug this pins, reported from the app: answer a word's cloze, reload the
// page, and the SAME word's produce card is served a second later. The answer
// is still on screen in the learner's memory, so the second card tests nothing
// — and because a reload also tops the session back up to the daily budget, it
// reads as the app having forgotten every answer. `burySiblingCards` only ever
// kept one card per note within ONE queue build; a reload built a new one.
func TestSiblingsStayBuriedAcrossQueueBuilds(t *testing.T) {
	e, userID := newEngine(t)
	s, now := e.store, time.Now()

	for i := 0; i < 3; i++ {
		if _, _, err := s.CreateNote(Note{
			UserID: userID, Kind: "sentence", HU: "A házban vagyok.",
			TargetForm: "házban", TargetStart: -1, TargetEnd: -1,
			GlossEN: "in the house",
		}, []Direction{Recognize, Produce}, now.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatalf("note %d: %v", i, err)
		}
	}

	first, _, err := s.Queue(userID, ScopeAll, 0, 0, true, now)
	if err != nil {
		t.Fatalf("first queue: %v", err)
	}
	if len(first) != 3 {
		t.Fatalf("first session has %d cards, want one per note", len(first))
	}

	// Answer one of them, as a learner would, then rebuild the queue the way a
	// page reload does.
	done := first[0]
	card, err := s.Card(userID, done.CardID)
	if err != nil {
		t.Fatal(err)
	}
	note, err := s.Note(userID, done.NoteID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Answer(userID, card, note, Outcome{Correct: true}, "", now); err != nil {
		t.Fatalf("answer: %v", err)
	}

	again, _, err := s.Queue(userID, ScopeAll, 0, 0, true, now)
	if err != nil {
		t.Fatalf("second queue: %v", err)
	}
	for _, it := range again {
		if it.NoteID == done.NoteID && it.CardID != done.CardID {
			t.Errorf("the other direction of a word answered today came straight back: note %d %s",
				it.NoteID, it.Direction)
		}
	}
	if len(again) != 2 {
		t.Errorf("second session has %d cards, want the 2 words not yet answered", len(again))
	}
}

// The card that WAS answered must still come back on its own learning step:
// that is short-term scheduling working, not a sibling sneaking in.
func TestTheAnsweredCardItselfIsNotBuried(t *testing.T) {
	e, userID := newEngine(t)
	s, now := e.store, time.Now()

	if _, _, err := s.CreateNote(Note{
		UserID: userID, Kind: "sentence", HU: "A házban vagyok.",
		TargetForm: "házban", TargetStart: -1, TargetEnd: -1, GlossEN: "in the house",
	}, []Direction{Recognize}, now); err != nil {
		t.Fatal(err)
	}
	first, _, _ := s.Queue(userID, ScopeAll, 0, 0, true, now)
	if len(first) != 1 {
		t.Fatalf("want 1 card, got %d", len(first))
	}
	card, _ := s.Card(userID, first[0].CardID)
	note, _ := s.Note(userID, first[0].NoteID)
	if _, err := e.Answer(userID, card, note, Outcome{Correct: true}, "", now); err != nil {
		t.Fatal(err)
	}

	// Ten minutes on, its learning step has elapsed and it is due again.
	later := now.Add(11 * time.Minute)
	again, _, err := s.Queue(userID, ScopeAll, 0, 0, true, later)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 1 || again[0].CardID != first[0].CardID {
		t.Errorf("the answered card did not return on its learning step: %+v", again)
	}
}

// Burial is a setting. A learner who turns it off gets every direction of a
// word in one sitting, and that must keep working.
func TestBurialOffServesSiblingsAgain(t *testing.T) {
	e, userID := newEngine(t)
	s, now := e.store, time.Now()

	if _, _, err := s.CreateNote(Note{
		UserID: userID, Kind: "sentence", HU: "A házban vagyok.",
		TargetForm: "házban", TargetStart: -1, TargetEnd: -1, GlossEN: "in the house",
	}, []Direction{Recognize, Produce}, now); err != nil {
		t.Fatal(err)
	}
	first, _, _ := s.Queue(userID, ScopeAll, 0, 0, false, now)
	card, _ := s.Card(userID, first[0].CardID)
	note, _ := s.Note(userID, first[0].NoteID)
	if _, err := e.Answer(userID, card, note, Outcome{Correct: true}, "", now); err != nil {
		t.Fatal(err)
	}
	again, _, _ := s.Queue(userID, ScopeAll, 0, 0, false, now)
	if len(again) == 0 {
		t.Error("with burial off the sibling should still be offered")
	}
}
