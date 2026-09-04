package srs

import (
	"testing"
	"time"
)

// These tests pin the STORAGE round-trip of FSRS-6's learning steps, not the
// scheduler itself. The in-memory scheduler always handled steps correctly;
// what broke was that RemainingSteps never survived a save/load, so every
// answer on a reloaded card ran with 0 remaining steps — and Again/Hard on a
// Learning or Relearning card graduated it a day or more out instead of
// repeating the minutes-scale step. Migration 00011, ADR-020. Every review
// below therefore goes through SaveCard + Store.Card between answers, which is
// exactly what the answer handler does.

// storedReview applies one rating to the card AS STORED, persists the result,
// and returns the reloaded card — one full answer round-trip.
func storedReview(t *testing.T, e *Engine, userID, cardID int64, r Rating, now time.Time) Card {
	t.Helper()
	card, err := e.store.Card(userID, cardID)
	if err != nil {
		t.Fatal(err)
	}
	next, _, err := e.scheduler.Review(card.State, r, now)
	if err != nil {
		t.Fatal(err)
	}
	card.State = next
	if err := e.store.SaveCard(nil, card); err != nil {
		t.Fatal(err)
	}
	card, err = e.store.Card(userID, cardID)
	if err != nil {
		t.Fatal(err)
	}
	return card
}

func oneCardNote(t *testing.T, e *Engine, userID int64, now time.Time) int64 {
	t.Helper()
	_, cards, err := e.store.CreateNote(Note{
		UserID: userID, Kind: "sentence", HU: "A házban vagyok.",
		TargetForm: "házban", TargetStart: -1, TargetEnd: -1, GlossEN: "in the house",
	}, []Direction{Recognize}, now)
	if err != nil {
		t.Fatal(err)
	}
	return cards[0].ID
}

// The reported bug verbatim: a new card answered Good is due on its 10-minute
// step; answering it Again there must repeat the step in minutes, not graduate
// it to tomorrow.
func TestAgainOnALearningCardRepeatsTheStep(t *testing.T) {
	e, uid := newEngine(t)
	t0 := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	cardID := oneCardNote(t, e, uid, t0)

	card := storedReview(t, e, uid, cardID, Good, t0)
	if card.State.State != 1 {
		t.Fatalf("after first Good the card should be Learning, got state %d", card.State.State)
	}
	t1 := card.State.Due // the learning step elapses; the learner answers again
	if step := t1.Sub(t0); step <= 0 || step > time.Hour {
		t.Fatalf("first learning step should be minutes away, got %v", step)
	}

	card = storedReview(t, e, uid, cardID, Again, t1)
	if card.State.State != 1 {
		t.Fatalf("Again on a Learning card must stay Learning, got state %d", card.State.State)
	}
	if wait := card.State.Due.Sub(t1); wait > 10*time.Minute {
		t.Fatalf("Again on a Learning card must return in minutes, got %v", wait)
	}
}

func TestHardOnALearningCardStaysInLearning(t *testing.T) {
	e, uid := newEngine(t)
	t0 := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	cardID := oneCardNote(t, e, uid, t0)

	card := storedReview(t, e, uid, cardID, Good, t0)
	t1 := card.State.Due

	card = storedReview(t, e, uid, cardID, Hard, t1)
	if card.State.State != 1 {
		t.Fatalf("Hard on a Learning card must stay Learning, got state %d", card.State.State)
	}
	if wait := card.State.Due.Sub(t1); wait > time.Hour {
		t.Fatalf("Hard on a Learning card must return in minutes, got %v", wait)
	}
}

func TestAgainOnAMatureCardEntersRelearningThenAgainRepeatsTheStep(t *testing.T) {
	e, uid := newEngine(t)
	t0 := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	cardID := oneCardNote(t, e, uid, t0)

	// Graduate, then review once on schedule so the card is solidly state 2.
	card := storedReview(t, e, uid, cardID, Easy, t0)
	card = storedReview(t, e, uid, cardID, Good, card.State.Due)
	if card.State.State != 2 {
		t.Fatalf("setup: expected a Review-state card, got state %d", card.State.State)
	}

	// A lapse: Relearning, due on the 10-minute relearning step.
	tLapse := card.State.Due
	card = storedReview(t, e, uid, cardID, Again, tLapse)
	if card.State.State != 3 {
		t.Fatalf("Again on a mature card should enter Relearning, got state %d", card.State.State)
	}
	tStep := card.State.Due
	if wait := tStep.Sub(tLapse); wait > time.Hour {
		t.Fatalf("the relearning step should be minutes away, got %v", wait)
	}

	// Failing the step again must repeat it, not graduate back to days out.
	card = storedReview(t, e, uid, cardID, Again, tStep)
	if card.State.State != 3 {
		t.Fatalf("Again on a Relearning card must stay Relearning, got state %d", card.State.State)
	}
	if wait := card.State.Due.Sub(tStep); wait > 10*time.Minute {
		t.Fatalf("Again on a Relearning card must return in minutes, got %v", wait)
	}
}

// The storage round-trip is lossless: a chain of answers folded through the
// database equals the same chain folded in memory, field for field.
func TestStoredChainMatchesInMemoryChain(t *testing.T) {
	e, uid := newEngine(t)
	t0 := time.Date(2026, 8, 2, 8, 0, 0, 0, time.UTC)
	cardID := oneCardNote(t, e, uid, t0)

	mem := NewCardState(t0)
	now := t0
	var stored Card
	for _, r := range []Rating{Good, Again, Good, Good, Again, Good} {
		next, _, err := e.scheduler.Review(mem, r, now)
		if err != nil {
			t.Fatal(err)
		}
		mem = next
		stored = storedReview(t, e, uid, cardID, r, now)
		now = mem.Due
	}
	if !sameCardState(stored.State, mem) {
		t.Fatalf("stored chain diverged from in-memory chain:\nstored %+v\nmemory %+v",
			stored.State, mem)
	}
}
