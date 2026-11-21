package drill

import (
	"math/rand"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"tanulom/internal/db"
	"tanulom/internal/srs"
)

// checkMatchInvariants asserts what every match item must satisfy, whatever the
// pack: 3-5 cues, one tile per cue, distinct forms, each an attested form of
// the lemma, features inside the topic and never Person/Number.
func checkMatchInvariants(t *testing.T, g *Generator, where string, topicFeats []string, it Item) {
	t.Helper()
	if it.Kind != KindMatch || !it.Kind.MultiPart() || it.Kind.Typed() {
		t.Errorf("%s: wrong kind %s", where, it.Kind)
	}
	if n := len(it.Columns); n < matchMinPairs || n > matchMaxPairs || len(it.Tiles) != n {
		t.Fatalf("%s: %d columns, %d tiles", where, len(it.Columns), len(it.Tiles))
	}
	cols := map[string]bool{}
	for _, c := range it.Columns {
		if cols[c] || c == "" {
			t.Errorf("%s: duplicate or empty cue %q", where, c)
		}
		cols[c] = true
	}
	want := expectedParts(it.Expected())
	forms, _ := g.analyzer.FormsOf(it.LemmaID)
	seenText, seenCol := map[string]bool{}, map[string]bool{}
	for _, tl := range it.Tiles {
		low := strings.ToLower(tl.Text)
		if seenText[low] {
			t.Errorf("%s: duplicate form %q", where, tl.Text)
		}
		seenText[low] = true
		col, ok := want[tl.ID]
		if !ok || !cols[col] || seenCol[col] {
			t.Errorf("%s: tile %s -> %q is not a bijection", where, tl.ID, col)
		}
		seenCol[col] = true
		attested := false
		for _, f := range forms {
			if f.Attested() && strings.EqualFold(f.Form, tl.Text) {
				attested = true
			}
		}
		if !attested {
			t.Errorf("%s: %q is not an attested form of %s", where, tl.Text, it.Lemma)
		}
		if len(it.pairFeats[tl.ID]) == 0 {
			t.Errorf("%s: tile %s carries no feature", where, tl.ID)
		}
	}
	if len(it.FeatureIDs) == 0 {
		t.Errorf("%s: grades nothing", where)
	}
	for _, id := range it.FeatureIDs {
		if !slices.Contains(topicFeats, id) {
			t.Errorf("%s: grades %q, not a feature of the topic", where, id)
		}
		if strings.HasPrefix(id, "Person") || strings.HasPrefix(id, "Number") {
			t.Errorf("%s: person/number must never be graded: %q", where, id)
		}
	}
	if len(it.Options) != 0 || it.Frame != "" {
		t.Errorf("%s: a match ships columns and tiles only", where)
	}
}

func TestMatchItems(t *testing.T) {
	g, dag := newTestGenerator(t)
	loaded(t, g)
	for _, code := range []string{"present-indefinite", "past-tense", "locative-grid"} {
		topic := topicOf(t, dag, code)
		if a := g.KindsFor(topic); !slices.Contains(a.Kinds, KindMatch) {
			t.Fatalf("%s must offer match, got %v", code, a.Kinds)
		}
		n := 0
		keys := map[string]bool{}
		for seed := int64(1); seed <= 40; seed++ {
			it, ok := g.matchItem(Request{Topic: topic, Phase: Blocked, Count: 1, Rand: seededRand(seed)})
			if !ok {
				continue
			}
			n++
			checkMatchInvariants(t, g, code, topic.FeatureIDs, it)
			keys[it.Key()] = true
		}
		if n == 0 {
			t.Errorf("%s: no match items", code)
		}
		if !strings.HasPrefix(firstKey(keys), "match|") {
			t.Errorf("%s: key %q", code, firstKey(keys))
		}
	}
	// A topic with a single case (no question-word contrast) has no match.
	if a := g.KindsFor(topicOf(t, dag, "accusative")); slices.Contains(a.Kinds, KindMatch) {
		t.Error("accusative has one case: nothing to match")
	}
}

func firstKey(m map[string]bool) string {
	for k := range m {
		return k
	}
	return ""
}

func TestMatchCheckParts(t *testing.T) {
	g, dag := newTestGenerator(t)
	loaded(t, g)
	it, ok := g.matchItem(Request{Topic: topicOf(t, dag, "present-indefinite"), Phase: Blocked, Count: 1, Rand: seededRand(3)})
	if !ok {
		t.Fatal("no item")
	}
	good := expectedParts(it.Expected())
	if err := it.CheckParts(good); err != nil {
		t.Fatalf("the right answer is refused: %v", err)
	}
	missing := map[string]string{}
	for k, v := range good {
		missing[k] = v
	}
	delete(missing, it.Tiles[0].ID)
	extra := map[string]string{"zz": it.Columns[0]}
	for k, v := range good {
		extra[k] = v
	}
	twice := map[string]string{}
	for k, v := range good {
		twice[k] = v
	}
	twice[it.Tiles[0].ID] = twice[it.Tiles[1].ID] // two tiles under one cue
	badCue := map[string]string{}
	for k, v := range good {
		badCue[k] = v
	}
	badCue[it.Tiles[0].ID] = "nobody"
	for name, parts := range map[string]map[string]string{"missing": missing, "extra": extra, "twice": twice, "cue": badCue} {
		if err := it.CheckParts(parts); err == nil {
			t.Errorf("%s: a malformed match answer must be refused", name)
		}
	}
}

// One review per DISTINCT feature; a wrong pair puts exactly the features it
// carries at Again and leaves the others; person and number are never graded.
func TestEngineMatchGrading(t *testing.T) {
	g, dag := newTestGenerator(t)
	loaded(t, g)
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	uid, _ := d.CreateUser("a@example.com", "h", db.RoleAdmin)
	eng := NewEngine(NewStore(d.DB), srs.NewEngine(srs.NewStore(d.DB), 0.9, 20), dag, g.analyzer, GateConfig{})
	reviews := func() int {
		var n int
		d.QueryRow(`SELECT COUNT(*) FROM feature_review`).Scan(&n)
		return n
	}

	// A case match: each pair carries its own case.
	loc := topicOf(t, dag, "locative-grid")
	it, ok := g.matchItem(Request{Topic: loc, Phase: Blocked, Count: 1, Rand: seededRand(5)})
	if !ok {
		t.Fatal("no case match")
	}
	good := expectedParts(it.Expected())
	res, err := eng.Answer(AnswerInput{UserID: uid, Item: it, Parts: good}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Correct || len(res.GradedFeatures) != len(it.FeatureIDs) || reviews() != len(it.FeatureIDs) {
		t.Fatalf("a right match grades each distinct feature once: %v, %d reviews", res.GradedFeatures, reviews())
	}
	for _, gr := range res.GradedFeatures {
		if !gr.Correct || gr.Rating > srs.Good {
			t.Errorf("%s: right pair graded %+v (a tap is at most Good)", gr.FeatureID, gr)
		}
	}
	if len(res.PartResults) != len(it.Tiles) || res.Expected != "" {
		t.Errorf("partResults like sort, and no expected string: %+v", res)
	}

	// Swap two forms: both cases are wrong, the third is not.
	wrong := map[string]string{}
	for k, v := range good {
		wrong[k] = v
	}
	a, b := it.Tiles[0].ID, it.Tiles[1].ID
	wrong[a], wrong[b] = good[b], good[a]
	before := reviews()
	res, err = eng.Answer(AnswerInput{UserID: uid, Item: it, Parts: wrong}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if res.Correct || reviews()-before != len(it.FeatureIDs) {
		t.Fatalf("one review per feature, got %d new", reviews()-before)
	}
	bad := map[string]bool{}
	for _, id := range []string{a, b} {
		for _, f := range it.pairFeats[id] {
			bad[f] = true
		}
	}
	for _, gr := range res.GradedFeatures {
		if gr.Correct == bad[gr.FeatureID] {
			t.Errorf("%s: correct=%v but wrong pair carries it=%v", gr.FeatureID, gr.Correct, bad[gr.FeatureID])
		}
	}
	var class string
	d.QueryRow(`SELECT error_class FROM drill_attempt ORDER BY id DESC LIMIT 1`).Scan(&class)
	if class != "case" {
		t.Errorf("a wrong case match is class case, got %q", class)
	}

	// A verb match: every pair carries the same features; one wrong pair = one
	// Again per feature, class person, no Person/Number review.
	pi := topicOf(t, dag, "present-indefinite")
	vit, ok := g.matchItem(Request{Topic: pi, Phase: Blocked, Count: 1, Rand: seededRand(9)})
	if !ok {
		t.Fatal("no verb match")
	}
	vgood := expectedParts(vit.Expected())
	vwrong := map[string]string{}
	for k, v := range vgood {
		vwrong[k] = v
	}
	x, y := vit.Tiles[0].ID, vit.Tiles[1].ID
	vwrong[x], vwrong[y] = vgood[y], vgood[x]
	before = reviews()
	res, err = eng.Answer(AnswerInput{UserID: uid, Item: vit, Parts: vwrong}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got := reviews() - before; got != len(vit.FeatureIDs) {
		t.Errorf("%d reviews, want one per feature (%d)", got, len(vit.FeatureIDs))
	}
	for _, gr := range res.GradedFeatures {
		if gr.Correct || gr.Rating != srs.Again {
			t.Errorf("%s: want Again, got %+v", gr.FeatureID, gr)
		}
		if strings.HasPrefix(gr.FeatureID, "Person") || strings.HasPrefix(gr.FeatureID, "Number") {
			t.Errorf("person/number graded: %s", gr.FeatureID)
		}
	}
	d.QueryRow(`SELECT error_class FROM drill_attempt ORDER BY id DESC LIMIT 1`).Scan(&class)
	if class != "person" {
		t.Errorf("a wrong verb match is class person, got %q", class)
	}
}

// Anti-repeat: the key names the lemma and the cues, however tiles shuffled.
func TestMatchKeyAndAntiRepeat(t *testing.T) {
	g, dag := newTestGenerator(t)
	loaded(t, g)
	topic := topicOf(t, dag, "present-indefinite")
	a, _ := g.matchItem(Request{Topic: topic, Phase: Blocked, Count: 1, Rand: seededRand(1)})
	b := a
	b.Tiles = slices.Clone(a.Tiles)
	rand.New(rand.NewSource(4)).Shuffle(len(b.Tiles), func(i, j int) { b.Tiles[i], b.Tiles[j] = b.Tiles[j], b.Tiles[i] })
	if a.Key() != b.Key() {
		t.Errorf("tile order changed the key: %q vs %q", a.Key(), b.Key())
	}
	if lemmaOfKey(a.Key()) != a.LemmaID {
		t.Errorf("lemma not recoverable from %q", a.Key())
	}
	r := NewRecent()
	r.AddItem(a)
	if stageOf(a, r, nil, nil) != 3 {
		t.Error("an exact repeat is stage 3")
	}
	// Placement never gets a match.
	items, _, _ := g.Next(Request{Topic: topic, Phase: Blocked, Count: 6, Origin: "placement", Rand: seededRand(2)})
	for _, it := range items {
		if it.Kind == KindMatch {
			t.Error("placement must not serve match")
		}
	}
}

// Bare person items: sentence-less, attested, one string per cell.
func TestBarePersonOnTheFixture(t *testing.T) {
	g, dag := newTestGenerator(t)
	loaded(t, g)
	topic := topicOf(t, dag, "present-indefinite")
	vf := g.personFeatures(topic)
	refs := g.bareRefs(topic, vf)
	if len(refs) == 0 {
		t.Fatal("no bare cells")
	}
	for _, r := range refs {
		if r.form == "látunk" {
			t.Error("a generated form became a bare cell")
		}
	}
	it, ok := g.randomBare(Request{Topic: topic, Phase: Blocked, Count: 1, Rand: seededRand(2)}, vf)
	if !ok || !it.Bare || it.Frame != BlankMark || it.SentenceID != 0 {
		t.Fatalf("bare item: %+v", it)
	}
	if !strings.HasPrefix(it.Key(), "person|bare|") || lemmaOfKey(it.Key()) != it.LemmaID {
		t.Errorf("bare key %q", it.Key())
	}
	if !g.analyzer.IsAttested(it.Expected()) || it.PromptArgs["pronoun"] == "" || it.PromptArgs["lemma"] == "" {
		t.Errorf("bare item incomplete: %+v", it)
	}
}
