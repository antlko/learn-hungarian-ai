package drill

import (
	"path/filepath"
	"time"

	"math/rand"
	"slices"
	"strings"
	"tanulom/internal/db"
	"tanulom/internal/srs"
	"testing"

	"tanulom/internal/morph"
)

func seededRand(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }

// loaded returns the generator with its catalogue and vocabulary read, which
// Next does on first use and the direct item builders assume.
func loaded(t *testing.T, g *Generator) *Generator {
	t.Helper()
	if err := g.load(); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestPersonKindIsOfferedOnlyForVerbTopics(t *testing.T) {
	g, dag := newTestGenerator(t)
	for _, code := range []string{"present-indefinite", "past-tense"} {
		if a := g.KindsFor(topicOf(t, dag, code)); !slices.Contains(a.Kinds, KindPerson) {
			t.Errorf("%s: person missing from %v", code, a.Kinds)
		}
	}
	for _, code := range []string{"accusative", "locative-grid", "vowel-harmony"} {
		if a := g.KindsFor(topicOf(t, dag, code)); slices.Contains(a.Kinds, KindPerson) {
			t.Errorf("%s is not a verb topic but offers person", code)
		}
	}
	// The topic that had no drills at all now has one.
	if av := g.Availability(topicOf(t, dag, "present-indefinite")); !av.Available {
		t.Errorf("present-indefinite must be drillable through person, got %+v", av)
	}
}

func TestPersonItems(t *testing.T) {
	g, dag := newTestGenerator(t)
	loaded(t, g)
	allowed := map[string][]string{
		// Before the definite conjugation: no Definite=Def-only tokens (látom,
		// nézem, nézte), no generated forms (látunk).
		"present-indefinite": {"látok", "nézel", "néznek", "nézünk", "nézek"},
		"past-tense":         {"láttál", "látott", "néztél"},
	}
	for code, want := range allowed {
		topic := topicOf(t, dag, code)
		got := map[string]bool{}
		for seed := int64(1); seed <= 80; seed++ {
			it, ok := g.personItem(Request{Topic: topic, Phase: Blocked, Count: 1, Rand: seededRand(seed)})
			if !ok {
				continue
			}
			got[strings.ToLower(it.Expected())] = true
			if it.Kind != KindPerson || !it.Kind.Typed() || len(it.Options) != 0 {
				t.Errorf("%s: a person item is typed, got %+v", code, it)
			}
			if !g.analyzer.IsAttested(it.Expected()) {
				t.Errorf("%s: %q is not attested", code, it.Expected())
			}
			if !strings.Contains(it.Frame, BlankMark) || strings.Contains(it.Frame, it.Expected()) {
				t.Errorf("%s: frame %q must blank the verb %q", code, it.Frame, it.Expected())
			}
			if _, ok := it.PromptArgs["lemma"]; !ok {
				t.Errorf("%s: no lemma arg", code)
			}
			pronoun := it.PromptArgs["pronoun"]
			known := false
			for _, p := range personPronouns {
				known = known || p == pronoun
			}
			if !known {
				t.Errorf("%s: pronoun %q is not in the closed table", code, pronoun)
			}
			for _, id := range it.FeatureIDs {
				if !slices.Contains(topic.FeatureIDs, id) {
					t.Errorf("%s: grades %q which the topic does not teach", code, id)
				}
				if strings.HasPrefix(id, "Person") || strings.HasPrefix(id, "Number") {
					t.Errorf("%s: person and number must never be graded, got %q", code, id)
				}
			}
			if len(it.FeatureIDs) == 0 {
				t.Errorf("%s: grades nothing", code)
			}
		}
		if len(got) == 0 {
			t.Fatalf("%s produced no person items", code)
		}
		for form := range got {
			if !slices.Contains(want, form) {
				t.Errorf("%s: served %q, which is not an allowed token (%v)", code, form, want)
			}
		}
	}
}

// Once the definite conjugation is taught its tokens are fair game, and the
// topic that teaches the contrast needs the token to say which side it is on.
func TestPersonDefiniteTokensWaitForTheDefiniteTopic(t *testing.T) {
	g, dag := newTestGenerator(t)
	loaded(t, g)
	topic := topicOf(t, dag, "definite-conjugation")
	if !g.personTopic(topic) {
		t.Fatal("definite-conjugation is a verb topic")
	}
	sawDef := false
	for seed := int64(1); seed <= 80; seed++ {
		it, ok := g.personItem(Request{Topic: topic, Phase: Blocked, Count: 1, Rand: seededRand(seed)})
		if ok && (slices.Contains([]string{"látom", "nézem", "látja", "nézte"}, strings.ToLower(it.Expected()))) {
			sawDef = true
		}
	}
	if !sawDef {
		t.Error("a Definite=Def token should be allowed from the definite topic on")
	}
}

func TestPersonGradingNamesTheRightMistake(t *testing.T) {
	g, _ := newTestGenerator(t)
	item := Item{
		Kind: KindPerson, LemmaID: 3, Lemma: "lát", answerPerson: "1|Sing",
		FeatureIDs: []string{"Definite=Ind", "Tense=Pres"},
	}.WithExpected("látok")

	cases := []struct {
		typed     string
		wantClass morph.ErrorClass
		keepsFeat bool
	}{
		{"látsz", ErrPerson, true},      // 2sg of the same verb: wrong person, right features
		{"látott", ErrPerson, false},    // another person AND another tense
		{"lát", ErrPerson, true},        // bare lemma = 3sg present indefinite: same features, another person
		{"látok", morph.NoError, false}, // right
		{"xyzzy", morph.ErrStem, false}, // not a form of the verb at all
	}
	for _, c := range cases {
		d := morph.DiffForms(c.typed, item.Expected(), g.analyzer)
		d, keeps := classifyPerson(d, item, c.typed, g.analyzer)
		if c.wantClass == morph.ErrStem {
			if d.Class == ErrPerson {
				t.Errorf("%q: not a form of the verb, must not be a person error", c.typed)
			}
			continue
		}
		if d.Class != c.wantClass {
			t.Errorf("%q: class = %q, want %q", c.typed, d.Class, c.wantClass)
		}
		if keeps != c.keepsFeat {
			t.Errorf("%q: features kept = %v, want %v", c.typed, keeps, c.keepsFeat)
		}
		if c.wantClass == ErrPerson && (d.WrongFeature != "" || d.NearMatch || d.Hint != "diff.person") {
			t.Errorf("%q: a person error blames no feature and is no typo: %+v", c.typed, d)
		}
	}
}

func TestQwordTable(t *testing.T) {
	for _, c := range []string{"Ine", "Sup", "Ade"} {
		if qwordFor[c] != "hol" {
			t.Errorf("%s -> %q", c, qwordFor[c])
		}
	}
	for _, c := range []string{"Ill", "Sbl", "All"} {
		if qwordFor[c] != "hova" {
			t.Errorf("%s -> %q", c, qwordFor[c])
		}
	}
	for _, c := range []string{"Ela", "Del", "Abl"} {
		if qwordFor[c] != "honnan" {
			t.Errorf("%s -> %q", c, qwordFor[c])
		}
	}
	if qwordFor["Acc"] != "mit/kit" || qwordFor["Dat"] != "kinek" || qwordFor["Ins"] != "mivel/kivel" {
		t.Error("the accusative, dative and instrumental words are wrong")
	}
	for word, opts := range qwordOptions {
		if len(opts) != 3 || !slices.Contains(opts, word) {
			t.Errorf("%s: options %v must be three and include the answer", word, opts)
		}
	}
}

func TestQwordItems(t *testing.T) {
	g, dag := newTestGenerator(t)
	loaded(t, g)
	for _, code := range []string{"locative-grid", "accusative"} {
		topic := topicOf(t, dag, code)
		if a := g.KindsFor(topic); !slices.Contains(a.Kinds, KindQword) {
			t.Fatalf("%s must offer qword, got %v", code, a.Kinds)
		}
		n := 0
		for seed := int64(1); seed <= 60; seed++ {
			it, ok := g.qwordItem(Request{Topic: topic, Phase: Blocked, Count: 1, Rand: seededRand(seed)})
			if !ok {
				continue
			}
			n++
			if it.Kind.Typed() || len(it.Options) != 3 {
				t.Errorf("%s: qword is a three-way choice, got %d options", code, len(it.Options))
			}
			focus := 0
			var joined, focusText string
			for _, p := range it.Parts {
				joined += p.Text
				if p.Focus {
					focus++
					focusText = p.Text
				}
			}
			if focus != 1 {
				t.Errorf("%s: %d focused parts, want 1", code, focus)
			}
			var hu string
			if err := g.pack.DB().QueryRow(`SELECT hu FROM sentence WHERE id = ?`, it.SentenceID).Scan(&hu); err != nil {
				t.Fatal(err)
			}
			if joined != hu {
				t.Errorf("%s: parts %q do not rebuild the sentence %q", code, joined, hu)
			}
			if len(it.FeatureIDs) != 1 || !slices.Contains(topic.FeatureIDs, it.FeatureIDs[0]) {
				t.Errorf("%s: feature ids %v are not the topic's", code, it.FeatureIDs)
			}
			f := g.features[it.FeatureIDs[0]]
			if it.Expected() != qwordFor[f.Value] {
				t.Errorf("%s: expected %q for %s", code, it.Expected(), f.ID)
			}
			if !g.caseReading(focusText, it.LemmaID, f) {
				t.Errorf("%s: %q is not an unambiguous %s", code, focusText, f.ID)
			}
			// No generated reading may carry the case.
			all, _ := g.analyzer.Lookup(focusText)
			attestedReading := false
			for _, a := range all {
				if a.LemmaID == it.LemmaID && a.Source != "generated" && a.Feats["Case"] == f.Value {
					attestedReading = true
				}
			}
			if !attestedReading {
				t.Errorf("%s: %q has no attested reading as %s", code, focusText, f.ID)
			}
			if it.Frame != "" {
				t.Errorf("%s: qword ships Parts, not a Frame", code)
			}
		}
		if n == 0 {
			t.Errorf("%s: no qword items", code)
		}
	}
}

func TestQwordErrorClasses(t *testing.T) {
	cases := []struct {
		expected, chosen string
		want             morph.ErrorClass
	}{
		{"hol", "hova", ErrDirection},
		{"hova", "hol", ErrDirection},
		{"hol", "honnan", ErrSource},
		{"honnan", "hova", ErrSource},
		{"hol", "mit/kit", morph.ErrCase},
		{"mit/kit", "hol", morph.ErrCase},
	}
	for _, c := range cases {
		if got := classifyQword(c.expected, c.chosen); got != c.want {
			t.Errorf("%s vs %s: %q, want %q", c.expected, c.chosen, got, c.want)
		}
	}
}

// The engine itself: a wrong-person answer is "no evidence" for the features
// (nothing graded) yet still an incorrect attempt for the gate; a wrong-tense
// answer is graded as usual.
func TestEnginePersonSlip(t *testing.T) {
	g, dag := newTestGenerator(t)
	loaded(t, g)
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	uid, err := d.CreateUser("a@example.com", "h", db.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(NewStore(d.DB), srs.NewEngine(srs.NewStore(d.DB), 0.9, 20), dag, g.analyzer, GateConfig{})
	topic := topicOf(t, dag, "present-indefinite")
	mk := func() Item {
		return Item{
			TopicCode: topic.Code, Kind: KindPerson, Phase: Blocked, Lemma: "lát", LemmaID: 3,
			PromptKey: "drill.prompt.person", Frame: "___ egy házat az utcán.",
			FeatureIDs: []string{"Definite=Ind", "Tense=Pres"}, Headline: "Definite=Ind",
			answerPerson: "1|Sing",
		}.WithExpected("Látok")
	}
	feats := func() int {
		var n int
		if err := d.QueryRow(`SELECT COUNT(*) FROM feature_review`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	now := time.Now()

	res, err := eng.Answer(AnswerInput{UserID: uid, Item: mk(), Answer: "látsz"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.Correct || res.Diff == nil || res.Diff.Class != ErrPerson || res.Diff.Hint != "diff.person" {
		t.Errorf("wrong person: want an incorrect answer classed person, got %+v", res.Diff)
	}
	if len(res.GradedFeatures) != 0 || feats() != 0 {
		t.Errorf("a person slip is no evidence: graded %v, feature_review rows %d", res.GradedFeatures, feats())
	}
	if res.Progress.Attempts != 1 || res.Progress.Correct != 0 || res.Progress.BlockedTotal != 1 {
		t.Errorf("the attempt must still count as incorrect for the gate: %+v", res.Progress)
	}
	var class string
	if err := d.QueryRow(`SELECT error_class FROM drill_attempt ORDER BY id DESC LIMIT 1`).Scan(&class); err != nil || class != "person" {
		t.Errorf("logged class = %q (%v)", class, err)
	}

	// Another tense AND person: the features are not shown to be held, so they
	// are graded normally.
	res, err = eng.Answer(AnswerInput{UserID: uid, Item: mk(), Answer: "látott"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.Correct || len(res.GradedFeatures) == 0 || feats() == 0 {
		t.Errorf("a wrong-tense answer must grade normally: %v", res.GradedFeatures)
	}
}

// Content quality: a token that is also a form of another verb, or a subjunctive
// in a topic that teaches the indicative, is never a person item.
func TestPersonReadingRefusesDubiousTokens(t *testing.T) {
	g, dag := newTestGenerator(t)
	loaded(t, g)
	vf := g.personFeatures(topicOf(t, dag, "present-indefinite"))

	if _, ok := g.personReading("jelennek", 8, vf, false); ok {
		t.Error("jelennek is also a form of jelenik: the corpus lemma cannot be trusted")
	}
	if _, ok := g.personReading("maradjak", 10, vf, false); ok {
		t.Error("a subjunctive is outside what present-indefinite teaches")
	}
	if _, ok := g.personReading("marad", 10, vf, false); ok {
		t.Error("`marad` is also read as subjunctive: ambiguous")
	}
	if _, ok := g.personReading("maradok", 10, vf, false); !ok {
		t.Error("a plain indicative form must still qualify")
	}
}

// The same (verb, sentence) is one question whichever feature the draw aimed at,
// so a frame can never appear twice under two keys.
func TestPersonKeyIsStablePerSentence(t *testing.T) {
	g, dag := newTestGenerator(t)
	loaded(t, g)
	topic := topicOf(t, dag, "present-indefinite")
	byFrame := map[string]string{}
	for seed := int64(1); seed <= 120; seed++ {
		it, ok := g.personItem(Request{Topic: topic, Phase: Blocked, Count: 1, Rand: seededRand(seed)})
		if !ok {
			continue
		}
		if k, seen := byFrame[it.Frame]; seen && k != it.Key() {
			t.Fatalf("frame %q has two keys: %q and %q", it.Frame, k, it.Key())
		}
		byFrame[it.Frame] = it.Key()
		if it.SentenceID == 0 {
			t.Error("a person item must record its corpus sentence")
		}
	}
}

// The bare lemma typed against a past or a definite item is the 3sg present
// indefinite: another person, but NOT the item's tense or conjugation, so the
// features are graded normally rather than waved through as a no-evidence slip.
func TestPersonBareLemmaAgainstPastAndDefiniteItems(t *testing.T) {
	g, _ := newTestGenerator(t)
	loaded(t, g)
	for _, c := range []struct {
		name  string
		feats []string
	}{{"past", []string{"Tense=Past"}}, {"definite", []string{"Definite=Def"}}} {
		item := Item{Kind: KindPerson, LemmaID: 3, Lemma: "lát", answerPerson: "1|Sing",
			FeatureIDs: c.feats}.WithExpected("láttam")
		d := morph.DiffForms("lát", item.Expected(), g.analyzer)
		d, keeps := classifyPerson(d, item, "lát", g.analyzer)
		if keeps {
			t.Errorf("%s: a form lacking the item's feature must be graded normally", c.name)
		}
		if d.Class != ErrPerson {
			t.Errorf("%s: still another person, class %q", c.name, d.Class)
		}
	}
}

// One reading carrying the features and another not: the features are graded.
func TestPersonSlipIsOrderIndependent(t *testing.T) {
	g, _ := newTestGenerator(t)
	loaded(t, g)
	// `láttam` as 1sg Def and Ind past: asking for 2sg past, both readings are
	// another person; both carry Tense=Past, so features are kept either way.
	item := Item{Kind: KindPerson, LemmaID: 3, Lemma: "lát", answerPerson: "2|Sing",
		FeatureIDs: []string{"Tense=Past"}}.WithExpected("láttál")
	d := morph.DiffForms("láttam", item.Expected(), g.analyzer)
	if _, keeps := classifyPerson(d, item, "láttam", g.analyzer); !keeps {
		t.Error("every other-person reading carries Tense=Past: features are kept")
	}
	item.FeatureIDs = []string{"Tense=Pres"}
	if _, keeps := classifyPerson(d, item, "láttam", g.analyzer); keeps {
		t.Error("no reading carries Tense=Pres: graded normally")
	}
}
