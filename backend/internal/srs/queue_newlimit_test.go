package srs

import (
	"testing"
	"time"
)

// The daily new-card limit has to budget what reaches the SCREEN.
//
// It used to select `limit` cards and then bury siblings, in that order. New
// cards come back ordered by note id, so a note's three directions sit next to
// each other: a limit of 20 selected 20 cards, collapsed them to about seven
// notes, and showed seven. The learner met a third of the new material their
// own setting promised, every day, and nothing on the screen said so.
func TestNewLimitBudgetsWhatIsShown(t *testing.T) {
	e, userID := newEngine(t)
	s := e.store
	now := time.Now()

	// Ten words, three directions each: the shape a course unit actually makes.
	const words = 10
	dirs := []Direction{Recognize, Produce, Cloze}
	for i := range words {
		if _, _, err := s.CreateNote(Note{
			UserID: userID, Kind: "sentence",
			HU:          "A házban vagyok.",
			TargetForm:  "házban",
			TargetStart: -1, TargetEnd: -1,
			GlossEN: "in the house",
		}, dirs, now); err != nil {
			t.Fatalf("note %d: %v", i, err)
		}
	}

	const limit = 6
	items, counts, err := s.Queue(userID, ScopeAll, limit, 0, true, now)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}

	if len(items) != limit {
		t.Errorf("session has %d cards, want the full budget of %d", len(items), limit)
	}
	// One card per note, and therefore six distinct WORDS rather than two.
	notes := map[int64]bool{}
	for _, it := range items {
		if notes[it.NoteID] {
			t.Errorf("note %d appears twice — siblings were not buried", it.NoteID)
		}
		notes[it.NoteID] = true
	}
	if len(notes) != limit {
		t.Errorf("session covers %d words, want %d", len(notes), limit)
	}

	// The counts have to speak both languages: cards for the mechanism, words
	// for the learner — and the three word figures must account for every word
	// on ONE basis: shown, deferred for a sibling, or held back by the budget.
	if counts.New != words*len(dirs) {
		t.Errorf("new cards = %d, want %d", counts.New, words*len(dirs))
	}
	if counts.NewShownNotes != limit {
		t.Errorf("words shown = %d, want %d", counts.NewShownNotes, limit)
	}
	if counts.NewDeferredNotes != 0 {
		t.Errorf("no siblings were due or answered, yet %d words read as deferred",
			counts.NewDeferredNotes)
	}
	if counts.NewBacklogNotes != words-limit {
		t.Errorf("words held back by the budget = %d, want %d", counts.NewBacklogNotes, words-limit)
	}
	if counts.NewPerDay != limit {
		t.Errorf("the limit itself must be reported, got %d", counts.NewPerDay)
	}
	if counts.NewBudgetLeft != limit {
		t.Errorf("budget left at build time = %d, want %d (nothing started yet)",
			counts.NewBudgetLeft, limit)
	}
}

// A limit of zero means no limit, and must not silently mean no new cards.
func TestNewLimitZeroMeansUnlimited(t *testing.T) {
	e, userID := newEngine(t)
	s := e.store
	now := time.Now()
	for range 4 {
		if _, _, err := s.CreateNote(Note{
			UserID: userID, Kind: "sentence", HU: "Szia!", TargetForm: "Szia",
			TargetStart: -1, TargetEnd: -1, GlossEN: "hi",
		}, []Direction{Recognize}, now); err != nil {
			t.Fatal(err)
		}
	}
	items, counts, err := s.Queue(userID, ScopeAll, 0, 0, true, now)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if len(items) != 4 {
		t.Errorf("unlimited queue has %d cards, want 4", len(items))
	}
	if counts.NewBacklogNotes != 0 {
		t.Errorf("nothing should be held back, got %d", counts.NewBacklogNotes)
	}
}
