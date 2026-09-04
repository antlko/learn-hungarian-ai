package srs

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// Ten words mined from one paragraph are ten notes over three or four
// sentences, created in one batch — so note order, and a day later due order,
// hands them out in a run. The session then reads as the same sentence eight
// times with the blank sliding along it, and the answer to card three is a word
// the learner just read in the prompt of card two.

// sentences renders a queue as its sentence initials, so a failure prints the
// actual shape rather than a diff of structs.
func sentences(items []QueueItem) string {
	var b strings.Builder
	for _, it := range items {
		b.WriteString(it.HU)
	}
	return b.String()
}

// sorted spells a shape in canonical order, so a test can check that the same
// cards came back without caring where they landed.
func sorted(s string) string {
	b := []byte(s)
	slices.Sort(b)
	return string(b)
}

func queueOf(hus ...string) []QueueItem {
	out := make([]QueueItem, len(hus))
	for i, hu := range hus {
		out[i] = QueueItem{CardID: int64(i + 1), NoteID: int64(i + 1), HU: hu}
	}
	return out
}

// Whenever separating the repeats is possible at all, they are separated.
// Three cards of one sentence among three others alternate perfectly.
func TestSpreadSentencesSeparatesEveryRepeat(t *testing.T) {
	got := sentences(spreadSentences(queueOf("A", "A", "A", "B", "C", "D")))
	for i := 1; i < len(got); i++ {
		if got[i] == got[i-1] {
			t.Fatalf("two cards of one sentence still land together: %q", got)
		}
	}
	if sorted(got) != "AAABCD" {
		t.Errorf("spreading lost or invented cards: %q", got)
	}
}

// The property the due list depends on: a queue in which no sentence repeats is
// returned EXACTLY as it came in. This is the ordinary case — a course session
// is one sentence per card — and it is what lets this run over an oldest-first
// list without disturbing it. Nothing moves unless something repeats.
func TestSpreadSentencesLeavesAQueueWithoutRepeatsUntouched(t *testing.T) {
	in := queueOf("A", "B", "C", "D", "E")
	got := spreadSentences(in)
	if sentences(got) != "ABCDE" {
		t.Errorf("a queue with nothing to spread was reordered: %q", sentences(got))
	}
	for i := range in {
		if got[i].CardID != in[i].CardID {
			t.Fatalf("card %d moved from position %d", got[i].CardID, i)
		}
	}
}

// Best effort, and it says so rather than pretending: a queue that is nothing
// but one sentence cannot be separated, and every card must still be there.
func TestSpreadSentencesKeepsEveryCardWhenItCannotSeparateThem(t *testing.T) {
	got := sentences(spreadSentences(queueOf("A", "A", "A", "A")))
	if got != "AAAA" {
		t.Errorf("got %q, want every card kept", got)
	}
}

// A lopsided pile — one sentence with more cards than all the others together —
// cannot be fully separated either, and must still come back whole.
func TestSpreadSentencesKeepsEveryCardWhenTheyOutnumberTheRest(t *testing.T) {
	got := sentences(spreadSentences(queueOf("A", "A", "A", "A", "A", "B", "C")))
	if sorted(got) != "AAAAABC" {
		t.Errorf("spreading a lopsided queue lost cards: %q", got)
	}
}

// Deterministic: a session topped up by a maturing learning step must not look
// as though it reshuffled itself under the learner's hands.
func TestSpreadSentencesIsDeterministic(t *testing.T) {
	in := queueOf("A", "A", "B", "A", "C", "B", "A")
	first := sentences(spreadSentences(in))
	for i := range 5 {
		if got := sentences(spreadSentences(in)); got != first {
			t.Fatalf("run %d gave %q, first run gave %q", i, got, first)
		}
	}
	if sentences(in) != "AABACBA" {
		t.Errorf("spreadSentences mutated its input: %q", sentences(in))
	}
}

// End to end, through the real queue: eight words from two sentences must not
// arrive as four of one followed by four of the other.
func TestQueueDoesNotServeOneSentenceInARun(t *testing.T) {
	e, userID := newEngine(t)
	s, now := e.store, time.Now()

	// Two sentences, four words each, created in source order — exactly what
	// mining a text produces.
	for _, sentence := range []string{"Reggel hét órakor kelek.", "Este tízkor fekszem le."} {
		for _, target := range []string{"w", "x", "y", "z"} {
			if _, _, err := s.CreateNote(Note{
				UserID: userID, Kind: "sentence", HU: sentence,
				TargetForm: sentence + target, TargetStart: -1, TargetEnd: -1,
				Source: "manual",
			}, []Direction{Recognize}, now); err != nil {
				t.Fatal(err)
			}
		}
	}

	items, _, err := s.Queue(userID, ScopeAll, 0, 0, true, now)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if len(items) != 8 {
		t.Fatalf("got %d cards, want 8", len(items))
	}
	runs := 0
	for i := 1; i < len(items); i++ {
		if items[i].HU == items[i-1].HU {
			runs++
		}
	}
	// Four and four can interleave perfectly; anything else is the clumping
	// this exists to remove.
	if runs != 0 {
		t.Errorf("%d adjacent pairs share a sentence; the session still reads as one item", runs)
	}
}
