package api

import (
	"net/http"
	"testing"

	"tanulom/internal/srs"
)

// Free practice is retrieval WITHOUT scheduling. These pin the "without".

// Answers come from the item's `Expected`, never from TargetForm: `produce`
// grades the DICTIONARY form, so typing the surface form is wrong for it, and a
// test that used TargetForm passed or failed according to which direction the
// random sample happened to deal.
func practiceDeck(t *testing.T, c *client) {
	t.Helper()
	for _, w := range []string{"házban", "könyvben", "házba", "könyvet", "házból"} {
		noteOn(t, c, map[string]any{"hu": "Itt van a " + w + " ma.", "targetForm": w})
	}
}

// The whole point: it opens whatever the queue says. A learner with nothing due
// must still be able to train.
func TestPracticeIsAvailableWithAnEmptyReviewQueue(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	practiceDeck(t, c)

	// Clear the day's queue the way finishing it would.
	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)
	for _, it := range queue.Items {
		c.json(http.MethodPost, "/api/review/answer",
			AnswerRequest{CardID: it.CardID, Rating: 3, Typed: it.Expected}, nil)
	}

	var p PracticeQueueResponse
	if resp := c.json(http.MethodGet, "/api/practice/queue", nil, &p); resp.StatusCode != http.StatusOK {
		t.Fatalf("practice queue: %d", resp.StatusCode)
	}
	if len(p.Items) == 0 {
		t.Error("nothing to practise with a full deck; the mode exists for exactly this moment")
	}
	if p.Pool == 0 {
		t.Error("pool = 0, so the screen would claim the deck is empty")
	}
}

// The invariant the whole feature rests on.
func TestPracticeChangesNoCardState(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	practiceDeck(t, c)

	var p PracticeQueueResponse
	c.json(http.MethodGet, "/api/practice/queue", nil, &p)
	if len(p.Items) == 0 {
		t.Fatal("no practice items")
	}
	it := p.Items[0]

	before := cardRow(t, c, it.CardID)
	var out PracticeAnswerResponse
	c.json(http.MethodPost, "/api/practice/answer",
		AnswerRequest{CardID: it.CardID, Rating: 3, Typed: it.Expected}, &out)
	after := cardRow(t, c, it.CardID)

	if before != after {
		t.Errorf("practice moved the schedule:\n  before %+v\n  after  %+v", before, after)
	}
}

// review_log is replayed to reproduce FSRS state bit for bit, and undo is
// delete-and-replay over it. A practice row in there would break both.
func TestPracticeWritesNothingToTheReviewLog(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	practiceDeck(t, c)

	var p PracticeQueueResponse
	c.json(http.MethodGet, "/api/practice/queue", nil, &p)
	for _, it := range p.Items {
		c.json(http.MethodPost, "/api/practice/answer",
			AnswerRequest{CardID: it.CardID, Rating: 3, Typed: it.Expected}, nil)
	}

	var n int
	if err := c.app.db.QueryRow(`SELECT COUNT(*) FROM review_log`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("practice wrote %d rows into review_log; undo replays that log", n)
	}
	// And undo has nothing to undo, rather than reverting somebody's practice.
	if resp := c.json(http.MethodPost, "/api/review/undo", nil, nil); resp.StatusCode == http.StatusOK {
		t.Error("undo reverted a practice answer; practice is not on the schedule")
	}
}

// Practice must not advance the second state space either: `morph_state` is an
// FSRS state space, so writing to it IS scheduling.
func TestPracticeGradesNoFeatures(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	practiceDeck(t, c)

	var p PracticeQueueResponse
	c.json(http.MethodGet, "/api/practice/queue", nil, &p)
	for _, it := range p.Items {
		c.json(http.MethodPost, "/api/practice/answer",
			AnswerRequest{CardID: it.CardID, Rating: 3, Typed: it.Expected}, nil)
	}
	var n int
	if err := c.app.db.QueryRow(`SELECT COUNT(*) FROM feature_review`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("practice emitted %d feature reviews into the morph state space", n)
	}
}

// It does not consume the daily new-card budget: nothing was introduced.
func TestPracticeDoesNotSpendTheNewCardBudget(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	practiceDeck(t, c)

	var p PracticeQueueResponse
	c.json(http.MethodGet, "/api/practice/queue", nil, &p)
	for _, it := range p.Items {
		c.json(http.MethodPost, "/api/practice/answer",
			AnswerRequest{CardID: it.CardID, Rating: 3, Typed: it.Expected}, nil)
	}
	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)
	if queue.Counts.IntroducedToday != 0 {
		t.Errorf("introducedToday = %d after practice only", queue.Counts.IntroducedToday)
	}
}

// The verdict must be the review queue's verdict. A trainer that marks `ház`
// right where the queue marks it wrong is worse than no trainer.
func TestPracticeGradesTheSameAnswerTheQueueWould(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	created := noteOn(t, c, map[string]any{
		"hu": "A házban van a könyv.", "targetForm": "házban",
		"glossRu": "дом", "glossEn": "house",
	})

	// Addressed directly rather than fished out of a random sample: the point
	// is the grading, not which card the shuffle happened to deal.
	var cardID int64
	if err := c.app.db.QueryRow(
		`SELECT id FROM card WHERE note_id = ? AND direction = ?`,
		created.NoteID, string(srs.Produce)).Scan(&cardID); err != nil {
		t.Fatalf("find the produce card: %v", err)
	}

	var out PracticeAnswerResponse
	c.json(http.MethodPost, "/api/practice/answer",
		AnswerRequest{CardID: cardID, Typed: "ház"}, &out)
	if !out.Correct || out.Expected != "ház" {
		t.Errorf("produce in practice expected %q correct=%v, want the dictionary form",
			out.Expected, out.Correct)
	}

	// ...and the inflected form is as wrong here as it is in the queue.
	c.json(http.MethodPost, "/api/practice/answer",
		AnswerRequest{CardID: cardID, Typed: "házban"}, &out)
	if out.Correct {
		t.Error("practice accepted the sentence's inflected form on a produce card")
	}
}

// Practice is reported as practice — separately, with accuracy beside volume.
func TestPracticeIsReportedSeparatelyFromReviews(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	practiceDeck(t, c)

	var p PracticeQueueResponse
	c.json(http.MethodGet, "/api/practice/queue", nil, &p)
	if len(p.Items) < 2 {
		t.Fatal("need two items")
	}
	// Rating travels too: a receptive card is self-graded and would 400 without
	// one, whichever direction the shuffle dealt.
	c.json(http.MethodPost, "/api/practice/answer",
		AnswerRequest{CardID: p.Items[0].CardID, Rating: 3, Typed: p.Items[0].Expected}, nil)
	c.json(http.MethodPost, "/api/practice/answer",
		AnswerRequest{CardID: p.Items[1].CardID, Rating: 1, Typed: "nonsense"}, nil)

	var ov OverviewResponse
	c.json(http.MethodGet, "/api/stats/overview", nil, &ov)
	if ov.Practice.Today != 2 {
		t.Errorf("practice.today = %d, want 2", ov.Practice.Today)
	}
	if ov.Practice.TodayRight != 1 {
		t.Errorf("practice.todayRight = %d, want 1", ov.Practice.TodayRight)
	}
	if ov.Today.Reviews != 0 {
		t.Errorf("reviews = %d — practice was counted as review", ov.Today.Reviews)
	}
}

// The streak needs the queue cleared as well, so practice can never stand in
// for the day's obligation however long it goes on.
func TestPracticeAloneEarnsNoStreak(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	practiceDeck(t, c)

	var p PracticeQueueResponse
	c.json(http.MethodGet, "/api/practice/queue", nil, &p)
	for _, it := range p.Items {
		c.json(http.MethodPost, "/api/practice/answer",
			AnswerRequest{CardID: it.CardID, Typed: it.Expected, DurationMs: 600000}, nil)
	}
	var ov OverviewResponse
	c.json(http.MethodGet, "/api/stats/overview", nil, &ov)
	if ov.Streak.Current != 0 {
		t.Errorf("streak = %d on practice alone", ov.Streak.Current)
	}
}

func TestPracticeRejectsAnUnknownScope(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	if resp := c.json(http.MethodGet, "/api/practice/queue?scope=nope", nil, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
}

// cardRow is the scheduling state of one card, for before/after comparison.
type cardSnapshot struct {
	Due                   string
	Stability, Difficulty float64
	Reps, Lapses, State   int
}

func cardRow(t *testing.T, c *client, id int64) cardSnapshot {
	t.Helper()
	var s cardSnapshot
	err := c.app.db.QueryRow(
		`SELECT due, stability, difficulty, reps, lapses, state FROM card WHERE id = ?`, id).
		Scan(&s.Due, &s.Stability, &s.Difficulty, &s.Reps, &s.Lapses, &s.State)
	if err != nil {
		t.Fatalf("read card %d: %v", id, err)
	}
	return s
}

// Round length and selection mode are the learner's per-session choices
// (ADR-024): size clamps to its bounds, an unknown mode is refused, weak mode
// serves the shakiest words first, and a virgin deck falls back to random —
// echoed, so the screen never claims a criterion it did not apply.
func TestPracticeSizeIsClampedAndHonoured(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	practiceDeck(t, c)

	var p PracticeQueueResponse
	c.json(http.MethodGet, "/api/practice/queue?size=3", nil, &p)
	// 3 is below MinPracticeSession, so the clamp raises it; five words exist,
	// so the session serves all five words rather than three.
	if len(p.Items) != 5 {
		t.Errorf("size=3 should clamp UP to the minimum and serve the 5-word deck, got %d", len(p.Items))
	}

	if resp := c.json(http.MethodGet, "/api/practice/queue?size=abc", nil, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("a non-numeric size answered %d, want 400", resp.StatusCode)
	}
}

func TestPracticeRejectsAnUnknownMode(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	if resp := c.json(http.MethodGet, "/api/practice/queue?mode=managed", nil, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("an unknown mode answered %d, want 400", resp.StatusCode)
	}
}

func TestWeakModeServesLowestRetrievabilityFirstAndFallsBackOnAVirginDeck(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	practiceDeck(t, c)

	// Nothing reviewed yet: weak has nothing to rank and must SAY it served
	// random instead.
	var p PracticeQueueResponse
	c.json(http.MethodGet, "/api/practice/queue?mode=weak", nil, &p)
	if p.Mode != "random" {
		t.Fatalf("virgin deck served mode %q, want the honest fallback to random", p.Mode)
	}

	// Review two words, then age one of them hard: the older answer has decayed
	// further, so weak mode must lead with it.
	var queue QueueResponse
	c.json(http.MethodGet, "/api/review/queue", nil, &queue)
	if len(queue.Items) < 2 {
		t.Fatal("need two cards")
	}
	first, second := queue.Items[0], queue.Items[1]
	for _, it := range []srs.QueueItem{first, second} {
		c.json(http.MethodPost, "/api/review/answer",
			AnswerRequest{CardID: it.CardID, Rating: 3, Typed: it.Expected}, nil)
	}
	if _, err := c.app.db.Exec(`
		UPDATE card SET last_review = '2020-01-01T00:00:00.000Z', due = '2020-01-11T00:00:00.000Z',
		       state = 2, stability = 5, difficulty = 5
		 WHERE id = ?`, first.CardID); err != nil {
		t.Fatal(err)
	}

	c.json(http.MethodGet, "/api/practice/queue?mode=weak", nil, &p)
	if p.Mode != "weak" {
		t.Fatalf("served mode %q, want weak", p.Mode)
	}
	if len(p.Items) == 0 || p.Items[0].CardID != first.CardID {
		t.Errorf("weak mode did not lead with the decayed card; got %d items", len(p.Items))
	}

	// Still ADR-019: a weak-mode answer schedules nothing.
	var before, after string
	if err := c.app.db.QueryRow(`SELECT due FROM card WHERE id = ?`, first.CardID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	c.json(http.MethodPost, "/api/practice/answer",
		AnswerRequest{CardID: first.CardID, Rating: 3, Typed: p.Items[0].Expected}, nil)
	if err := c.app.db.QueryRow(`SELECT due FROM card WHERE id = ?`, first.CardID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Error("a weak-mode practice answer moved a due date — ADR-019 broken")
	}
}
