package srs

import (
	"testing"
	"time"
)

// The deck is two piles that answer different questions: the course units the
// plan screen reports on, and what the learner mined from their own reading.
// These pin that scoping is a FILTER over one scheduler, never a second one.

func scopedNote(t *testing.T, s *Store, userID int64, source, target string, now time.Time) {
	t.Helper()
	if _, _, err := s.CreateNote(Note{
		UserID: userID, Kind: "sentence", HU: "A házban vagyok.",
		TargetForm: target, TargetStart: -1, TargetEnd: -1,
		GlossEN: "in the house", Source: source,
	}, []Direction{Recognize}, now); err != nil {
		t.Fatalf("note %s/%s: %v", source, target, err)
	}
}

func TestScopeSplitsTheDeckByWhereItCameFrom(t *testing.T) {
	e, userID := newEngine(t)
	s, now := e.store, time.Now()

	for _, target := range []string{"a", "b", "c"} {
		scopedNote(t, s, userID, "course:m1-u1", target, now)
	}
	for _, target := range []string{"d", "e"} {
		scopedNote(t, s, userID, "manual", target, now)
	}

	for _, tc := range []struct {
		scope Scope
		want  int
	}{{ScopeCourse, 3}, {ScopeReader, 2}, {ScopeAll, 5}} {
		items, _, err := s.Queue(userID, tc.scope, 0, 0, true, now)
		if err != nil {
			t.Fatalf("%s: %v", tc.scope, err)
		}
		if len(items) != tc.want {
			t.Errorf("%s returned %d cards, want %d", tc.scope, len(items), tc.want)
		}
	}
}

// The invariant the segmented control is built on. If it stops holding, the
// tabs stop adding up and there is no way to tell from the screen.
func TestScopesSumToAll(t *testing.T) {
	e, userID := newEngine(t)
	s, now := e.store, time.Now()
	scopedNote(t, s, userID, "course:m1-u1", "a", now)
	scopedNote(t, s, userID, "manual", "b", now)

	bds, err := s.Breakdowns(userID, true, now)
	if err != nil {
		t.Fatalf("breakdowns: %v", err)
	}
	by := map[Scope]Breakdown{}
	for _, b := range bds {
		by[b.Scope] = b
	}
	if len(bds) != len(Scopes) {
		t.Fatalf("got %d scopes, want every one of %v", len(bds), Scopes)
	}
	// Field by field: the segmented control's arithmetic rests on this.
	all, course, reader := by[ScopeAll], by[ScopeCourse], by[ScopeReader]
	if all.NewCards != course.NewCards+reader.NewCards ||
		all.NewNotes != course.NewNotes+reader.NewNotes ||
		all.DueNow != course.DueNow+reader.DueNow ||
		all.DueLaterToday != course.DueLaterToday+reader.DueLaterToday ||
		all.BuriedToday != course.BuriedToday+reader.BuriedToday {
		t.Errorf("all != course + reader:\nall %+v\ncourse %+v\nreader %+v", all, course, reader)
	}
}

// `reader` is the COMPLEMENT of `course`, not `source = 'manual'`. A source
// added later must land under a tab rather than vanishing from every one.
func TestAnUnfamiliarSourceCountsAsReader(t *testing.T) {
	e, userID := newEngine(t)
	s, now := e.store, time.Now()
	scopedNote(t, s, userID, "llm", "a", now)

	items, _, err := s.Queue(userID, ScopeReader, 0, 0, true, now)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("an unfamiliar source vanished from the reader pile: got %d cards", len(items))
	}
}

// Due and new are separate obligations. The reader pile really does sit at 0
// due and 88 new on a live deck, and one merged figure would either hide that
// backlog or invent one.
func TestScopeCountsSeparateDueFromNew(t *testing.T) {
	e, userID := newEngine(t)
	s, now := e.store, time.Now()
	scopedNote(t, s, userID, "manual", "a", now)

	bds, err := s.Breakdowns(userID, true, now)
	if err != nil {
		t.Fatalf("breakdowns: %v", err)
	}
	for _, b := range bds {
		if b.Scope != ScopeReader {
			continue
		}
		if b.DueNow != 0 || b.NewCards != 1 {
			t.Errorf("reader = %d due / %d new, want 0 due and 1 new", b.DueNow, b.NewCards)
		}
	}
}

// The whole argument of ADR-016 in one test. Scoping chooses which new cards
// are ELIGIBLE; it never enlarges the daily budget. Were the cap per scope, a
// learner would take `newLimit` course cards AND `newLimit` reader cards in a
// day while the setting said one number — the review avalanche the cap exists
// to prevent, arrived at by using the feature as intended.
func TestTheDailyNewCapIsSharedAcrossScopes(t *testing.T) {
	e, userID := newEngine(t)
	s, now := e.store, time.Now()

	const limit = 2
	for _, target := range []string{"a", "b", "c"} {
		scopedNote(t, s, userID, "course:m1-u1", target, now)
	}
	for _, target := range []string{"d", "e", "f"} {
		scopedNote(t, s, userID, "manual", target, now)
	}

	course, _, err := s.Queue(userID, ScopeCourse, limit, 0, true, now)
	if err != nil {
		t.Fatalf("course queue: %v", err)
	}
	if len(course) != limit {
		t.Fatalf("course session has %d cards, want the budget of %d", len(course), limit)
	}
	// Spend the budget for real: the cap counts cards actually STARTED.
	for _, it := range course {
		if _, err := e.Answer(userID, mustCard(t, s, userID, it.CardID),
			mustNote(t, s, userID, it.NoteID), Outcome{Correct: true}, "", now); err != nil {
			t.Fatalf("answer: %v", err)
		}
	}

	reader, counts, err := s.Queue(userID, ScopeReader, limit, 0, true, now)
	if err != nil {
		t.Fatalf("reader queue: %v", err)
	}
	if len(reader) != 0 {
		t.Errorf("the reader pile served %d new cards after the day's budget was spent elsewhere", len(reader))
	}
	if counts.IntroducedToday != limit {
		t.Errorf("introducedToday = %d, want %d — the tally is the whole deck's, not one pile's",
			counts.IntroducedToday, limit)
	}
}

// A text scope is a SUB-slice of the reader pile (ADR-022): its notes are
// reader notes linked through note_origin, and `all == course + reader` keeps
// holding with no fourth term.
func TestTextScopeIsASubsetOfReader(t *testing.T) {
	e, userID := newEngine(t)
	s, now := e.store, time.Now()

	// Two texts the user owns; three mined words, one shared between texts.
	for _, tid := range []int64{1, 2} {
		if _, err := s.db.Exec(`
			INSERT INTO text (id, user_id, title, body) VALUES (?, ?, 't', 'b')`,
			tid, userID); err != nil {
			t.Fatal(err)
		}
	}
	scopedNote(t, s, userID, "manual", "a", now)
	scopedNote(t, s, userID, "manual", "b", now)
	scopedNote(t, s, userID, "manual", "c", now)
	var ids []int64
	rows, err := s.db.Query(`SELECT id FROM note WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, link := range []struct{ note, text int64 }{
		{ids[0], 1}, {ids[1], 1}, {ids[1], 2}, {ids[2], 2},
	} {
		if err := s.LinkNoteOrigin(link.note, link.text); err != nil {
			t.Fatal(err)
		}
	}

	scope, ok := ParseScope("text:1")
	if !ok {
		t.Fatal("text:1 refused")
	}
	items, _, err := s.Queue(userID, scope, 0, 0, true, now)
	if err != nil {
		t.Fatalf("text queue: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("text 1 served %d words, want its 2", len(items))
	}
	if n, err := s.TextNoteCount(userID, 2); err != nil || n != 2 {
		t.Errorf("text 2 note count = %d (%v), want 2", n, err)
	}

	bds, err := s.Breakdowns(userID, true, now, scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(bds) != len(Scopes)+1 {
		t.Fatalf("breakdowns = %d entries, want the fixed three plus the text", len(bds))
	}
	text := breakdownFor(bds, scope)
	reader := breakdownFor(bds, ScopeReader)
	if text.NewNotes != 2 || reader.NewNotes != 3 {
		t.Errorf("text/reader new notes = %d/%d, want 2/3 — a sub-slice, not a pile",
			text.NewNotes, reader.NewNotes)
	}
}

// The id is parsed strictly BEFORE it is ever inlined into SQL; anything else
// is refused the way an unknown scope always was.
func TestParseScopeRejectsMalformedTextIds(t *testing.T) {
	for _, bad := range []string{"text:", "text:abc", "text:1 OR 1=1", "text:-3", "text:1.5", "texts:1"} {
		if _, ok := ParseScope(bad); ok {
			t.Errorf("ParseScope accepted %q", bad)
		}
	}
	if sc, ok := ParseScope("text:0042"); !ok || sc != Scope("text:42") {
		t.Errorf("ParseScope did not canonicalise: %q %v", sc, ok)
	}
}

// ADR-016 extends to the text scope: it filters eligibility, never enlarges
// the day's budget.
func TestTheDailyNewCapIsSharedWithTextScope(t *testing.T) {
	e, userID := newEngine(t)
	s, now := e.store, time.Now()
	if _, err := s.db.Exec(`
		INSERT INTO text (id, user_id, title, body) VALUES (1, ?, 't', 'b')`, userID); err != nil {
		t.Fatal(err)
	}
	scopedNote(t, s, userID, "manual", "a", now)
	scopedNote(t, s, userID, "manual", "b", now)
	var noteID int64
	if err := s.db.QueryRow(`SELECT MIN(id) FROM note WHERE user_id = ?`, userID).Scan(&noteID); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkNoteOrigin(noteID, 1); err != nil {
		t.Fatal(err)
	}

	scope, _ := ParseScope("text:1")
	const limit = 1
	served, _, err := s.Queue(userID, scope, limit, 0, true, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(served) != 1 {
		t.Fatalf("text scope served %d, want 1", len(served))
	}
	if _, err := e.Answer(userID, mustCard(t, s, userID, served[0].CardID),
		mustNote(t, s, userID, served[0].NoteID), Outcome{Correct: true}, "", now); err != nil {
		t.Fatal(err)
	}

	reader, counts, err := s.Queue(userID, ScopeReader, limit, 0, true, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(reader) != 0 {
		t.Errorf("the reader pile served %d new cards after the budget was spent in a text scope", len(reader))
	}
	if counts.IntroducedToday != limit {
		t.Errorf("introducedToday = %d, want %d", counts.IntroducedToday, limit)
	}
}

func mustCard(t *testing.T, s *Store, userID, id int64) Card {
	t.Helper()
	c, err := s.Card(userID, id)
	if err != nil {
		t.Fatalf("card %d: %v", id, err)
	}
	return c
}

func mustNote(t *testing.T, s *Store, userID, id int64) Note {
	t.Helper()
	n, err := s.Note(userID, id)
	if err != nil {
		t.Fatalf("note %d: %v", id, err)
	}
	return n
}

// The card can say which unit a word came from; a word the learner added
// themselves says nothing. Display-only: it is read off note.source.
func TestQueueItemNamesTheCourseUnit(t *testing.T) {
	e, userID := newEngine(t)
	s, now := e.store, time.Now()
	scopedNote(t, s, userID, "course:m1-u1", "a", now)
	scopedNote(t, s, userID, "manual", "b", now)

	items, _, err := s.Queue(userID, ScopeAll, 0, 0, true, now)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, it := range items {
		got[it.TargetForm] = it.Unit
	}
	if got["a"] != "m1-u1" || got["b"] != "" {
		t.Errorf("units = %v, want a=m1-u1 and b empty", got)
	}
}
