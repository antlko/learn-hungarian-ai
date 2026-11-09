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

// ADR-048: next follows ENROLMENT. Words added from the reader complete unit 2
// (done is derived from the deck) while the learner is part-way through
// unit 1; the plan must keep pointing at unit 1 instead of jumping.
func TestReaderWordsDoNotMoveNextAwayFromAnEnrolledUnit(t *testing.T) {
	s := syllabus(
		unit("u1", 1, 101, "a", "b"),
		unit("u2", 1, 102, "c", "d"),
		unit("u3", 2, 201, "e"),
	)
	progress := map[string]Progress{"u1": {UnitCode: "u1", Enrolled: true}}
	plan := s.Resolve(progress, inPlay("a", "c", "d"), nil)

	if plan.Units[1].Status != StatusDone {
		t.Fatalf("u2 = %q, want done (the deck says so)", plan.Units[1].Status)
	}
	if plan.Current != "u1" || plan.Units[0].Status != StatusCurrent {
		t.Errorf("next = %q (u1 %q), want u1: the learner started it", plan.Current, plan.Units[0].Status)
	}
	if plan.Units[2].Status != StatusAhead {
		t.Errorf("u3 = %q, want ahead", plan.Units[2].Status)
	}
}

// Nothing enrolled: the first untouched, unfinished unit — and an enrolled
// unit LATER in the list still wins over an earlier untouched one.
func TestNextPrefersEnrolledThenFirstUntouched(t *testing.T) {
	s := syllabus(
		unit("u1", 1, 101, "a"),
		unit("u2", 1, 102, "b"),
		unit("u3", 2, 201, "c"),
	)
	if plan := s.Resolve(nil, inPlay("a"), nil); plan.Current != "u2" {
		t.Errorf("fresh: next = %q, want u2", plan.Current)
	}
	jumped := map[string]Progress{"u3": {UnitCode: "u3", Enrolled: true}}
	if plan := s.Resolve(jumped, inPlay(), nil); plan.Current != "u3" {
		t.Errorf("enrolled u3: next = %q, want u3", plan.Current)
	}
	// Enrolled but finished: no longer next.
	done := map[string]Progress{"u1": {UnitCode: "u1", Enrolled: true}}
	if plan := s.Resolve(done, inPlay("a"), nil); plan.Current != "u2" {
		t.Errorf("finished enrolled unit: next = %q, want u2", plan.Current)
	}
}

func theme(code string, ord int, words ...string) Unit {
	u := unit(code, 6, ord, words...)
	u.Track = TrackTheme
	return u
}

// Theme units are optional side branches (ADR-050). They show their own
// done/ahead state but never become "next", never move the headline count and
// never gate the hand-over.
func TestThemeUnitsNeverDecideNextOrHandover(t *testing.T) {
	s := syllabus(
		unit("u1", 1, 101, "a"),
		theme("t1", 701, "red"),
		unit("u2", 1, 102, "b"),
	)

	// Fresh learner: next is the first CORE unit, theme is only "ahead".
	plan := s.Resolve(nil, inPlay(), nil)
	if plan.Current != "u1" || plan.Total != 2 {
		t.Errorf("fresh: current=%q total=%d, want u1/2", plan.Current, plan.Total)
	}
	if plan.Units[1].Status != StatusAhead {
		t.Errorf("theme status = %q, want ahead", plan.Units[1].Status)
	}

	// Enrolling a theme unit does not make it next: it started something
	// optional, the path is unchanged.
	enrolled := map[string]Progress{"t1": {UnitCode: "t1", Enrolled: true}}
	plan = s.Resolve(enrolled, inPlay(), nil)
	if plan.Current != "u1" || !plan.Units[1].Enrolled {
		t.Errorf("enrolled theme: current=%q enrolled=%v", plan.Current, plan.Units[1].Enrolled)
	}

	// A finished theme unit is done for display but does not count.
	plan = s.Resolve(nil, inPlay("red"), nil)
	if plan.Units[1].Status != StatusDone || plan.Completed != 0 || plan.Current != "u1" {
		t.Errorf("theme done: %+v", plan)
	}

	// Core finished, theme untouched: the course is handed over regardless.
	plan = s.Resolve(nil, inPlay("a", "b"), nil)
	if !plan.HandedOver || plan.Current != "" || plan.Completed != 2 || plan.Total != 2 {
		t.Errorf("core done, theme open: %+v", plan)
	}
	if plan.Units[1].Status != StatusAhead {
		t.Errorf("open theme after handover = %q, want ahead", plan.Units[1].Status)
	}

	// A syllabus of only theme units never claims completion.
	only := syllabus(theme("t1", 701, "red"))
	if p := only.Resolve(nil, inPlay("red"), nil); p.HandedOver || p.Total != 0 {
		t.Errorf("theme-only: %+v", p)
	}
}

// An empty or unknown track is core: a unit can only leave the path by saying
// "theme" explicitly.
func TestTrackDefaultsToCore(t *testing.T) {
	for _, tr := range []string{"", "core", "bogus"} {
		if (Unit{Track: tr}).IsTheme() {
			t.Errorf("track %q must be core", tr)
		}
	}
	if !(Unit{Track: TrackTheme}).IsTheme() {
		t.Error("theme must be theme")
	}
}

func tiered(code, tier string, month, ord int, words ...string) Unit {
	u := unit(code, month, ord, words...)
	u.Tier = tier
	return u
}

// The course is three finite tiers. `current`, the totals and the hand-over run
// over ALL core units in order; theme units never count (ADR-057).
func TestResolveRunsAcrossTiersAndHandsOverOnlyAfterTheLast(t *testing.T) {
	s := syllabus(
		tiered("b1", TierBasic, 1, 101, "a"),
		tiered("b2", TierBasic, 6, 601, "b"),
		theme("t1", 1001, "red"),
		tiered("i1", TierIntermediate, 7, 701, "c"),
		tiered("v1", TierAdvanced, 13, 801, "d"),
	)
	if p := s.Resolve(nil, inPlay("a", "b"), nil); p.HandedOver || p.Current != "i1" || p.Completed != 2 || p.Total != 4 {
		t.Errorf("basic done: %+v", p)
	}
	if p := s.Resolve(nil, inPlay("a", "b", "c"), nil); p.HandedOver || p.Current != "v1" {
		t.Errorf("intermediate done: handedOver=%v current=%q", p.HandedOver, p.Current)
	}
	// Every core word, no theme word: handed over.
	if p := s.Resolve(nil, inPlay("a", "b", "c", "d"), nil); !p.HandedOver || p.Current != "" || p.Completed != 4 {
		t.Errorf("all core done: %+v", p)
	}
	// The theme alone, however finished, decides nothing.
	if p := s.Resolve(nil, inPlay("red"), nil); p.HandedOver || p.Current != "b1" || p.Completed != 0 {
		t.Errorf("theme done only: %+v", p)
	}
	// An enrolled advanced unit is "next" before an untouched basic one: next
	// follows enrolment, tiers do not gate (ADR-013, ADR-048).
	jumped := map[string]Progress{"v1": {UnitCode: "v1", Enrolled: true}}
	if p := s.Resolve(jumped, inPlay(), nil); p.Current != "v1" {
		t.Errorf("enrolled advanced: current=%q", p.Current)
	}
}

// `learned` counts only words the caller calls counted, and only words that are
// in play; inPlay without counted is "in the deck, not learned".
func TestLearnedCountsCountedLemmasOnly(t *testing.T) {
	s := syllabus(
		tiered("b1", TierBasic, 1, 101, "a", "b", "c"),
		tiered("i1", TierIntermediate, 7, 701, "d", "e"),
		theme("t1", 1001, "red", "blue"),
	)
	inDeck := inPlay("a", "b", "c", "d", "red")
	counted := inPlay("a", "d", "e", "red") // e is counted but not in play: must not count
	plan := s.ResolveCounted(nil, inDeck, nil, counted, nil)

	if st := plan.Units[0]; st.WordsInPlay != 3 || st.Learned != 1 {
		t.Errorf("b1: inPlay=%d learned=%d, want 3/1", st.WordsInPlay, st.Learned)
	}
	if st := plan.Units[1]; st.WordsInPlay != 1 || st.Learned != 1 {
		t.Errorf("i1: inPlay=%d learned=%d, want 1/1", st.WordsInPlay, st.Learned)
	}
	// A unit all in the deck is `done` and still has learned < total.
	all := s.ResolveCounted(nil, inPlay("a", "b", "c"), nil, inPlay("a"), nil)
	if all.Units[0].Status != StatusDone || all.Units[0].Learned != 1 {
		t.Errorf("done unit: %+v", all.Units[0])
	}
	// No counted function: Learned stays 0 (Resolve's behaviour).
	if p := s.Resolve(nil, inDeck, nil); p.Units[0].Learned != 0 {
		t.Errorf("Resolve must not invent learned: %d", p.Units[0].Learned)
	}

	// Totals: core words only, per tier, all three tiers always present.
	tot := plan.Totals()
	if tot.Words != 5 || tot.InDeck != 4 || tot.Learned != 2 {
		t.Errorf("totals = %+v, want 5/4/2 (theme words excluded)", tot.TierTotals)
	}
	if got := tot.PerTier[TierBasic]; got != (TierTotals{Words: 3, InDeck: 3, Learned: 1}) {
		t.Errorf("basic = %+v", got)
	}
	if got := tot.PerTier[TierIntermediate]; got != (TierTotals{Words: 2, InDeck: 1, Learned: 1}) {
		t.Errorf("intermediate = %+v", got)
	}
	if got, ok := tot.PerTier[TierAdvanced]; !ok || got != (TierTotals{}) {
		t.Errorf("advanced = %+v present=%v, want an explicit zero", got, ok)
	}
}

// An unknown or empty tier is counted as basic so a bad pack row cannot drop a
// word from the totals.
func TestTotalsTreatAnUnknownTierAsBasic(t *testing.T) {
	s := syllabus(unit("u1", 1, 101, "a", "b"), tiered("u2", "weird", 2, 201, "c"))
	tot := s.Resolve(nil, inPlay("a"), nil).Totals()
	if got := tot.PerTier[TierBasic]; got.Words != 3 || got.InDeck != 1 {
		t.Errorf("basic = %+v", got)
	}
	if tot.Words != 3 {
		t.Errorf("total words = %d", tot.Words)
	}
}

// `settled` counts only settled words that are in play; a unit with no words
// has none and does not divide by anything; Resolve (no settled func) gives 0.
func TestSettledCountsSettledLemmasInPlayOnly(t *testing.T) {
	s := syllabus(
		tiered("b1", TierBasic, 1, 101, "a", "b", "c"),
		tiered("i1", TierIntermediate, 7, 701),
		tiered("a1", TierAdvanced, 13, 801, "d"),
	)
	settled := inPlay("a", "c", "d") // c is settled but not in play
	plan := s.ResolveCounted(nil, inPlay("a", "b", "d"), nil, nil, settled)
	if got := plan.Units[0].Settled; got != 1 {
		t.Errorf("b1 settled = %d, want 1", got)
	}
	if got := plan.Units[1].Settled; got != 0 {
		t.Errorf("empty unit settled = %d", got)
	}
	tot := plan.Totals()
	if tot.Settled != 2 || tot.PerTier[TierBasic].Settled != 1 || tot.PerTier[TierAdvanced].Settled != 1 {
		t.Errorf("totals = %+v", tot)
	}
	if p := s.Resolve(nil, inPlay("a"), nil); p.Units[0].Settled != 0 {
		t.Errorf("Resolve invented settled: %d", p.Units[0].Settled)
	}
	if UnlockShare != 0.70 {
		t.Errorf("UnlockShare = %v", UnlockShare)
	}
}
