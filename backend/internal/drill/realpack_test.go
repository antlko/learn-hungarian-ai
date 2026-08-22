package drill

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tanulom/internal/curriculum"
	"tanulom/internal/lang"
	"tanulom/internal/morph"
)

// Generation against the REAL language pack.
//
// The fixture tests prove the logic; this one proves the logic survives contact
// with 1.98 million wordforms scraped from Wiktionary, which contain genuine
// defects — `-` stored as the essive of several thousand nouns, `csapdába`
// tagged both singular and plural, and frequency ranks assigned per lemma STRING
// so that the NOUN reading of `nem` sits at rank 4. Every one of those produced a
// wrong or nonsensical drill item before it was fixed.
//
// Skipped when no pack is present, so CI stays green without a 260 MB artefact.
// Run it with the pack in place:
//
//	go test ./internal/drill -run RealPack -v
func realPack(t *testing.T) *lang.Pack {
	t.Helper()
	path := os.Getenv("TANULOM_TEST_PACK")
	if path == "" {
		path = filepath.Join("..", "..", "..", "data", "langpack-hu-v1.sqlite")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no language pack at %s — set TANULOM_TEST_PACK to run this", path)
	}
	p, err := lang.Open(path)
	if err != nil {
		t.Fatalf("open pack: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

func realGenerator(t *testing.T) (*Generator, *curriculum.DAG) {
	t.Helper()
	p := realPack(t)
	dag, err := curriculum.Load(p)
	if err != nil {
		t.Fatalf("load curriculum: %v", err)
	}
	return NewGenerator(p, morph.NewAnalyzer(p), dag), dag
}

// Every item the real pack produces must be answerable and correctly attributed.
func TestRealPackItemsAreSound(t *testing.T) {
	g, dag := realGenerator(t)
	analyzer := g.analyzer

	produced := 0
	for _, topic := range dag.Topics() {
		if !g.Availability(topic).Available {
			continue
		}
		for _, phase := range []Phase{Blocked, Interleaved} {
			items, _, err := g.Next(Request{
				Topic: topic, Phase: phase, Count: 12,
				Definiteness: Weights{DefiniteObjectPct: 55, DemonstrativePct: 25},
				Rand:         rand.New(rand.NewSource(int64(topic.TeachOrder))),
			})
			if err != nil {
				t.Fatalf("%s/%s: %v", topic.Code, phase, err)
			}
			for _, it := range items {
				produced++
				where := topic.Code + "/" + string(it.Kind)

				if !analyzer.IsAttested(it.Expected()) {
					t.Errorf("%s: expected answer %q is not attested", where, it.Expected())
				}
				// `-` is Wiktionary's empty-cell marker and the pack imported it
				// as a form. It is attested, high-confidence and the shortest
				// candidate, so it wins the trust sort outright if not rejected.
				if !hasLetter(it.Expected()) {
					t.Errorf("%s: expected answer %q contains no letters", where, it.Expected())
				}
				if len(it.FeatureIDs) == 0 {
					t.Errorf("%s: item grades nothing", where)
				}
				for _, id := range it.FeatureIDs {
					if _, ok := g.features[id]; !ok {
						t.Errorf("%s: grades unknown feature %q", where, id)
					}
				}
				if it.Kind.Typed() && len(it.Options) != 0 {
					t.Errorf("%s: a typed item must not ship options", where)
				}
				if !it.Kind.Typed() {
					matches := 0
					seen := map[string]bool{}
					for _, o := range it.Options {
						if seen[strings.ToLower(o.Text)] {
							t.Errorf("%s: duplicate option %q", where, o.Text)
						}
						seen[strings.ToLower(o.Text)] = true
						if o.Text == it.Expected() {
							matches++
						}
					}
					if len(it.Options) < 2 {
						t.Errorf("%s: %d options is not a choice", where, len(it.Options))
					}
					if matches != 1 {
						t.Errorf("%s: %d options match the answer, want 1", where, matches)
					}
				}
				if it.Frame != "" && !strings.Contains(it.Frame, BlankMark) {
					t.Errorf("%s: frame %q has no blank", where, it.Frame)
				}
				if excludedLemmas[it.Lemma] {
					t.Errorf("%s: built a drill around %q", where, it.Lemma)
				}
				if it.Lemma != "" && it.Lemma != strings.ToLower(it.Lemma) {
					t.Errorf("%s: %q is a proper noun or acronym", where, it.Lemma)
				}
			}
		}
	}
	if produced < 100 {
		t.Fatalf("only %d items across the whole curriculum — the generators are not working", produced)
	}
	t.Logf("generated and checked %d items from the real pack", produced)
}

// The harmony answer must be the one the classifier predicts. If these two ever
// disagree, the app is marking a learner wrong against its own rule.
func TestRealPackHarmonyAnswersMatchTheRule(t *testing.T) {
	g, dag := realGenerator(t)
	topic, ok := dag.Topic("vowel-harmony")
	if !ok {
		t.Skip("no vowel-harmony topic in this pack")
	}
	items, _, err := g.Next(Request{
		Topic: topic, Phase: Blocked, Count: 40, Rand: rand.New(rand.NewSource(3)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 10 {
		t.Fatalf("only %d harmony items", len(items))
	}
	for _, it := range items {
		want := string(Classify(it.Lemma))
		if it.PromptArgs["harmony"] != want {
			t.Errorf("%s: item says harmony=%q, Classify says %q",
				it.Lemma, it.PromptArgs["harmony"], want)
		}
		// Pure harmony practice grades the skill and nothing else: the learner
		// was never asked what the suffix means.
		if len(it.FeatureIDs) != 1 || it.FeatureIDs[0] != curriculum.FeatureVowelHarmony {
			t.Errorf("%s: harmony item grades %v", it.Lemma, it.FeatureIDs)
		}
	}
}

// A topic that cannot produce items must say so BEFORE the learner opens it.
// The pack's attested `olvashat` carries no features at all, so the potential
// mood passes every structural test and yields nothing.
func TestRealPackAvailabilityIsProvenNotAssumed(t *testing.T) {
	g, dag := realGenerator(t)
	for _, topic := range dag.Topics() {
		avail := g.Availability(topic)
		if !avail.Available {
			if avail.Reason == "" {
				t.Errorf("%s: unavailable with no reason given", topic.Code)
			}
			continue
		}
		items, _, err := g.Next(Request{
			Topic: topic, Phase: Blocked, Count: 6,
			Definiteness: Weights{DefiniteObjectPct: 55, DemonstrativePct: 25},
			Rand:         rand.New(rand.NewSource(99)),
		})
		if err != nil {
			t.Fatalf("%s: %v", topic.Code, err)
		}
		if len(items) == 0 {
			t.Errorf("%s: reported available but produced nothing", topic.Code)
		}
	}
}

// Blocked practice is one kind, on the real pack too — the mix is the thing it
// exists to avoid.
func TestRealPackBlockedPracticeIsSingleKind(t *testing.T) {
	g, dag := realGenerator(t)
	for _, topic := range dag.Topics() {
		if !g.Availability(topic).Available {
			continue
		}
		items, _, err := g.Next(Request{
			Topic: topic, Phase: Blocked, Count: 10,
			Definiteness: Weights{DefiniteObjectPct: 55, DemonstrativePct: 25},
			Rand:         rand.New(rand.NewSource(5)),
		})
		if err != nil || len(items) == 0 {
			continue
		}
		kinds := map[Kind]bool{}
		for _, it := range items {
			kinds[it.Kind] = true
		}
		if len(kinds) != 1 {
			t.Errorf("%s: blocked practice served %d kinds", topic.Code, len(kinds))
		}
	}
}

func hasLetter(s string) bool {
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r > 127 {
			return true
		}
	}
	return false
}
