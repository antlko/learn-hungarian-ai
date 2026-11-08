package course

import "testing"

func syllabus(units ...Unit) *Syllabus {
	s := &Syllabus{Units: units, byCode: map[string]Unit{}}
	for _, u := range units {
		s.byCode[u.Code] = u
	}
	return s
}

func unit(code string, month, ord int, words ...string) Unit {
	u := Unit{Code: code, Month: month, Ord: ord}
	for _, w := range words {
		u.Words = append(u.Words, Word{Lemma: w, GlossRU: "x", GlossEN: "x"})
	}
	return u
}

func inPlay(lemmas ...string) func(string) bool {
	set := map[string]bool{}
	for _, l := range lemmas {
		set[l] = true
	}
	return func(l string) bool { return set[l] }
}

// A unit is done when its words are in the review deck — a fact about the deck,
// not a button someone pressed. That is the whole reason the plan cannot drift
// out of step with what the learner is actually studying.
func TestUnitIsDoneWhenItsWordsAreInTheDeck(t *testing.T) {
	s := syllabus(
		unit("u1", 1, 101, "szia", "kösz"),
		unit("u2", 1, 102, "bolt", "kenyér"),
	)

	plan := s.Resolve(nil, inPlay("szia", "kösz"), nil)
	if plan.Units[0].Status != StatusDone {
		t.Errorf("u1 status = %q, want done", plan.Units[0].Status)
	}
	if plan.Units[1].Status != StatusCurrent {
		t.Errorf("u2 status = %q, want current", plan.Units[1].Status)
	}
	if plan.Current != "u2" || plan.Completed != 1 || plan.Total != 2 {
		t.Errorf("plan = %+v", plan)
	}

	// Partially in play is NOT done: a unit whose progress bar rounds up would
	// tell the learner they know words they have never seen.
	partial := s.Resolve(nil, inPlay("szia"), nil)
	if partial.Units[0].Status == StatusDone {
		t.Error("one word of two must not complete a unit")
	}
	if partial.Units[0].WordsInPlay != 1 || partial.Units[0].WordsTotal != 2 {
		t.Errorf("counts = %d/%d", partial.Units[0].WordsInPlay, partial.Units[0].WordsTotal)
	}
}

// Later units are "ahead", never "locked". Nothing in the app stops a learner
// jumping to month 4, and a lock that does not exist must not be drawn.
func TestLaterUnitsAreAheadNotLocked(t *testing.T) {
	s := syllabus(
		unit("u1", 1, 101, "a"),
		unit("u2", 1, 102, "b"),
		unit("u3", 2, 201, "c"),
	)
	plan := s.Resolve(nil, inPlay(), nil)
	want := []Status{StatusCurrent, StatusAhead, StatusAhead}
	for i, st := range plan.Units {
		if st.Status != want[i] {
			t.Errorf("unit %d status = %q, want %q", i, st.Status, want[i])
		}
	}
}

// Finishing the course is a state change, not just a full progress bar: the app
// stops prescribing vocabulary and hands over to the resource catalogue.
func TestHandoverWhenEveryUnitIsDone(t *testing.T) {
	s := syllabus(unit("u1", 1, 101, "a"), unit("u2", 1, 102, "b"))

	mid := s.Resolve(nil, inPlay("a"), nil)
	if mid.HandedOver {
		t.Error("handed over with a unit outstanding")
	}

	done := s.Resolve(nil, inPlay("a", "b"), nil)
	if !done.HandedOver {
		t.Fatal("every unit done must hand over")
	}
	if done.Current != "" {
		t.Errorf("current = %q, want empty once the course is finished", done.Current)
	}
	if done.Completed != done.Total {
		t.Errorf("completed %d of %d", done.Completed, done.Total)
	}
}

// An empty syllabus must not claim to have handed over — "you finished the
// course" is a very different message from "this pack has no course".
func TestEmptySyllabusDoesNotClaimCompletion(t *testing.T) {
	s := syllabus()
	if !s.Empty() {
		t.Fatal("Empty")
	}
	plan := s.Resolve(nil, inPlay(), nil)
	if plan.HandedOver {
		t.Error("an empty syllabus must not report the course as finished")
	}
}

// Enrolment state rides along so the plan can say "taken up" separately from
// "done": a learner may enrol a unit and then not review any of it.
func TestEnrolmentIsSeparateFromCompletion(t *testing.T) {
	s := syllabus(unit("u1", 1, 101, "a", "b"))
	plan := s.Resolve(
		map[string]Progress{"u1": {UnitCode: "u1", Enrolled: true, NotesMade: 2}},
		inPlay(),
		nil,
	)
	if !plan.Units[0].Enrolled {
		t.Error("enrolment not reflected")
	}
	if plan.Units[0].Status == StatusDone {
		t.Error("enrolling must not by itself complete a unit")
	}
}

func TestSplitList(t *testing.T) {
	for in, want := range map[string]int{
		"":                0,
		"  ":              0,
		"a":               1,
		"a,b":             2,
		" a , b , ,  c  ": 3,
	} {
		if got := splitList(in); len(got) != want {
			t.Errorf("splitList(%q) = %v, want %d entries", in, got, want)
		}
	}
}

// The plan names the words that are NOT in the deck rather than only counting
// them: a count forces the UI to guess which ones, and greying out the wrong
// word is worse than greying out none.
func TestPlanNamesTheWordsNotYetInTheDeck(t *testing.T) {
	s := syllabus(unit("u1", 1, 101, "szia", "kösz", "viszlát"))
	plan := s.Resolve(nil, inPlay("kösz"), nil)

	st := plan.Units[0]
	if st.WordsInPlay != 1 {
		t.Fatalf("wordsInPlay = %d", st.WordsInPlay)
	}
	if len(st.NotInPlay) != 2 || st.NotInPlay[0] != "szia" || st.NotInPlay[1] != "viszlát" {
		t.Errorf("notInPlay = %v, want [szia viszlát] in unit order", st.NotInPlay)
	}

	// A finished unit names nothing — and the field is an empty slice rather
	// than nil so the JSON is [] and the client needs no null check.
	done := s.Resolve(nil, inPlay("szia", "kösz", "viszlát"), nil)
	if len(done.Units[0].NotInPlay) != 0 {
		t.Errorf("a done unit still names %v", done.Units[0].NotInPlay)
	}
}
