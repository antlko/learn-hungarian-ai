package srs

import (
	"testing"
	"time"

	"tanulom/internal/db"
)

// The day-granular queue semantics of ADR-021, pinned. These are the tests
// behind the reported «10 served / +5 after a refresh / 26 on Today» session:
// every number a screen shows must be a number the queue will actually honour.

// forceCard puts a card into an exact FSRS state via SQL, the way the api tests
// backdate rows — the queue must read what is stored, not what a helper built.
func forceCard(t *testing.T, s *Store, cardID int64, state int, due, lastReview time.Time) {
	t.Helper()
	if _, err := s.db.Exec(`
		UPDATE card SET state=?, due=?, last_review=?, stability=5, difficulty=5, reps=2
		 WHERE id=?`, state, db.FmtTime(due), db.FmtTime(lastReview), cardID); err != nil {
		t.Fatal(err)
	}
}

func noteWith(t *testing.T, s *Store, userID int64, target string, dirs []Direction, now time.Time) []Card {
	t.Helper()
	_, cards, err := s.CreateNote(Note{
		UserID: userID, Kind: "sentence", HU: "A házban vagyok.",
		TargetForm: target, TargetStart: -1, TargetEnd: -1, GlossEN: "in the house",
	}, dirs, now)
	if err != nil {
		t.Fatal(err)
	}
	return cards
}

// «К повторению» must not creep upward through the day. A graduated card due at
// 23:00 is today's work at 09:00 — serving it hours early is safe (FSRS
// computes elapsed time from the last review, not from the due date), and the
// alternative is a number that grows all afternoon while the learner watches.
func TestAReviewCardDueLaterTodayIsServedNow(t *testing.T) {
	e, uid := newEngine(t)
	s := e.store
	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)

	cards := noteWith(t, s, uid, "házban", []Direction{Recognize}, now)
	forceCard(t, s, cards[0].ID, 2, now.Add(14*time.Hour), now.Add(-24*time.Hour))

	items, counts, err := s.Queue(uid, ScopeAll, 0, 0, true, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("a graduated card due later today was not served: %d items", len(items))
	}
	if counts.Due != 1 {
		t.Errorf("counts.due = %d, want 1 — the number must equal the session", counts.Due)
	}
}

// A learning step due in minutes is NOT served early — the minutes are the
// mechanism — but it must be announced: the count and the earliest due time are
// what lets the done screen say "3 return at 12:40" instead of "done for
// today", which a refresh then contradicts.
func TestALearningStepDueLaterTodayIsNotServedButAnnounced(t *testing.T) {
	e, uid := newEngine(t)
	s := e.store
	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)

	cards := noteWith(t, s, uid, "házban", []Direction{Recognize}, now)
	stepDue := now.Add(10 * time.Minute)
	forceCard(t, s, cards[0].ID, 1, stepDue, now.Add(-time.Minute))

	items, counts, err := s.Queue(uid, ScopeAll, 0, 0, true, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("a learning step 10 minutes out was served early: %d items", len(items))
	}
	if counts.DueLaterToday != 1 {
		t.Errorf("dueLaterToday = %d, want 1", counts.DueLaterToday)
	}
	if counts.NextDueAt == nil || !counts.NextDueAt.Equal(stepDue) {
		t.Errorf("nextDueAt = %v, want %v", counts.NextDueAt, stepDue)
	}

	// And once the step elapses, it is served.
	items, _, err = s.Queue(uid, ScopeAll, 0, 0, true, stepDue.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("the matured step was not served: %d items", len(items))
	}
}

// The root of «26 к повторению» over a 10-item session: due cards hidden by
// burial were counted as if they would be shown. They are still counted — but
// as BuriedToday, where the screen can explain them, and never inside DueNow.
func TestBuriedSiblingsAreCountedNotVanished(t *testing.T) {
	e, uid := newEngine(t)
	s := e.store
	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)

	cards := noteWith(t, s, uid, "házban", []Direction{Recognize, Produce}, now)
	yesterday := now.Add(-24 * time.Hour)
	for _, c := range cards {
		forceCard(t, s, c.ID, 2, now.Add(-time.Hour), yesterday)
	}

	// Before any answer: one word, two due cards. The session serves ONE card;
	// the sibling is due-but-hidden and must be reported as such.
	items, counts, err := s.Queue(uid, ScopeAll, 0, 0, true, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || counts.Due != 1 {
		t.Fatalf("served %d / due %d, want 1 / 1 — one card per word", len(items), counts.Due)
	}
	bds, err := s.Breakdowns(uid, true, now)
	if err != nil {
		t.Fatal(err)
	}
	all := breakdownFor(bds, ScopeAll)
	if all.DueNow != 1 || all.BuriedToday != 1 {
		t.Fatalf("breakdown = %d due / %d buried, want 1 / 1", all.DueNow, all.BuriedToday)
	}

	// Answer the served card: the sibling stays hidden until tomorrow, and the
	// numbers must say 0 to do now and 1 buried — not «1 к повторению».
	if _, err := e.Answer(uid, mustCard(t, s, uid, items[0].CardID),
		mustNote(t, s, uid, items[0].NoteID), Outcome{Correct: true}, "", now); err != nil {
		t.Fatal(err)
	}
	items, counts, err = s.Queue(uid, ScopeAll, 0, 0, true, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.CardID != cards[0].ID && it.CardID != cards[1].ID {
			continue
		}
		if answered := items[0].CardID; it.CardID != answered {
			t.Fatalf("the buried sibling was served: card %d", it.CardID)
		}
	}
	if counts.Due != 0 {
		t.Errorf("counts.due = %d after clearing the session, want 0", counts.Due)
	}
	bds, err = s.Breakdowns(uid, true, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if all = breakdownFor(bds, ScopeAll); all.BuriedToday != 1 {
		t.Errorf("buriedToday = %d, want the hidden sibling counted", all.BuriedToday)
	}
}

// maxReviewsPerDay is a day cap, not a fetch cap: what was answered today is
// subtracted, and a page refresh must not hand out another full batch. Learning
// steps pass the cap — a step already in flight has to be allowed to finish.
func TestMaxReviewsPerDayIsADayNotAFetch(t *testing.T) {
	e, uid := newEngine(t)
	s := e.store
	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)

	var cards []Card
	for _, target := range []string{"a", "b", "c"} {
		cards = append(cards, noteWith(t, s, uid, target, []Direction{Recognize}, now)...)
	}
	yesterday := now.Add(-24 * time.Hour)
	for _, c := range cards {
		forceCard(t, s, c.ID, 2, now.Add(-time.Hour), yesterday)
	}

	const cap = 2
	items, counts, err := s.Queue(uid, ScopeAll, 0, cap, true, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != cap || counts.ReviewsCapped != 1 {
		t.Fatalf("served %d (capped %d), want %d served and 1 capped",
			len(items), counts.ReviewsCapped, cap)
	}
	for _, it := range items {
		if _, err := e.Answer(uid, mustCard(t, s, uid, it.CardID),
			mustNote(t, s, uid, it.NoteID), Outcome{Correct: true}, "", now); err != nil {
			t.Fatal(err)
		}
	}

	// The refresh: the day's allowance is spent, so the third card waits for
	// tomorrow instead of being served by a rebuilt queue.
	items, counts, err = s.Queue(uid, ScopeAll, 0, cap, true, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("a refresh served %d more review cards past the day cap", len(items))
	}
	if counts.ReviewsCapped != 1 {
		t.Errorf("reviewsCapped = %d, want the trimmed card reported", counts.ReviewsCapped)
	}

	// A learning step is never trimmed by the cap.
	lc := noteWith(t, s, uid, "d", []Direction{Recognize}, now)
	forceCard(t, s, lc[0].ID, 1, now.Add(-time.Minute), now.Add(-11*time.Minute))
	items, _, err = s.Queue(uid, ScopeAll, 0, cap, true, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].CardID != lc[0].ID {
		t.Fatalf("an elapsed learning step must pass the review cap, got %d items", len(items))
	}
}

// The bounded new-card select must still fill the budget when the low note ids
// belong to words already answered today (their remaining directions are
// deferred, not eligible).
func TestNewSelectBoundStillFillsTheBudget(t *testing.T) {
	e, uid := newEngine(t)
	s := e.store
	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)

	const answeredWords = 8
	for i := 0; i < answeredWords; i++ {
		cards := noteWith(t, s, uid, "w", []Direction{Recognize, Produce}, now)
		if _, err := e.Answer(uid, mustCard(t, s, uid, cards[0].ID),
			mustNote(t, s, uid, cards[0].NoteID), Outcome{Correct: true}, "", now); err != nil {
			t.Fatal(err)
		}
	}
	noteWith(t, s, uid, "x", []Direction{Recognize, Produce}, now)
	noteWith(t, s, uid, "y", []Direction{Recognize, Produce}, now)

	items, counts, err := s.Queue(uid, ScopeAll, answeredWords+2, 0, true, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	fresh := 0
	for _, it := range items {
		if it.IsNew {
			fresh++
		}
	}
	if fresh != 2 || counts.NewShownNotes != 2 {
		t.Fatalf("budget of 2 filled with %d fresh cards (%d words) — the select bound starved it",
			fresh, counts.NewShownNotes)
	}
	if counts.NewDeferredNotes != answeredWords {
		t.Errorf("deferred = %d, want the %d answered words' other directions",
			counts.NewDeferredNotes, answeredWords)
	}
}

// Bug E pinned: a new word deferred because its sibling was answered today is
// NOT the daily limit's doing, and must never be reported as budget — the old
// screen said «дневная порция набрана» and linked to Settings over a limit
// that was never touched.
func TestDeferredSiblingsAreNotReportedAsBudget(t *testing.T) {
	e, uid := newEngine(t)
	s := e.store
	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)

	cards := noteWith(t, s, uid, "házban", []Direction{Recognize, Produce}, now)
	if _, err := e.Answer(uid, mustCard(t, s, uid, cards[0].ID),
		mustNote(t, s, uid, cards[0].NoteID), Outcome{Correct: true}, "", now); err != nil {
		t.Fatal(err)
	}

	_, counts, err := s.Queue(uid, ScopeAll, 20, 0, true, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if counts.NewDeferredNotes != 1 {
		t.Errorf("deferred = %d, want 1 — the produce direction of the answered word",
			counts.NewDeferredNotes)
	}
	if counts.NewBacklogNotes != 0 {
		t.Errorf("backlog = %d, want 0 — the budget held nothing back", counts.NewBacklogNotes)
	}
	if counts.NewShownNotes != 0 {
		t.Errorf("shown = %d, want 0", counts.NewShownNotes)
	}
}
