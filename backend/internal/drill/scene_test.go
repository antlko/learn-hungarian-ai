package drill

import (
	"context"
	"encoding/json"
	"math/rand"
	"path/filepath"
	"slices"
	"strings"
	"tanulom/internal/pictures"
	"testing"
	"time"

	"tanulom/internal/db"
	"tanulom/internal/morph"
	"tanulom/internal/srs"
)

// The relation x motion table is the whole semantics of the drill; pin it.
func TestSceneCaseTable(t *testing.T) {
	want := map[[2]string]string{
		{"in", "static"}: "Ine", {"in", "into"}: "Ill", {"in", "out"}: "Ela",
		{"on", "static"}: "Sup", {"on", "into"}: "Sbl", {"on", "out"}: "Del",
	}
	for k, c := range want {
		if got := sceneCaseFor(k[0], k[1]); got != c {
			t.Errorf("%v -> %q, want %q", k, got, c)
		}
	}
	if len(sceneCases) != 6 {
		t.Errorf("exactly six cases, got %d", len(sceneCases))
	}
	for _, banned := range []string{"Ade", "All", "Abl", "Acc", "Dat", "Ins"} {
		if _, ok := sceneCases[banned]; ok {
			t.Errorf("%s must not be a scene case (postposition / not a place case)", banned)
		}
	}
	for rel, tr := range sceneTriples {
		qs := []string{sceneCases[tr[0]].question, sceneCases[tr[1]].question, sceneCases[tr[2]].question}
		if !slices.Equal(qs, []string{"hol", "hova", "honnan"}) {
			t.Errorf("%s triple asks %v", rel, qs)
		}
		for _, c := range tr {
			if sceneCases[c].relation != rel {
				t.Errorf("%s sits in the %s triple", c, rel)
			}
		}
	}
}

func TestSceneItems(t *testing.T) {
	g, dag := newTestGenerator(t)
	loaded(t, g)
	topic := topicOf(t, dag, "locative-grid")
	if a := g.KindsFor(topic); !slices.Contains(a.Kinds, KindScene) {
		t.Fatalf("locative-grid must offer scene, got %v", a.Kinds)
	}
	for _, code := range []string{"accusative", "vowel-harmony", "present-indefinite"} {
		if a := g.KindsFor(topicOf(t, dag, code)); slices.Contains(a.Kinds, KindScene) {
			t.Errorf("%s has no pair of place cases but offers scene", code)
		}
	}
	forms, _ := g.analyzer.FormsOf(1)
	n := 0
	for seed := int64(1); seed <= 60; seed++ {
		it, ok := g.sceneItem(Request{Topic: topic, Phase: Blocked, Count: 1, Rand: seededRand(seed)})
		if !ok {
			continue
		}
		n++
		if it.Kind.Typed() || len(it.Options) != 3 || it.Picture == nil {
			t.Fatalf("a scene is a three-way choice with a picture: %+v", it)
		}
		if it.Picture.Place != "house" || it.Picture.Object == "" || it.Picture.Object == it.Picture.Place {
			t.Errorf("picture %+v", it.Picture)
		}
		if it.Picture.AltEn == "" || it.Picture.AltRu == "" || it.Picture.PlaceAltEn == "" || it.Picture.ObjectAltRu == "" {
			t.Errorf("alt text missing: %+v", it.Picture)
		}
		if it.PromptArgs["place"] != "ház" || !slices.Contains([]string{"hol", "hova", "honnan"}, it.PromptArgs["question"]) {
			t.Errorf("promptArgs %v", it.PromptArgs)
		}
		c := strings.TrimPrefix(it.Headline, "Case=")
		if spec := sceneCases[c]; spec.question != it.PromptArgs["question"] || spec.motion != it.Picture.Motion {
			t.Errorf("%s does not match the picture: %+v", c, it.Picture)
		}
		if !slices.Equal(it.FeatureIDs, []string{"Case=" + c}) {
			t.Errorf("features %v", it.FeatureIDs)
		}
		hit := 0
		for _, o := range it.Options {
			attested := false
			for _, f := range forms {
				if f.Attested() && strings.EqualFold(f.Form, o.Text) {
					attested = true
				}
			}
			if !attested {
				t.Errorf("option %q is not an attested form", o.Text)
			}
			if o.Text == it.Expected() {
				hit++
			}
			for _, bad := range []string{"házan", "házra", "házról"} {
				if o.Text == bad {
					t.Errorf("a generated or non-triple form %q was offered", bad)
				}
			}
		}
		if hit != 1 {
			t.Errorf("%d options equal the answer", hit)
		}
		if !strings.HasPrefix(it.Key(), "scene|1|") || !strings.HasSuffix(it.Key(), "|"+c+"|"+it.Picture.Motion) {
			t.Errorf("key %q", it.Key())
		}
		wire, _ := json.Marshal(it)
		if strings.Contains(string(wire), `"expected"`) {
			t.Errorf("answer field on the wire: %s", wire)
		}
	}
	if n == 0 {
		t.Fatal("no scene items on the fixture")
	}
	// All three motions are served, not only the static one.
	motions := map[string]bool{}
	for seed := int64(1); seed <= 80; seed++ {
		if it, ok := g.sceneItem(Request{Topic: topic, Phase: Blocked, Count: 1, Rand: seededRand(seed)}); ok {
			motions[it.Picture.Motion] = true
		}
	}
	if len(motions) != 3 {
		t.Errorf("motions served: %v, want static, into and out", motions)
	}
}

// A place or case without an attested, unambiguous form is skipped, never made up.
func TestSceneSkipsUnattestedForms(t *testing.T) {
	g, _ := newTestGenerator(t)
	loaded(t, g)
	if _, ok := g.sceneForms(1, "in"); !ok {
		t.Error("ház has Ine/Ill/Ela attested")
	}
	if _, ok := g.sceneForms(1, "on"); ok {
		t.Error("the fixture has no Sbl/Del forms and no such features: must skip, not generate")
	}
	if _, ok := g.sceneForms(99999, "in"); ok {
		t.Error("an unknown lemma has no forms")
	}
}

func TestEngineSceneGrading(t *testing.T) {
	g, dag := newTestGenerator(t)
	loaded(t, g)
	d, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	uid, _ := d.CreateUser("a@example.com", "h", db.RoleAdmin)
	eng := NewEngine(NewStore(d.DB), srs.NewEngine(srs.NewStore(d.DB), 0.9, 20), dag, g.analyzer, GateConfig{})
	topic := topicOf(t, dag, "locative-grid")

	mk := func(c string) Item {
		for seed := int64(1); seed < 400; seed++ {
			it, ok := g.sceneItem(Request{Topic: topic, Phase: Blocked, Count: 1, Rand: seededRand(seed)})
			if ok && it.Headline == "Case="+c {
				return it
			}
		}
		t.Fatalf("no scene item for %s", c)
		return Item{}
	}
	other := func(it Item, c string) string {
		for form, cs := range it.sceneForms {
			if cs == c {
				return form
			}
		}
		t.Fatalf("no option for %s", c)
		return ""
	}
	count := func() int {
		var n int
		d.QueryRow(`SELECT COUNT(*) FROM feature_review`).Scan(&n)
		return n
	}
	class := func() string {
		var s string
		d.QueryRow(`SELECT error_class FROM drill_attempt ORDER BY id DESC LIMIT 1`).Scan(&s)
		return s
	}

	it := mk("Ine")
	res, err := eng.Answer(AnswerInput{UserID: uid, Item: it, Answer: it.Expected(), DurationMs: 1}, time.Now())
	if err != nil || !res.Correct || count() != 1 || len(res.GradedFeatures) != 1 || res.GradedFeatures[0].FeatureID != "Case=Ine" {
		t.Fatalf("a right pick is one review of the case: %+v %v", res, err)
	}
	if res.GradedFeatures[0].Rating > srs.Good {
		t.Errorf("a tap is at most Good, got %v", res.GradedFeatures[0].Rating)
	}
	for _, tc := range []struct{ pick, class string }{{"Ill", "direction"}, {"Ela", "source"}} {
		it = mk("Ine")
		res, err = eng.Answer(AnswerInput{UserID: uid, Item: it, Answer: other(it, tc.pick)}, time.Now())
		if err != nil || res.Correct {
			t.Fatalf("a wrong form was graded right: %+v %v", res, err)
		}
		if class() != tc.class {
			t.Errorf("picked %s: class %q, want %q", tc.pick, class(), tc.class)
		}
		if len(res.GradedFeatures) != 1 || res.GradedFeatures[0].Correct {
			t.Errorf("one Again on the case: %+v", res.GradedFeatures)
		}
	}
	_ = morph.NoError
}

// Anti-repeat: the key names place, object, case and motion; the same place and
// case never twice in one request while alternatives exist.
func TestSceneAntiRepeat(t *testing.T) {
	g, dag := newTestGenerator(t)
	loaded(t, g)
	topic := topicOf(t, dag, "locative-grid")
	it, ok := g.sceneItem(Request{Topic: topic, Phase: Blocked, Count: 1, Rand: seededRand(2)})
	if !ok {
		t.Fatal("no item")
	}
	rec := NewRecent()
	rec.AddItem(it)
	if stageOf(it, rec, nil, nil) != 3 {
		t.Error("an exact repeat is stage 3")
	}
	if lemmaOfKey(it.Key()) != it.LemmaID {
		t.Errorf("lemma not recoverable from %q", it.Key())
	}
	items := g.fill([]Kind{KindScene}, Request{Topic: topic, Phase: Blocked, Count: 3, Recent: NewRecent(), Rand: seededRand(4)})
	seen := map[string]bool{}
	for _, x := range items {
		k := x.Lemma + "|" + x.Headline
		if seen[k] && len(items) <= 3 {
			t.Errorf("%s twice in one session", k)
		}
		seen[k] = true
	}
	// Placement never gets a scene.
	placed, _, _ := g.Next(Request{Topic: topic, Phase: Blocked, Count: 6, Origin: "placement", Rand: seededRand(2)})
	for _, x := range placed {
		if x.Kind == KindScene {
			t.Error("placement must not serve scene")
		}
	}
}

// Real pack: 80 scene items, every option an attested form; and a sanity count
// of how many places have all their forms. The dropped catalogue lemmas are
// logged for the operator.
func TestRealPackSceneSweep(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	loaded(t, g)
	st := g.sceneCatalogue()
	t.Logf("catalogue: %d of %d resolved (%d places, %d objects); DROPPED: %v",
		len(st.places)+len(st.objects), st.total, len(st.places), len(st.objects), st.dropped)

	usable := 0
	for _, p := range st.places {
		for rel := range p.Relations {
			if _, ok := g.sceneForms(p.LemmaID, rel); ok {
				usable++
			}
		}
	}
	t.Logf("places x relations with all three attested forms: %d", usable)
	if usable < 6 {
		t.Errorf("only %d usable (place, relation) pairs", usable)
	}

	topic := topicOf(t, dag, "locative-grid")
	built := 0
	for seed := int64(0); seed < 200 && built < 80; seed++ {
		it, ok := g.sceneItem(Request{Topic: topic, Phase: Blocked, Count: 1, Rand: rand.New(rand.NewSource(seed))})
		if !ok {
			continue
		}
		built++
		forms, _ := g.analyzer.FormsOf(it.LemmaID)
		for _, o := range it.Options {
			ok := false
			for _, f := range forms {
				if f.Attested() && strings.EqualFold(f.Form, o.Text) {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s: option %q is not attested", it.Lemma, o.Text)
			}
		}
		if it.Picture.Object == it.Picture.Place || it.LemmaID == it.sceneObjectID {
			t.Errorf("object equals place: %+v", it.Picture)
		}
		if it.LemmaID == 0 || it.sceneObjectID == 0 {
			t.Errorf("unresolved lemma ids: %+v", it)
		}
	}
	if built < 80 {
		t.Fatalf("only %d scene items", built)
	}
}

// A pre-cancelled warm-up returns at once; a normal one completes and leaves the
// caches filled. Run under -race: it touches the generator's shared caches.
func TestWarm(t *testing.T) {
	g, _ := newTestGenerator(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	g.Warm(ctx)
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Errorf("a cancelled warm-up took %v", d)
	}

	g2, dag := newTestGenerator(t)
	done := make(chan struct{})
	go func() { g2.Warm(context.Background()); close(done) }()
	// A concurrent request while warming is the real situation.
	go func() {
		topic, _ := dag.Topic("locative-grid")
		g2.Next(Request{Topic: topic, Phase: Blocked, Count: 3, Rand: seededRand(1)})
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("warm-up did not finish")
	}
	g2.probeMu.Lock()
	n := len(g2.kindProbed)
	g2.probeMu.Unlock()
	if n == 0 {
		t.Error("warm-up probed nothing")
	}
}

// Match and scene sessions on the real pack: one lemma at most once per session
// (and one object, one place+case) whenever the pool is big enough.
func TestRealPackNoLemmaTwiceInMatchAndSceneSessions(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	loaded(t, g)

	poolOf := func(topic string, k Kind, key func(Item) string) int {
		seen := map[string]bool{}
		tp := topicOf(t, dag, topic)
		for seed := int64(0); seed < 150; seed++ {
			if it, ok := g.build(k, Request{Topic: tp, Phase: Blocked, Count: 1, Rand: rand.New(rand.NewSource(seed))}); ok {
				seen[key(it)] = true
			}
		}
		return len(seen)
	}
	lemma := func(it Item) string { return it.Lemma }

	for _, c := range []struct {
		topic string
		kind  Kind
		key   func(Item) string
		label string
	}{
		{"conditional", KindMatch, lemma, "lemma"},
		{"locative-grid", KindMatch, lemma, "lemma"},
		{"present-indefinite", KindMatch, lemma, "lemma"},
		{"past-tense", KindMatch, lemma, "lemma"},
		{"definite-conjugation", KindMatch, lemma, "lemma"},
		{"locative-grid", KindScene, func(it Item) string { return it.Picture.Object }, "object"},
		{"locative-grid", KindScene, func(it Item) string { return it.Lemma + "|" + it.Headline }, "place+case"},
	} {
		pool := poolOf(c.topic, c.kind, c.key)
		topic := topicOf(t, dag, c.topic)
		rec := NewRecent()
		worst := 0
		for s := range 6 {
			items := g.fill([]Kind{c.kind}, Request{Topic: topic, Phase: Blocked, Count: 10, Recent: rec,
				Rand: rand.New(rand.NewSource(int64(700 + s)))})
			counts := map[string]int{}
			for _, it := range items {
				counts[c.key(it)]++
				rec.AddItem(it)
			}
			for k, n := range counts {
				worst = max(worst, n)
				if n > 1 && pool >= 10 {
					t.Errorf("%s/%s session %d: %s %q %d times (pool %d)", c.topic, c.kind, s, c.label, k, n, pool)
				}
			}
		}
		t.Logf("POOL %-22s %-6s distinct %-10s %d (worst repeat in a session: %d)", c.topic, c.kind, c.label, pool, worst)
	}
}

// Scene pairs only objects that fit the place; a cup holds only small things.
func TestSceneObjectsFitThePlace(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	loaded(t, g)
	st := g.sceneCatalogue()
	for _, p := range st.places {
		t.Logf("place %-9s capacity %-6s usable objects %d", p.ID, p.Capacity, len(st.fits[p.ID]))
		if len(st.fits[p.ID]) < 6 {
			t.Errorf("%s has only %d usable objects", p.ID, len(st.fits[p.ID]))
		}
	}
	topic := topicOf(t, dag, "locative-grid")
	byID := map[string]pictures.Entry{}
	for _, o := range st.objects {
		byID[o.ID] = o.Entry
	}
	places := map[string]pictures.Entry{}
	for _, p := range st.places {
		places[p.ID] = p.Entry
	}
	for seed := int64(0); seed < 300; seed++ {
		it, ok := g.sceneItem(Request{Topic: topic, Phase: Blocked, Count: 1, Rand: rand.New(rand.NewSource(seed))})
		if !ok {
			continue
		}
		if !pictures.FitsEntry(byID[it.Picture.Object], places[it.Picture.Place]) {
			t.Errorf("%s in/on %s does not fit", it.Picture.Object, it.Picture.Place)
		}
	}
}
