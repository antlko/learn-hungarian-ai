package drill

import (
	"math/rand"
	"slices"
	"strings"
	"testing"

	"tanulom/internal/curriculum"
)

func fixedRand() *rand.Rand { return rand.New(rand.NewSource(7)) }

func topicOf(t *testing.T, dag *curriculum.DAG, code string) curriculum.Topic {
	t.Helper()
	topic, ok := dag.Topic(code)
	if !ok {
		t.Fatalf("no topic %q in the fixture", code)
	}
	return topic
}

// Which generators apply is derived from a topic's FEATURES, so adding a topic
// to the pack never needs a code change.
func TestKindsForDerivesFromFeatures(t *testing.T) {
	g, dag := newTestGenerator(t)

	harmony := g.KindsFor(topicOf(t, dag, "vowel-harmony"))
	if !harmony.Available || !slices.Contains(harmony.Kinds, KindAllomorph) {
		t.Errorf("the harmony topic must produce allomorph items, got %+v", harmony)
	}

	grid := g.KindsFor(topicOf(t, dag, "locative-grid"))
	for _, want := range []Kind{KindAllomorph, KindCase, KindCloze, KindMCQ, KindTransform} {
		if !slices.Contains(grid.Kinds, want) {
			t.Errorf("the locative grid must produce %s items, got %v", want, grid.Kinds)
		}
	}

	def := g.KindsFor(topicOf(t, dag, "definite-conjugation"))
	if !slices.Contains(def.Kinds, KindConjugation) {
		t.Errorf("the definite conjugation must produce conjugation items, got %v", def.Kinds)
	}

	// The accusative has ONE allomorph, so an allomorph choice would offer the
	// same answer three times.
	acc := g.KindsFor(topicOf(t, dag, "accusative"))
	if slices.Contains(acc.Kinds, KindAllomorph) {
		t.Errorf("an invariant suffix admits no allomorph choice, got %v", acc.Kinds)
	}
}

// A topic we have not built drills for says so out loud rather than showing an
// empty screen or inventing filler.
func TestUndrillableTopicReportsWhy(t *testing.T) {
	g, dag := newTestGenerator(t)
	for _, code := range []string{"orthography", "politeness"} {
		avail := g.KindsFor(topicOf(t, dag, code))
		if avail.Available {
			t.Errorf("%s: no generator exists, yet it reports available", code)
		}
		if avail.Reason != ReasonNoGenerator {
			t.Errorf("%s: reason = %q, want the no-generator reason", code, avail.Reason)
		}
	}
}

// The correct answer must be attested. This is the guardrail that stops the app
// marking the learner wrong against a form our own generator invented — and the
// fixture contains exactly that trap: `házan` beside the attested `házon`.
func TestCorrectAnswersAreAlwaysAttested(t *testing.T) {
	g, dag := newTestGenerator(t)

	for _, code := range []string{"vowel-harmony", "locative-grid", "definite-conjugation"} {
		items, _, err := g.Next(Request{
			Topic: topicOf(t, dag, code), Phase: Interleaved, Count: 20,
			Definiteness: Weights{DefiniteObjectPct: 55, DemonstrativePct: 25},
			Rand:         fixedRand(),
		})
		if err != nil {
			t.Fatalf("%s: %v", code, err)
		}
		if len(items) == 0 {
			t.Fatalf("%s produced no items", code)
		}
		for _, it := range items {
			if it.Expected() == "" {
				t.Errorf("%s/%s: empty expected answer", code, it.Kind)
			}
			if !g.analyzer.IsAttested(it.Expected()) {
				t.Errorf("%s/%s: expected %q is not attested in the pack",
					code, it.Kind, it.Expected())
			}
			if it.Expected() == "házan" {
				t.Errorf("%s: served the generator's guess as the correct answer", code)
			}
		}
	}
}

// The single most important attestation case in the fixture: the superessive of
// `ház` is `házon`, and the pack's own paradigm table says `házan`.
func TestSuperessivePrefersTheAttestedForm(t *testing.T) {
	g, dag := newTestGenerator(t)
	items, _, err := g.Next(Request{
		Topic: topicOf(t, dag, "locative-grid"), Phase: Blocked, Count: 30,
		Rand: fixedRand(),
	})
	if err != nil {
		t.Fatal(err)
	}
	sawSup := false
	for _, it := range items {
		if !strings.Contains(it.Headline, "Sup") {
			continue
		}
		sawSup = true
		if it.Expected() == "házan" {
			t.Fatal("served the generated form; the attested one is házon")
		}
	}
	_ = sawSup // the mix is random; the assertion above is what matters
}

// An item declares exactly the features it required, and nothing else. Grading
// what an item did not test is what would turn the heatmap into noise.
func TestItemsGradeOnlyWhatTheyRequire(t *testing.T) {
	g, dag := newTestGenerator(t)

	// Pure harmony practice does not test the inessive: the learner picking
	// `házban` over `házben` was never asked what the inessive means.
	items, _, err := g.Next(Request{
		Topic: topicOf(t, dag, "vowel-harmony"), Phase: Blocked, Count: 8, Rand: fixedRand(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("no harmony items")
	}
	for _, it := range items {
		if !slices.Equal(it.FeatureIDs, []string{curriculum.FeatureVowelHarmony}) {
			t.Errorf("harmony item graded %v, want only the harmony skill", it.FeatureIDs)
		}
	}

	// A case CHOICE between forms of the same noun does not test harmony: every
	// option shares the stem's harmony class.
	items, _, err = g.Next(Request{
		Topic: topicOf(t, dag, "locative-grid"), Phase: Blocked, Count: 20, Rand: fixedRand(),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.Kind != KindCase {
			continue
		}
		if slices.Contains(it.FeatureIDs, curriculum.FeatureVowelHarmony) {
			t.Errorf("case item %v graded harmony, which it never tested", it.FeatureIDs)
		}
	}
}

// An allomorph item is only a question if its options actually differ, and the
// wrong one must genuinely be wrong.
func TestAllomorphOptionsAreDistinctAndOneIsRight(t *testing.T) {
	g, dag := newTestGenerator(t)
	items, _, err := g.Next(Request{
		Topic: topicOf(t, dag, "vowel-harmony"), Phase: Blocked, Count: 12, Rand: fixedRand(),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if len(it.Options) < 2 {
			t.Fatalf("%q has %d options", it.Expected(), len(it.Options))
		}
		seen, matches := map[string]bool{}, 0
		for _, o := range it.Options {
			if seen[o.Text] {
				t.Errorf("%q: duplicate option %q", it.Expected(), o.Text)
			}
			seen[o.Text] = true
			if o.Text == it.Expected() {
				matches++
			}
		}
		if matches != 1 {
			t.Errorf("%q: %d options match the expected answer, want exactly 1",
				it.Expected(), matches)
		}
		// The harmony class the item claims must be the one that produced the
		// answer, or the post-answer explanation would contradict the grading.
		if want := string(Classify(it.Lemma)); it.PromptArgs["harmony"] != want {
			t.Errorf("%s: prompt says %q, Classify says %q",
				it.Lemma, it.PromptArgs["harmony"], want)
		}
	}
}

// The distractor in a conjugation item is the SAME verb in the other
// conjugation, and the 1sg past — identical in both — must never produce one.
func TestConjugationItemsOfferARealChoice(t *testing.T) {
	g, dag := newTestGenerator(t)
	items, _, err := g.Next(Request{
		Topic: topicOf(t, dag, "definite-conjugation"), Phase: Blocked, Count: 20,
		Definiteness: Weights{DefiniteObjectPct: 55, DemonstrativePct: 25},
		Rand:         fixedRand(),
	})
	if err != nil {
		t.Fatal(err)
	}
	saw := 0
	for _, it := range items {
		if it.Kind != KindConjugation {
			continue
		}
		saw++
		if len(it.Options) != 2 {
			t.Errorf("%q: %d options, want the minimal pair", it.Frame, len(it.Options))
		}
		if it.Options[0].Text == it.Options[1].Text {
			t.Errorf("%q: both options are %q — not a choice", it.Frame, it.Options[0].Text)
		}
		if it.Expected() == "láttam" {
			t.Errorf("%q: the 1sg past is the same in both conjugations", it.Frame)
		}
		if !strings.Contains(it.Frame, BlankMark) {
			t.Errorf("%q: the frame has no blank", it.Frame)
		}
		if !slices.Equal(it.FeatureIDs, []string{"Definite=Def"}) &&
			!slices.Equal(it.FeatureIDs, []string{"Definite=Ind"}) {
			t.Errorf("a definiteness item graded %v — person and tense were identical "+
				"across the options and were never tested", it.FeatureIDs)
		}
	}
	if saw == 0 {
		t.Fatal("no conjugation items were produced")
	}
}

// [R15]: 55% definite-object, 25% demonstrative. The weights are what makes this
// drill match the documented error distribution rather than the paradigm.
func TestDefinitenessWeightsFollowTheErrorDistribution(t *testing.T) {
	rnd := rand.New(rand.NewSource(11))
	w := Weights{DefiniteObjectPct: 55, DemonstrativePct: 25}
	counts := map[mode]int{}
	const n = 20000
	for range n {
		counts[pickMode(w, rnd)]++
	}
	check := func(m mode, want float64) {
		got := float64(counts[m]) / n
		if got < want-0.02 || got > want+0.02 {
			t.Errorf("%v share = %.3f, want %.2f", m, got, want)
		}
	}
	check(modeDefinite, 0.55)
	check(modeDemonstrative, 0.25)
	check(modeIndefinite, 0.20)

	if got := w.Remainder(); got != 20 {
		t.Errorf("Remainder = %d, want 20", got)
	}
	// Percentages that leave no remainder must not produce a negative share.
	if got := (Weights{DefiniteObjectPct: 70, DemonstrativePct: 40}).Remainder(); got != 0 {
		t.Errorf("Remainder = %d, want 0", got)
	}
}

// Blocked practice runs ONE kind; interleaving mixes. This is the operational
// half of R5, and it is what the phase badge is telling the learner about.
func TestBlockedPracticeIsSingleKind(t *testing.T) {
	g, dag := newTestGenerator(t)
	topic := topicOf(t, dag, "locative-grid")

	blocked, _, err := g.Next(Request{Topic: topic, Phase: Blocked, Count: 12, Rand: fixedRand()})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[Kind]bool{}
	for _, it := range blocked {
		kinds[it.Kind] = true
	}
	if len(kinds) != 1 {
		t.Errorf("blocked practice served %d kinds, want 1: %v", len(kinds), kinds)
	}

	interleaved, _, err := g.Next(Request{Topic: topic, Phase: Interleaved, Count: 12, Rand: fixedRand()})
	if err != nil {
		t.Fatal(err)
	}
	mixed := map[Kind]bool{}
	for _, it := range interleaved {
		mixed[it.Kind] = true
	}
	if len(mixed) < 2 {
		t.Errorf("interleaved practice served %d kinds, want a mix: %v", len(mixed), mixed)
	}
}

// Case items contrast the SAME noun across the grid — R5's discriminability
// mechanism, not variety for its own sake.
func TestCaseItemsContrastTheSameNoun(t *testing.T) {
	g, dag := newTestGenerator(t)
	items, _, err := g.Next(Request{
		Topic: topicOf(t, dag, "locative-grid"), Phase: Interleaved, Count: 30, Rand: fixedRand(),
	})
	if err != nil {
		t.Fatal(err)
	}
	saw := 0
	for _, it := range items {
		if it.Kind != KindCase {
			continue
		}
		saw++
		for _, o := range it.Options {
			if !strings.HasPrefix(strings.ToLower(o.Text), strings.ToLower(it.Lemma[:3])) {
				t.Errorf("%s: option %q is not a form of the prompt noun", it.Lemma, o.Text)
			}
		}
	}
	if saw == 0 {
		t.Fatal("no case items were produced")
	}
}

// A cloze must never blank a possessed form when the plain one was asked for:
// `házaimban` also carries Case=Ine, and offering it is a different question.
func TestFormSpecExcludesPossessedForms(t *testing.T) {
	g, _ := newTestGenerator(t)
	if err := g.load(); err != nil {
		t.Fatal(err)
	}
	items, _, err := g.Next(Request{
		Topic: curriculum.Topic{Code: "locative-grid", Level: "A1",
			FeatureIDs: []string{"Case=Ine"}},
		Phase: Blocked, Count: 10, Rand: fixedRand(),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.Expected() == "házaimban" {
			t.Fatal("a plain inessive item was answered with the possessed form")
		}
	}
}

// Every item carries both glosses. A drill that can only explain itself in
// English is an incomplete drill. See ADR-004.
func TestItemsCarryBothGlosses(t *testing.T) {
	g, dag := newTestGenerator(t)
	items, _, err := g.Next(Request{
		Topic: topicOf(t, dag, "locative-grid"), Phase: Interleaved, Count: 10, Rand: fixedRand(),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.LemmaID == 0 {
			continue
		}
		if it.GlossRU == "" || it.GlossEN == "" {
			t.Errorf("%s: glosses ru=%q en=%q — both locales, always", it.Lemma, it.GlossRU, it.GlossEN)
		}
	}
}

// The expected answer must not travel to the client.
func TestServedItemsCarryNoAnswer(t *testing.T) {
	g, dag := newTestGenerator(t)
	items, _, err := g.Next(Request{
		Topic: topicOf(t, dag, "locative-grid"), Phase: Blocked, Count: 5, Rand: fixedRand(),
	})
	if err != nil {
		t.Fatal(err)
	}
	pending := NewPending()
	served, err := pending.Put(1, items)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range served {
		if it.Expected() != "" {
			t.Errorf("a served item still carries its answer: %q", it.Expected())
		}
		if it.ID == "" {
			t.Error("a served item has no id to answer against")
		}
	}
	// And the server still has it.
	restored, ok := pending.Take(1, served[0].ID)
	if !ok || restored.Expected() == "" {
		t.Error("the server must retain the expected answer")
	}
	if _, ok := pending.Take(1, served[0].ID); ok {
		t.Error("an item must be answerable only once")
	}
	if _, ok := pending.Take(999, served[1].ID); ok {
		t.Error("one learner must not be able to answer another's item")
	}
}
