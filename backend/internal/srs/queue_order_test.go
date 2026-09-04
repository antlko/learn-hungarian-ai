package srs

import (
	"testing"
	"time"
)

// The learner's explicit order choice (ADR-026): it changes the sequence and
// nothing else, and it holds still for the day.

func TestParseOrder(t *testing.T) {
	for in, want := range map[string]Order{"": OrderDue, "due": OrderDue, "random": OrderRandom} {
		if got, ok := ParseOrder(in); !ok || got != want {
			t.Errorf("ParseOrder(%q) = %q, %v", in, got, ok)
		}
	}
	if _, ok := ParseOrder("shuffled"); ok {
		t.Error("an unknown order must be refused, not widened to the default")
	}
}

func orderedDeck(t *testing.T, s *Store, uid int64, now time.Time) []int64 {
	t.Helper()
	var ids []int64
	for i, w := range []string{"házban", "könyvben", "házba", "könyvet", "házból", "könyvből", "házat", "könyvbe"} {
		_, cards, err := s.CreateNote(Note{
			UserID: uid, Kind: "sentence", HU: "Itt van a " + w + " ma.",
			TargetForm: w, TargetStart: -1, TargetEnd: -1,
		}, []Direction{Recognize}, now)
		if err != nil {
			t.Fatal(err)
		}
		// Overdue by a different amount each, so due order is unambiguous.
		forceCard(t, s, cards[0].ID, 2, now.Add(-time.Duration(i+1)*24*time.Hour), now.Add(-time.Duration(i+2)*24*time.Hour))
		ids = append(ids, cards[0].ID)
	}
	return ids
}

func cardIDs(items []QueueItem) []int64 {
	out := make([]int64, 0, len(items))
	for _, it := range items {
		out = append(out, it.CardID)
	}
	return out
}

func sameSet(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[int64]int{}
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		seen[x]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

// Random order serves exactly the cards due order serves — only the sequence
// differs — and is deterministic within a day.
func TestRandomOrderIsAPermutationAndHoldsStillForTheDay(t *testing.T) {
	e, uid := newEngine(t)
	s := e.store
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	orderedDeck(t, s, uid, now)

	due, _, err := s.QueueOrdered(uid, ScopeAll, 0, 0, true, OrderDue, now)
	if err != nil {
		t.Fatal(err)
	}
	rnd, _, err := s.QueueOrdered(uid, ScopeAll, 0, 0, true, OrderRandom, now)
	if err != nil {
		t.Fatal(err)
	}
	if !sameSet(cardIDs(due), cardIDs(rnd)) {
		t.Fatalf("random order changed WHICH cards are served:\n due %v\n rnd %v", cardIDs(due), cardIDs(rnd))
	}
	// Due order is oldest-first: the most overdue card leads.
	if due[0].CardID != cardIDs(due)[0] || !due[0].Due.Before(due[len(due)-1].Due) {
		t.Errorf("due order is not oldest-first: %v", cardIDs(due))
	}

	again, _, err := s.QueueOrdered(uid, ScopeAll, 0, 0, true, OrderRandom, now.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for i := range rnd {
		if rnd[i].CardID != again[i].CardID {
			t.Fatalf("random order moved within the day:\n first %v\n later %v", cardIDs(rnd), cardIDs(again))
		}
	}
	// And a queue that lost its first two cards to answers keeps the rest in
	// the same relative order — a reload must not reshuffle the remainder.
	rest := rnd[2:]
	sub := make([]QueueItem, 0, len(rest))
	for _, it := range rnd {
		if it.CardID != rnd[0].CardID && it.CardID != rnd[1].CardID {
			sub = append(sub, it)
		}
	}
	shuffleDayStable(sub, uid, now.Add(5*time.Hour))
	for i := range rest {
		if rest[i].CardID != sub[i].CardID {
			t.Fatalf("remainder reordered after two answers:\n %v\n %v", cardIDs(rest), cardIDs(sub))
		}
	}
}

// Two learners with the same deck get different random orders: the seed is
// the user's, so a shared install does not deal every learner the same hand.
func TestRandomOrderDiffersPerLearner(t *testing.T) {
	items := make([]QueueItem, 0, 8)
	for i := int64(1); i <= 8; i++ {
		items = append(items, QueueItem{CardID: i})
	}
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	a := append([]QueueItem(nil), items...)
	b := append([]QueueItem(nil), items...)
	shuffleDayStable(a, 1, now)
	shuffleDayStable(b, 2, now)
	same := true
	for i := range a {
		if a[i].CardID != b[i].CardID {
			same = false
		}
	}
	if same {
		t.Error("two learners were dealt the same order")
	}
}
