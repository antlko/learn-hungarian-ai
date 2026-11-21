package drill

import (
	"slices"
	"strings"
	"testing"
)

// sortNouns adds enough nouns for a six-tile item, each with the attested
// forms the miniature pack's locative features use. `híd` is the antiharmonic
// exception (hídon, not híden); `hotel` and `papír` are mixed-vowel stems.
func sortNouns(exec func(q string, args ...any)) {
	nouns := []struct {
		id              int64
		lemma, ine, ill string
		sup             string
	}{
		{20, "kert", "kertben", "kertbe", "kerten"},
		{21, "szék", "székben", "székbe", "széken"},
		{22, "lány", "lányban", "lányba", "lányon"},
		{23, "híd", "hídban", "hídba", "hídon"},
		{24, "tó", "tóban", "tóba", "tón"},
		{25, "fiú", "fiúban", "fiúba", "fiún"},
		{26, "ember", "emberben", "emberbe", "emberen"},
		{27, "hotel", "hotelben", "hotelbe", "hotelen"},
		{28, "papír", "papírban", "papírba", "papíron"},
	}
	for i, n := range nouns {
		exec(`INSERT INTO lemma (id, lemma, upos, freq_rank) VALUES (?,?,?,?)`, n.id, n.lemma, "NOUN", 300+i)
		exec(`INSERT INTO lemma_gloss VALUES (?,?,?,?)`, n.id, "en", 0, "en "+n.lemma)
		exec(`INSERT INTO lemma_gloss VALUES (?,?,?,?)`, n.id, "ru", 0, "ru "+n.lemma)
		for form, feats := range map[string]string{
			n.lemma: "Case=Nom|Number=Sing",
			n.ine:   "Case=Ine|Number=Sing", n.ill: "Case=Ill|Number=Sing", n.sup: "Case=Sup|Number=Sing",
		} {
			exec(`INSERT INTO wordform VALUES (?,?,?,?,?,?)`, form, n.id, "NOUN", feats, "wiktionary", "high")
		}
	}
}

func TestSortItemsAreAttestedAndWellFormed(t *testing.T) {
	g, dag := newTestGeneratorWith(t, sortNouns)
	loaded(t, g)
	sawHid := false
	for _, code := range []string{"locative-grid", "vowel-harmony"} {
		topic := topicOf(t, dag, code)
		built := 0
		for seed := int64(1); seed <= 60; seed++ {
			it, ok := g.sortItem(Request{Topic: topic, Phase: Blocked, Count: 1, Rand: seededRand(seed)})
			if !ok {
				continue
			}
			built++
			f, _ := g.Feature(it.PromptArgs["feature"])
			allo := Distinct(f.Allomorphs)
			if it.Kind != KindSort || it.Kind.Typed() || len(it.Options) != 0 {
				t.Fatalf("%s: a sort item is a choice item, got %+v", code, it)
			}
			if len(it.Tiles) != sortTiles || len(it.Columns) < 2 || len(it.Columns) > 3 {
				t.Fatalf("%s: %d tiles, columns %v", code, len(it.Tiles), it.Columns)
			}
			var labels []string
			for _, a := range allo {
				labels = append(labels, "-"+a)
			}
			for _, c := range it.Columns {
				if !slices.Contains(labels, c) {
					t.Errorf("%s: column %q is not an allomorph of %s (%v)", code, c, f.ID, labels)
				}
			}
			want := expectedParts(it.Expected())
			seenText, seenID := map[string]bool{}, map[string]bool{}
			used := map[string]bool{}
			for _, tile := range it.Tiles {
				if seenText[tile.Text] || seenID[tile.ID] {
					t.Errorf("%s: duplicate tile %+v", code, tile)
				}
				seenText[tile.Text], seenID[tile.ID] = true, true
				if tile.GlossRU == "" || tile.GlossEN == "" {
					t.Errorf("%s: tile %q has no gloss", code, tile.Text)
				}
				col := want[tile.ID]
				used[col] = true
				if !slices.Contains(it.Columns, col) {
					t.Errorf("%s: expected column %q is not offered", code, col)
				}
				// The column is the allomorph of the ATTESTED form.
				_, spec := g.poolFor(f)
				best, ok := g.analyzer.BestForm(tile.lemmaID, spec)
				if !ok || !best.Attested() || !strings.HasSuffix(best.Form, strings.TrimPrefix(col, "-")) {
					t.Errorf("%s: %s + %s: attested form %q does not end in %s", code, tile.Text, f.ID, best.Form, col)
				}
				if tile.Text == "híd" && f.ID == "Case=Sup" {
					sawHid = true
					if col != "-on" {
						t.Errorf("híd takes -on, got %s", col)
					}
				}
			}
			for _, c := range it.Columns {
				if !used[c] {
					t.Errorf("%s: column %s has no tile", code, c)
				}
			}
			for _, id := range it.FeatureIDs {
				if id != "Skill=VowelHarmony" && !slices.Contains(topic.FeatureIDs, id) {
					t.Errorf("%s: feature %s is not the topic's", code, id)
				}
			}
			if it.Headline == "" || len(it.FeatureIDs) == 0 {
				t.Errorf("%s: an item that grades nothing", code)
			}
		}
		if built == 0 {
			t.Errorf("%s: no sort item could be built", code)
		}
	}
	if !sawHid {
		t.Error("the híd exception never appeared; the test does not prove it")
	}
}

func TestSortIsOfferedOnlyWhereItCanBeBuilt(t *testing.T) {
	// The shared pack has two nouns: not enough for six tiles.
	thin, dag := newTestGenerator(t)
	if a := thin.KindsFor(topicOf(t, dag, "locative-grid")); slices.Contains(a.Kinds, KindSort) {
		t.Errorf("sort offered without material: %v", a.Kinds)
	}
	g, dag := newTestGeneratorWith(t, sortNouns)
	for _, code := range []string{"locative-grid", "vowel-harmony"} {
		if a := g.KindsFor(topicOf(t, dag, code)); !slices.Contains(a.Kinds, KindSort) {
			t.Errorf("%s: sort missing from %v", code, a.Kinds)
		}
	}
	// A single-allomorph topic has nothing to sort by.
	if a := g.KindsFor(topicOf(t, dag, "accusative")); slices.Contains(a.Kinds, KindSort) {
		t.Errorf("accusative offers sort: %v", a.Kinds)
	}
}

func TestSortIsNeverInPlacement(t *testing.T) {
	g, dag := newTestGeneratorWith(t, sortNouns)
	topic := topicOf(t, dag, "locative-grid")
	for seed := int64(1); seed <= 20; seed++ {
		// Even interleaved, where the draw is random over every kind.
		for _, ph := range []Phase{Blocked, Interleaved} {
			items, _, err := g.Next(Request{Topic: topic, Phase: ph, Count: 6, Origin: "placement", Rand: seededRand(seed)})
			if err != nil {
				t.Fatal(err)
			}
			for _, it := range items {
				if it.Kind == KindSort {
					t.Fatalf("placement served a sort item: %+v", it)
				}
			}
		}
	}
}

func TestSortAnswerValidationAndGrading(t *testing.T) {
	it := Item{
		Kind: KindSort, Columns: []string{"-ban", "-ben"},
		Tiles:    []Tile{{ID: "t0", Text: "ház"}, {ID: "t1", Text: "kert"}, {ID: "t2", Text: "híd"}},
		expected: "t0=-ban\nt1=-ben\nt2=-ban",
	}
	bad := []map[string]string{
		nil,
		{"t0": "-ban", "t1": "-ben"}, // missing tile
		{"t0": "-ban", "t1": "-ben", "t2": "-ban", "t3": "-ben"}, // extra tile
		{"t0": "-ban", "t1": "-ben", "t2": "-on"},                // no such column
		{"t0": "-ban", "t1": "-ben", "tX": "-ban"},               // unknown tile
	}
	for i, p := range bad {
		if err := it.CheckParts(p); err == nil {
			t.Errorf("case %d: %v should be refused", i, p)
		}
	}
	good := map[string]string{"t0": "-ban", "t1": "-ben", "t2": "-ban"}
	if err := it.CheckParts(good); err != nil {
		t.Fatal(err)
	}
	if res, all := it.gradeParts(good); !all || len(res) != 3 {
		t.Errorf("all right: %v %v", res, all)
	}
	wrong := map[string]string{"t0": "-ban", "t1": "-ben", "t2": "-ben"}
	res, all := it.gradeParts(wrong)
	if all || res[2].Correct || !res[0].Correct || res[2].Expected != "-ban" {
		t.Errorf("one wrong: %+v", res)
	}
}

func TestSortKeyIgnoresShuffleOrder(t *testing.T) {
	a := Item{Kind: KindSort, PromptArgs: map[string]string{"feature": "Case=Ine"},
		Tiles: []Tile{{ID: "t0", lemmaID: 2}, {ID: "t1", lemmaID: 10}}}
	b := Item{Kind: KindSort, PromptArgs: map[string]string{"feature": "Case=Ine"},
		Tiles: []Tile{{ID: "t0", lemmaID: 10}, {ID: "t1", lemmaID: 2}}}
	c := Item{Kind: KindSort, PromptArgs: map[string]string{"feature": "Case=Ill"}, Tiles: a.Tiles}
	if a.Key() != b.Key() {
		t.Errorf("the same lemmas shuffled are one item: %q vs %q", a.Key(), b.Key())
	}
	if a.Key() == c.Key() {
		t.Error("another feature is another item")
	}
	if got := sortLemmasOfKey(a.Key()); !slices.Equal(got, []int64{10, 2}) && !slices.Equal(got, []int64{2, 10}) {
		t.Errorf("lemmas of key = %v", got)
	}
	r := NewRecent()
	r.AddItem(a)
	if _, seen := r.keyRank(b); !seen {
		t.Error("a served sort item must count as recent")
	}
	if r.lemmaUses(10) != 1 || r.lemmaUses(2) != 1 {
		t.Errorf("tile lemmas not counted as used: %v", r.Lemmas)
	}
}

func TestSortRotatesAsAChoiceKind(t *testing.T) {
	order := blockedOrder([]Kind{KindAllomorph, KindSort, KindCloze}, nil)
	if !slices.Contains(order[:2], KindSort) || order[2] != KindCloze {
		t.Errorf("sort is a choice kind, cloze stays typed and last: %v", order)
	}
}
