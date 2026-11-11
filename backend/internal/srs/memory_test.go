package srs

import (
	"testing"
	"time"
)

// The card's memory figures (ADR-026): measured or computed by the library,
// never invented, and absent where nothing is scheduled.

func TestStageFollowsAnkisConvention(t *testing.T) {
	cases := []struct {
		name string
		st   CardState
		want Stage
	}{
		{"new", CardState{State: 0}, StageNew},
		{"learning", CardState{State: 1}, StageLearning},
		{"relearning", CardState{State: 3}, StageRelearning},
		{"young", CardState{State: 2, ScheduledDays: 20}, StageYoung},
		{"mature at 21", CardState{State: 2, ScheduledDays: 21}, StageMature},
	}
	for _, c := range cases {
		if got := StageOf(c.st); got != c.want {
			t.Errorf("%s: stage = %q, want %q", c.name, got, c.want)
		}
	}
}

// A new card has no history and no recall estimate, and says so with zeros
// rather than with an invented figure; a previewed grade still has a date.
func TestMemoryForANewCard(t *testing.T) {
	sched := NewScheduler(0.9)
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	m := MemoryFor(NewCardState(now), sched, now, true)
	if m.Stage != StageNew || m.Retrievability != 0 || m.Reps != 0 {
		t.Errorf("new card memory = %+v", m)
	}
	if m.Preview == nil || !m.Preview.Good.After(now) || !m.Preview.Easy.After(m.Preview.Good) {
		t.Errorf("preview should schedule Easy after Good after now: %+v", m.Preview)
	}
	// The instant the figures were computed from travels with them: the client
	// reads an interval as due − at, so a card reached minutes after the queue
	// was served still shows the 10-minute step as 10 minutes.
	if m.Preview != nil && !m.Preview.At.Equal(now) {
		t.Errorf("preview.at = %v, want %v", m.Preview.At, now)
	}
	if len(m.History) != 0 {
		t.Errorf("history should be empty, got %v", m.History)
	}
}

// The queue carries the tallies from the review log: how often this card was
// right, how often the whole word was, and the last outcomes in order.
func TestAttachMemoryReadsTheReviewLog(t *testing.T) {
	e, uid := newEngine(t)
	s := e.store
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)

	cards := noteWith(t, s, uid, "házban", []Direction{Recognize, Produce}, now)
	note, err := s.Note(uid, cards[0].NoteID)
	if err != nil {
		t.Fatal(err)
	}
	// Three answers on the recognize card (right, wrong, right) and one wrong
	// on produce: the card tally is 2/3, the word tally 2/4.
	outcomes := []struct {
		card    Card
		correct bool
	}{
		{cards[0], true}, {cards[0], false}, {cards[0], true}, {cards[1], false},
	}
	for i, o := range outcomes {
		card, err := s.Card(uid, o.card.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Answer(uid, card, note, Outcome{Correct: o.correct}, "", now.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}

	items, err := s.selectQueue(uid, ScopeAll, ` AND c.id = ?`, cards[0].ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("select: %v (%d items)", err, len(items))
	}
	if err := s.AttachMemory(items, e.scheduler, now.Add(time.Hour), true); err != nil {
		t.Fatal(err)
	}
	m := items[0].Memory
	if m == nil {
		t.Fatal("memory not attached")
	}
	if m.Correct != 2 || m.Total != 3 {
		t.Errorf("card tally = %d/%d, want 2/3", m.Correct, m.Total)
	}
	if m.WordCorrect != 2 || m.WordTotal != 4 {
		t.Errorf("word tally = %d/%d, want 2/4", m.WordCorrect, m.WordTotal)
	}
	if want := []bool{true, false, true}; len(m.History) != 3 || m.History[0] != want[0] || m.History[1] != want[1] || m.History[2] != want[2] {
		t.Errorf("history = %v, want %v", m.History, want)
	}
	if m.Stage == StageNew {
		t.Errorf("a card answered three times is not new")
	}
	if m.Retrievability <= 0 || m.Retrievability > 1 {
		t.Errorf("retrievability = %v, want (0,1]", m.Retrievability)
	}
	if m.Preview == nil {
		t.Error("a review queue item carries the grade preview")
	}
}

// Practice cards carry no grade preview: the round does not offer grades to
// choose from, so there is nothing to preview (ADR-064 superseded ADR-019).
func TestAttachMemoryWithoutPreviewForPractice(t *testing.T) {
	e, uid := newEngine(t)
	s := e.store
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	noteWith(t, s, uid, "házban", []Direction{Recognize}, now)

	items, err := s.PracticeQueue(uid, ScopeAll, 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("practice queue: %v (%d)", err, len(items))
	}
	if err := s.AttachMemory(items, e.scheduler, now, false); err != nil {
		t.Fatal(err)
	}
	if items[0].Memory == nil || items[0].Memory.Preview != nil {
		t.Errorf("practice memory = %+v; want figures without a preview", items[0].Memory)
	}
}

// The history keeps the last eight outcomes only: a row of marks, not a log.
func TestMemoryHistoryIsBounded(t *testing.T) {
	e, uid := newEngine(t)
	s := e.store
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	cards := noteWith(t, s, uid, "házban", []Direction{Recognize}, now)
	note, _ := s.Note(uid, cards[0].NoteID)
	for i := 0; i < 12; i++ {
		card, _ := s.Card(uid, cards[0].ID)
		if _, err := e.Answer(uid, card, note, Outcome{Correct: i%2 == 0}, "", now.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	m, err := s.CardMemory(cards[0].ID, cards[0].NoteID, CardState{State: 2, ScheduledDays: 3}, e.scheduler, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.History) != historyLength || m.Total != 12 || m.Correct != 6 {
		t.Errorf("history %d long, tally %d/%d; want %d long, 6/12", len(m.History), m.Correct, m.Total, historyLength)
	}
}

// [ADR-064] A practice answer on a new card is not counted, so a random round
// must deal the cards the learner has already met before it deals new ones;
// otherwise a deck that is mostly new gives a round that moves nothing.
func TestPracticeQueueDealsSeenCardsBeforeNewOnes(t *testing.T) {
	e, uid := newEngine(t)
	s := e.store
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	seen := map[int64]bool{}
	for i, w := range []string{"házban", "kertben", "boltban", "városban", "iskolában", "szobában", "autóban", "vízben"} {
		cards := noteWith(t, s, uid, w, []Direction{Recognize}, now)
		if i >= 2 {
			continue
		}
		note, err := s.Note(uid, cards[0].NoteID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Answer(uid, cards[0], note, Outcome{Correct: true}, "", now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		seen[cards[0].ID] = true
	}
	for round := 0; round < 5; round++ {
		items, err := s.PracticeQueue(uid, ScopeAll, 4)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 4 {
			t.Fatalf("round of %d, want 4", len(items))
		}
		got := 0
		for _, it := range items {
			if seen[it.CardID] {
				got++
			}
		}
		if got != len(seen) {
			t.Fatalf("round %d dealt %d of the %d seen cards; new cards may only fill what is left", round, got, len(seen))
		}
	}
}
