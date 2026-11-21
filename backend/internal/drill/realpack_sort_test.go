package drill

import (
	"math/rand"
	"slices"
	"strings"
	"testing"
	"time"

	"tanulom/internal/morph"
)

// On the real pack sort is offered for the allomorph-type topics and every
// tile's column is the allomorph of an attested form.
func TestRealPackSortIsAvailableAndSound(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	loaded(t, g)
	offered := 0
	for _, topic := range dag.Topics() {
		if len(g.sortFeatures(topic)) == 0 {
			continue
		}
		avail := g.KindsFor(topic)
		if !slices.Contains(avail.Kinds, KindSort) {
			t.Logf("%s: sort not offered (no material)", topic.Code)
			continue
		}
		offered++
		start := time.Now()
		built := 0
		for seed := int64(1); seed <= 25; seed++ {
			it, ok := g.sortItem(Request{Topic: topic, Phase: Blocked, Count: 1, Rand: rand.New(rand.NewSource(seed))})
			if !ok {
				continue
			}
			built++
			f, _ := g.Feature(it.PromptArgs["feature"])
			_, spec := g.poolFor(f)
			want := expectedParts(it.Expected())
			for _, tile := range it.Tiles {
				best, ok := g.analyzer.BestForm(tile.lemmaID, spec)
				col := want[tile.ID]
				if !ok || !best.Attested() || !strings.HasSuffix(best.Form, strings.TrimPrefix(col, "-")) {
					t.Errorf("%s: %s %s -> %q is not %s", topic.Code, tile.Text, f.ID, best.Form, col)
				}
			}
			for _, id := range it.FeatureIDs {
				if id != "Skill=VowelHarmony" && !slices.Contains(topic.FeatureIDs, id) {
					t.Errorf("%s: grades %s, not a feature of the topic", topic.Code, id)
				}
			}
		}
		t.Logf("SORT %-22s built %d/25 in %v per item", topic.Code, built, time.Since(start)/25)
		if built == 0 {
			t.Errorf("%s: offered but never built", topic.Code)
		}
	}
	if offered == 0 {
		t.Fatal("sort is offered for no topic on the real pack")
	}
	for _, code := range []string{"vowel-harmony", "locative-grid"} {
		if topic, ok := dag.Topic(code); ok && len(g.sortFeatures(topic)) > 0 && !slices.Contains(g.KindsFor(topic).Kinds, KindSort) {
			t.Errorf("%s should offer sort", code)
		}
	}
}

// Lesson examples on the real pack: attested, 4-10 words, one focus token that
// reads unambiguously as the feature; unmarked features return nothing.
func TestRealPackExamples(t *testing.T) {
	t.Parallel()
	g, dag := realGenerator(t)
	loaded(t, g)
	for _, code := range []string{"locative-grid", "accusative", "present-indefinite", "definite-conjugation"} {
		topic, ok := dag.Topic(code)
		if !ok {
			continue
		}
		for _, id := range topic.FeatureIDs {
			f := g.features[id]
			start := time.Now()
			exs, owned, err := g.Examples(topic, id, nil, nil)
			if err != nil || !owned {
				t.Fatalf("%s/%s: %v owned=%v", code, id, err, owned)
			}
			t.Logf("EXAMPLES %-22s %-18s n=%d in %v", code, id, len(exs), time.Since(start))
			if !f.HasExponent() && len(exs) != 0 {
				t.Errorf("%s has no ending but got %d examples", id, len(exs))
			}
			for _, ex := range exs {
				focus, text := "", ""
				for _, p := range ex.Parts {
					text += p.Text
					if p.Focus {
						focus = p.Text
					}
				}
				if n := wordCount(morph.Tokenize(text)); n < exampleMinWords || n > exampleMaxWords {
					t.Errorf("%s/%s: %d words in %q", code, id, n, text)
				}
				if focus == "" || !g.analyzer.IsAttested(focus) {
					t.Errorf("%s/%s: focus %q is not attested", code, id, focus)
				}
				if _, ok := g.readingOf(focus, ex.lemmaID, f); !ok {
					t.Errorf("%s/%s: %q is not an unambiguous %s", code, id, focus, id)
				}
			}
			if f.HasExponent() && len(exs) == 0 {
				t.Logf("WARNING %s/%s: no examples found", code, id)
			}
		}
	}
}
