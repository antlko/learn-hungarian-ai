package drill

import (
	"strings"

	"tanulom/internal/curriculum"
	"tanulom/internal/morph"
)

// The question-word drill: which question does the highlighted noun answer?
//
//	A [házban] van a könyv.   ->   hol? (not hova, not honnan)
//
// It teaches the case through the MEANING it expresses (place, goal, source)
// rather than through the suffix, which is how Hungarian textbooks introduce the
// locative grid. The answer is a closed table, so there is nothing to attest;
// what has to be attested is the sentence reading, and that is held to the same
// standard as every other drill: the noun must read unambiguously for the case,
// never from a generated analysis.

// Error classes of a wrong question word. They live here, like ErrPerson.
const (
	// ErrDirection: location mistaken for goal or the reverse (hol / hova).
	ErrDirection morph.ErrorClass = "direction"
	// ErrSource: source (honnan) mixed up with location or goal.
	ErrSource morph.ErrorClass = "source"
)

// qwordFor maps a case to its question word. The locative grid shares three:
// interior, surface and proximity all answer hol / hova / honnan.
var qwordFor = map[string]string{
	"Ine": "hol", "Sup": "hol", "Ade": "hol",
	"Ill": "hova", "Sbl": "hova", "All": "hova",
	"Ela": "honnan", "Del": "honnan", "Abl": "honnan",
	"Acc": "mit/kit", "Dat": "kinek", "Ins": "mivel/kivel",
}

// qwordOptions are the options offered for an answer: the other directions for a
// locative, the adjacent cases otherwise.
var qwordOptions = map[string][]string{
	"hol": {"hol", "hova", "honnan"}, "hova": {"hol", "hova", "honnan"},
	"honnan":      {"hol", "hova", "honnan"},
	"mit/kit":     {"mit/kit", "kinek", "hol"},
	"kinek":       {"kinek", "mit/kit", "hol"},
	"mivel/kivel": {"mivel/kivel", "mit/kit", "kinek"},
}

// qwordCases are the topic's own cases that have a question word.
func (g *Generator) qwordCases(t curriculum.Topic) []Feature {
	var out []Feature
	for _, id := range t.FeatureIDs {
		f, ok := g.features[id]
		if ok && f.Key == "Case" && qwordFor[f.Value] != "" {
			out = append(out, f)
		}
	}
	return out
}

// caseReading reports whether a token is UNAMBIGUOUSLY the wanted case of the
// wanted lemma: no non-generated reading of any lemma disagrees about the case,
// and the lemma has one that agrees.
func (g *Generator) caseReading(form string, lemmaID int64, f Feature) bool {
	all, err := g.analyzer.Lookup(form)
	if err != nil {
		return false
	}
	found := false
	for _, a := range all {
		if a.Source == "generated" {
			continue
		}
		v, present := a.Feats[f.Key]
		if !present {
			continue
		}
		if v != f.Value {
			return false
		}
		if a.LemmaID == lemmaID {
			found = true
		}
	}
	return found
}

// qwordItem builds one question-word item from a pack sentence.
func (g *Generator) qwordItem(req Request) (Item, bool) {
	cases := g.qwordCases(req.Topic)
	if len(cases) == 0 {
		return Item{}, false
	}
	for range maxCandidateTries {
		f := g.weightedFeature(cases, req)
		w, ok := g.pickWord(g.nouns, req)
		if !ok {
			continue
		}
		sentID, hu, ok := g.sampleSentenceContaining([]int64{w.LemmaID}, req.Rand)
		if !ok {
			continue
		}
		tokens := morph.Tokenize(hu)
		if n := wordCount(tokens); n < 3 || n > 14 {
			continue
		}
		for _, t := range tokens {
			if !t.IsWord || !g.caseReading(t.Text, w.LemmaID, f) {
				continue
			}
			want := qwordFor[f.Value]
			var opts []Option
			for _, o := range qwordOptions[want] {
				opts = append(opts, Option{Text: o, Attested: true})
			}
			req.Rand.Shuffle(len(opts), func(i, j int) { opts[i], opts[j] = opts[j], opts[i] })

			// The sentence is split HERE: the client never slices Hungarian with
			// byte offsets (they are UTF-16 in the browser).
			var parts []FramePart
			if t.Start > 0 {
				parts = append(parts, FramePart{Text: hu[:t.Start]})
			}
			parts = append(parts, FramePart{Text: hu[t.Start:t.End], Focus: true})
			if t.End < len(hu) {
				parts = append(parts, FramePart{Text: hu[t.End:]})
			}
			ru, en := g.gloss(w.LemmaID)
			return Item{
				TopicCode: req.Topic.Code, Kind: KindQword, Phase: req.Phase,
				PromptKey:  "drill.prompt.qword",
				PromptArgs: map[string]string{"lemma": w.Lemma},
				Parts:      parts,
				Lemma:      w.Lemma, LemmaID: w.LemmaID, GlossRU: ru, GlossEN: en,
				Options: opts, FeatureIDs: []string{f.ID}, Headline: f.ID,
				Attested: true, SentenceID: sentID, expected: want,
			}, true
		}
	}
	return Item{}, false
}

// classifyQword says what a wrong question word means: a source mistaken for a
// place or goal (or the reverse) is `source`; place against goal is `direction`;
// anything else is a different case altogether.
func classifyQword(expected, chosen string) morph.ErrorClass {
	chosen = strings.ToLower(strings.TrimSpace(chosen))
	locative := func(s string) bool { return s == "hol" || s == "hova" || s == "honnan" }
	if !locative(expected) || !locative(chosen) {
		return morph.ErrCase
	}
	if expected == "honnan" || chosen == "honnan" {
		return ErrSource
	}
	return ErrDirection
}

// probeKind proves, once per topic and kind, that the pack can build an item.
// Cached, with a fixed seed, so the verdict does not flicker between starts.
func (g *Generator) probeKind(t curriculum.Topic, k Kind) bool {
	key := t.Code + "|" + string(k)
	g.probeMu.Lock()
	if v, ok := g.kindProbed[key]; ok {
		g.probeMu.Unlock()
		return v
	}
	g.probeMu.Unlock()

	req := Request{Topic: t, Phase: Blocked, Count: 1, Rand: newProbeRand()}
	// Each build is itself up to maxCandidateTries draws, and on a thin pack one
	// call succeeds only about a third of the time; ten calls make a false
	// "no material" verdict vanishingly rare without costing more than once.
	ok := false
	for range 10 {
		if _, built := g.build(k, req); built {
			ok = true
			break
		}
	}
	g.probeMu.Lock()
	if g.kindProbed == nil {
		g.kindProbed = map[string]bool{}
	}
	g.kindProbed[key] = ok
	g.probeMu.Unlock()
	return ok
}
