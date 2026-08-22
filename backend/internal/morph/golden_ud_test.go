package morph

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The morphology, scored against human annotation instead of against itself.
//
// The pack's headline quality figure — 88.2% token resolution — is measured over
// the pack's own corpus with the pack's own tables. It answers "did the lookup
// find something", not "was it right", and a lookup table cannot be wrong by
// that measure because it IS the measure. This test asks the other question.
//
// The gold standard is UD Hungarian-Szeged's test split: 449 sentences of real
// newspaper and fiction prose, lemmatised and feature-tagged by linguists, and
// held out from everything the pack was built from.
//
// It is CC BY-NC-SA, so it is fetched into langpack/cache and never committed:
//
//	cd langpack && python3 -c "from pipeline import sources; \
//	    sources.fetch(next(s for s in sources.SOURCES if s.key=='ud_treebank'))"
//
// Without it the test skips, the same way the other real-pack tests do.

// Measured on hu-v1 (full pack), 8 873 word tokens:
//
//	coverage     78.2%   an analysis was produced at all
//	lemma        75.1%   the gold lemma among the returned candidates
//	UPOS         70.0%
//	feature set  56.2%   exact, over the 13 feature keys the pack models
//
// The plan asked for lemma ≥ 95% and features ≥ 90%. It is worth being exact
// about why those are not the numbers, because the gap is a design consequence
// and not a bug to be fixed later:
//
//  1. This is a DICTIONARY LOOKUP WITH NO SENTENCE CONTEXT. The plan's figures
//     assumed HuSpaCy, a contextual tagger. `vár` is "castle" and "waits";
//     `ért` is "understands" and a suffixed stem; `nyugat` is a noun and a
//     name. A human reads the sentence. This analyser is handed one word.
//     [ADR-002](../../../.claude/DECISIONS.md#adr-002) chose that trade on
//     purpose — no Python at runtime — and this is its price, now measured.
//  2. Coverage is the binding constraint, not correctness. Of the 1 938 tokens
//     with no analysis, proper nouns (60% of 745) and verbs (45% of 927)
//     dominate: names are not in a dictionary, and the conjugation generator
//     covers fewer forms than the noun one. Of what IS analysed, the lemma is
//     right 96% of the time.
//  3. One convention difference is counted as an error rather than normalised
//     away: on a plural possessive UD writes `Number=Plur` where this pack
//     writes `Number=Sing|Number[psed]=Plur`. Ours is the more explicit
//     analysis, and it still fails the comparison. See docs/LANGPACK.md.
//
// The floors are the measured values minus roughly three points of slack for
// pack rebuilds. They exist to catch a regression, not to look impressive: a
// number the code cannot reach is not a gate, it is a wish with a build step.
const (
	minLemmaAccuracy = 0.72
	minUPOSAccuracy  = 0.67
	minFeatsExact    = 0.53
	// Coverage is the share of gold tokens the analyser produced any answer for.
	// It bounds all three above: an analysis that is never offered cannot be
	// right, and this separates "wrong" from "absent", which are different
	// failures with different fixes.
	minCoverage = 0.75
)

type udToken struct {
	form  string
	lemma string
	upos  string
	feats map[string]string
}

func loadUD(t *testing.T) []udToken {
	t.Helper()
	path := os.Getenv("TANULOM_TEST_UD")
	if path == "" {
		path = filepath.Join("..", "..", "..", "langpack", "cache", "hu_szeged-ud-test.conllu")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("no UD treebank at %s — see the comment at the top of this file", path)
	}
	defer f.Close()

	var out []udToken
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 6 {
			continue
		}
		// Multi-word tokens ("1-2") and empty nodes ("1.1") expand into the rows
		// that follow; scoring the range as well would count them twice.
		if strings.ContainsAny(cols[0], "-.") {
			continue
		}
		// Punctuation is not in scope and never reaches the analyser: the
		// tokeniser marks it `IsWord: false` and the reader renders it as text.
		// Leaving UD's 1 479 punctuation tokens in the denominator would drag
		// every figure below down by fourteen points and measure nothing —
		// so the filter is the tokeniser's own verdict, not a POS whitelist,
		// which would be circular.
		if ts := Tokenize(cols[1]); len(ts) != 1 || !ts[0].IsWord {
			continue
		}
		tok := udToken{form: cols[1], lemma: cols[2], upos: cols[3], feats: map[string]string{}}
		if cols[5] != "_" {
			for _, kv := range strings.Split(cols[5], "|") {
				if k, v, ok := strings.Cut(kv, "="); ok {
					tok.feats[k] = v
				}
			}
		}
		out = append(out, tok)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("reading the treebank: %v", err)
	}
	if len(out) < 5000 {
		t.Fatalf("the treebank yielded only %d tokens; expected ~10 000", len(out))
	}
	return out
}

// featsEqual compares only the features BOTH sides can express.
//
// UD annotates syntax-adjacent categories this pack has no inventory for
// (PronType, Degree, VerbForm, Voice), and marking the analyser wrong for not
// emitting a feature it never claims to model would measure the schema, not the
// analysis. The comparison is therefore restricted to the pack's own feature
// keys — and it is exact within that set: a missing Case is a failure, and so is
// the wrong Number.
// normaliseUnmarked writes out what a bare Hungarian stem already says.
//
// UD annotates the nominative singular explicitly (`ház` is Case=Nom|Number=Sing);
// this pack treats it as the absence of a suffix, because that is what it is —
// there is no nominative ending to segment and no feature card to schedule for
// one. Both conventions are defensible and they disagree on roughly a third of
// all nominal tokens, so comparing them raw measures the convention rather than
// the analysis.
//
// The mapping only ever ADDS the default, and only where the analysis is silent:
// an analysis that says Case=Acc is left alone, and one that says Number=Plur is
// left alone. It cannot turn a wrong answer into a right one.
func normaliseUnmarked(upos string, feats map[string]string) map[string]string {
	out := make(map[string]string, len(feats)+5)
	for k, v := range feats {
		out[k] = v
	}
	fill := func(k, v string) {
		if _, ok := out[k]; !ok {
			out[k] = v
		}
	}
	switch upos {
	case "NOUN", "PROPN", "NUM", "PRON":
		fill("Case", "Nom")
		fill("Number", "Sing")
	case "ADJ":
		fill("Case", "Nom")
		fill("Number", "Sing")
		// `szép` is the positive degree the way `ház` is the nominative: the
		// comparative is `szebb`, and the thing being compared to is the bare
		// adjective. UD writes Degree=Pos out; this pack emits Degree only when
		// a suffix put it there.
		fill("Degree", "Pos")
	case "ADV":
		fill("Degree", "Pos")
	case "VERB":
		// The bare verb stem IS the third-person singular present indicative
		// indefinite — `ró` is "he inscribes", with nothing added. Same rule as
		// the nominative above and the same justification: this pack schedules
		// what a suffix teaches, and there is no suffix here to teach.
		fill("Mood", "Ind")
		fill("Tense", "Pres")
		fill("Person", "3")
		fill("Number", "Sing")
		fill("Definite", "Ind")
	case "DET":
		// UD marks `a`/`az` as Definite=Def. That is the article's lexical
		// class, not morphology: Hungarian articles do not inflect, and the
		// a/az alternation is phonological. This pack's `Definite` is the
		// VERB conjugation feature, which happens to share UD's name — so on a
		// determiner the two are different features and comparing them scores
		// a collision rather than an analysis.
		delete(out, "Definite")
	}
	return out
}

func featsEqual(gold, got map[string]string, modelled map[string]bool) bool {
	for k, v := range gold {
		if !modelled[k] {
			continue
		}
		if got[k] != v {
			return false
		}
	}
	for k, v := range got {
		if !modelled[k] {
			continue
		}
		if gold[k] != v {
			return false
		}
	}
	return true
}

func TestGoldenAgainstUDHungarian(t *testing.T) {
	tokens := loadUD(t)
	a := realPackAnalyzer(t)

	// The feature keys this pack models, read from the pack rather than listed
	// here, so extending the inventory automatically widens what is scored.
	modelled := map[string]bool{}
	rows, err := a.pack.DB().Query(`SELECT DISTINCT key FROM morph_feature`)
	if err != nil {
		t.Fatalf("reading the feature inventory: %v", err)
	}
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			t.Fatal(err)
		}
		modelled[f] = true
	}
	rows.Close()
	if len(modelled) == 0 {
		t.Fatal("the pack declares no morphological features")
	}

	var analysed, lemmaOK, uposOK, featsOK int
	// Where it goes wrong matters more than that it does: a lemma missed on an
	// unanalysable proper noun is a different problem from one missed on a verb.
	missByUPOS := map[string]int{}
	goldByUPOS := map[string]int{}
	var examples []string

	for _, tok := range tokens {
		goldByUPOS[tok.upos]++
		res, err := a.Lookup(tok.form)
		if err != nil || len(res) == 0 {
			missByUPOS[tok.upos]++
			continue
		}
		analysed++
		best := res[0]

		// The analyser returns ranked candidates and has no sentence to
		// disambiguate with, so a gold match anywhere in the returned set is
		// scored as a hit for lemma and POS. That is what the UI does too: an
		// ambiguous token is shown with its alternatives, not silently
		// collapsed to one. Features are stricter — they are scored on the
		// candidate the UI actually leads with.
		for _, cand := range res {
			if cand.Lemma == tok.lemma {
				lemmaOK++
				break
			}
		}
		for _, cand := range res {
			if cand.UPOS == tok.upos {
				uposOK++
				break
			}
		}
		if featsEqual(
			normaliseUnmarked(tok.upos, tok.feats),
			normaliseUnmarked(best.UPOS, best.Feats),
			modelled,
		) {
			featsOK++
		} else if len(examples) < 12 {
			examples = append(examples, fmt.Sprintf("%s (%s): gold %v, got %v",
				tok.form, tok.upos, tok.feats, best.Feats))
		}
	}

	total := len(tokens)
	coverage := float64(analysed) / float64(total)
	// Scored over ALL gold tokens, not only the analysed ones: a system that
	// answers a tenth of the corpus perfectly is not a 100% system, and dividing
	// by what it chose to answer is exactly how that gets claimed.
	lemmaAcc := float64(lemmaOK) / float64(total)
	uposAcc := float64(uposOK) / float64(total)
	featsAcc := float64(featsOK) / float64(total)

	t.Logf("UD Hungarian-Szeged test split: %d tokens", total)
	t.Logf("  coverage      %.1f%%  (%d analysed, %d with no analysis)",
		coverage*100, analysed, total-analysed)
	t.Logf("  lemma         %.1f%%  (correct lemma among the candidates)", lemmaAcc*100)
	t.Logf("  UPOS          %.1f%%", uposAcc*100)
	t.Logf("  feature set   %.1f%%  (exact, over the %d features the pack models)",
		featsAcc*100, len(modelled))

	type miss struct {
		upos       string
		missed     int
		goldTokens int
	}
	var misses []miss
	for upos, n := range goldByUPOS {
		misses = append(misses, miss{upos, missByUPOS[upos], n})
	}
	sort.Slice(misses, func(i, j int) bool { return misses[i].missed > misses[j].missed })
	t.Log("  no analysis, by gold POS:")
	for _, m := range misses[:min(6, len(misses))] {
		if m.missed == 0 {
			break
		}
		t.Logf("    %-6s %4d of %4d (%.0f%%)", m.upos, m.missed, m.goldTokens,
			100*float64(m.missed)/float64(m.goldTokens))
	}
	for _, e := range examples[:min(6, len(examples))] {
		t.Logf("  feature miss: %s", e)
	}

	if coverage < minCoverage {
		t.Errorf("coverage %.1f%% is below the %.0f%% floor", coverage*100, minCoverage*100)
	}
	if lemmaAcc < minLemmaAccuracy {
		t.Errorf("lemma accuracy %.1f%% is below the %.0f%% floor", lemmaAcc*100, minLemmaAccuracy*100)
	}
	if uposAcc < minUPOSAccuracy {
		t.Errorf("UPOS accuracy %.1f%% is below the %.0f%% floor", uposAcc*100, minUPOSAccuracy*100)
	}
	if featsAcc < minFeatsExact {
		t.Errorf("feature-set accuracy %.1f%% is below the %.0f%% floor", featsAcc*100, minFeatsExact*100)
	}
}

// The claim that matters most for trust: when the analyser says `certain`, it
// should be. A wrong analysis labelled uncertain costs the learner a moment;
// a wrong one labelled certain costs them the habit of checking.
func TestCertainAnalysesAreMoreOftenRightThanUncertainOnes(t *testing.T) {
	tokens := loadUD(t)
	a := realPackAnalyzer(t)

	type score struct{ right, total int }
	by := map[Certainty]*score{}

	for _, tok := range tokens {
		res, err := a.Lookup(tok.form)
		if err != nil || len(res) == 0 {
			continue
		}
		best := res[0]
		s := by[best.Certainty]
		if s == nil {
			s = &score{}
			by[best.Certainty] = s
		}
		s.total++
		if best.Lemma == tok.lemma {
			s.right++
		}
	}

	rate := func(c Certainty) float64 {
		s := by[c]
		if s == nil || s.total == 0 {
			return -1
		}
		return float64(s.right) / float64(s.total)
	}
	for c, s := range by {
		t.Logf("  %-10s %5d tokens, top-candidate lemma right %.1f%%",
			c, s.total, 100*float64(s.right)/float64(s.total))
	}

	certain, uncertain := rate(CertainAnalysis), rate(UncertainAnalysis)
	if certain < 0 {
		t.Skip("no certain analyses in the sample")
	}
	// The floor is what the label promises. Below this the badge is decoration.
	if certain < 0.90 {
		t.Errorf("analyses labelled `certain` are right %.1f%% of the time; the label claims more",
			certain*100)
	}
	// `certain` and `uncertain` are the pair worth asserting on: both come out of
	// the dictionary, and the only difference is that `uncertain` picked between
	// homographs by frequency rather than knowing. If a frequency guess were as
	// good as a unique match, the badge would be telling the learner to distrust
	// something that deserves trust.
	if uncertain >= 0 && uncertain >= certain {
		t.Errorf("`uncertain` (%.1f%%) is not worse than `certain` (%.1f%%) — the badge sorts nothing",
			uncertain*100, certain*100)
	}

	// `derived` is measured and printed but deliberately NOT ranked against the
	// other two. It scores higher than `certain` on this corpus (98% on ~180
	// tokens), and that is a selection effect rather than a result: the suffix
	// fallback only ever runs on forms the dictionary missed, which in running
	// text are overwhelmingly transparent regular formations. Asserting an
	// ordering here would enshrine an artefact of which words are hard.
}
