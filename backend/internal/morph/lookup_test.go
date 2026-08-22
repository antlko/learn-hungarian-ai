package morph

import "testing"

func TestLookupUnambiguousFormIsCertain(t *testing.T) {
	a := NewAnalyzer(newTestPack(t))

	got, err := a.Best("házban")
	if err != nil {
		t.Fatal(err)
	}
	if got.Lemma != "ház" {
		t.Errorf("lemma: got %q, want ház", got.Lemma)
	}
	if got.Certainty != CertainAnalysis {
		t.Errorf("certainty: got %q, want certain", got.Certainty)
	}
	if got.Feats["Case"] != "Ine" {
		t.Errorf("feats: got %v", got.Feats)
	}
	if len(got.FeatureID) == 0 || got.FeatureID[0] != "Case=Ine" {
		t.Errorf("featureIds: got %v, want Case=Ine first", got.FeatureID)
	}
}

func TestAmbiguousFormIsMarkedUncertainAndPrefersTheFrequentLemma(t *testing.T) {
	// `vár` is both a noun (castle) and a verb (waits). Without a sidecar we
	// cannot disambiguate in context, so the app must SAY it is unsure rather
	// than present a coin-flip as fact.
	a := NewAnalyzer(newTestPack(t))

	all, err := a.Lookup("vár")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 2 {
		t.Fatalf("want both readings, got %d", len(all))
	}
	for _, an := range all {
		if an.Certainty != UncertainAnalysis {
			t.Errorf("%s reading marked %q, want uncertain", an.UPOS, an.Certainty)
		}
		if an.Alternatives != 1 {
			t.Errorf("alternatives: got %d, want 1", an.Alternatives)
		}
	}
	// The frequency prior picks the verb (rank 400) over the noun (rank 2500).
	best, _ := a.Best("vár")
	if best.UPOS != "VERB" {
		t.Errorf("frequency prior should pick the VERB reading, got %s", best.UPOS)
	}
}

func TestGeneratedFormIsMarkedDerived(t *testing.T) {
	a := NewAnalyzer(newTestPack(t))

	got, _ := a.Best("házaimban")
	if got.Certainty != DerivedAnalysis {
		t.Errorf("certainty: got %q, want derived", got.Certainty)
	}
	// A derived reading still has to carry every feature, or the second state
	// space cannot credit them.
	for _, want := range []string{"Case=Ine", "Number[psed]=Plur", "Person[psor]=1"} {
		found := false
		for _, id := range got.FeatureID {
			if id == want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing feature %s in %v", want, got.FeatureID)
		}
	}
}

func TestLookupIsCaseInsensitive(t *testing.T) {
	a := NewAnalyzer(newTestPack(t))
	got, _ := a.Best("Házban")
	if got.Lemma != "ház" {
		t.Errorf("sentence-initial capital broke lookup: got %q", got.Lemma)
	}
}

func TestUnknownFormIsMarkedUnknownNotGuessed(t *testing.T) {
	a := NewAnalyzer(newTestPack(t))
	got, _ := a.Best("qwertyuiop")
	if got.Certainty != UnknownAnalysis {
		t.Errorf("certainty: got %q, want unknown", got.Certainty)
	}
	if len(got.Feats) != 0 {
		t.Errorf("an unknown form must not carry invented features: %v", got.Feats)
	}
}

func TestSuffixFallbackReachesUnlistedForms(t *testing.T) {
	// `gyümölcshöz` is not in the fixture's wordform table. Stripping the
	// three-form allative suffix must recover it -- and mark it derived,
	// because a stripped analysis is a guess.
	a := NewAnalyzer(newTestPack(t))

	got, err := a.Best("gyümölcshöz")
	if err != nil {
		t.Fatal(err)
	}
	if got.Lemma != "gyümölcs" {
		t.Fatalf("fallback failed: got lemma %q, certainty %q", got.Lemma, got.Certainty)
	}
	if got.Certainty != DerivedAnalysis {
		t.Errorf("certainty: got %q, want derived", got.Certainty)
	}
	if got.Feats["Case"] != "All" {
		t.Errorf("feats: got %v, want Case=All", got.Feats)
	}
}

func TestAnalyzeTextComputesCoverageAndSentences(t *testing.T) {
	a := NewAnalyzer(newTestPack(t))

	tokens, sentences, err := a.AnalyzeText("A házban. Egy kertben!")
	if err != nil {
		t.Fatal(err)
	}
	if len(sentences) != 2 {
		t.Errorf("want 2 sentences, got %d", len(sentences))
	}

	var known, words int
	for _, tok := range tokens {
		if !tok.IsWord {
			continue
		}
		words++
		if tok.Analysis.Certainty != UnknownAnalysis {
			known++
		}
	}
	if words != 4 {
		t.Errorf("want 4 words, got %d", words)
	}
	// `házban` and `kertben` resolve; `A` and `Egy` are not in the fixture.
	if known < 2 {
		t.Errorf("only %d/%d words resolved", known, words)
	}
}

func TestAnalyzeTextPreservesOffsets(t *testing.T) {
	a := NewAnalyzer(newTestPack(t))
	text := "Őt a házban láttam."
	tokens, _, err := a.AnalyzeText(text)
	if err != nil {
		t.Fatal(err)
	}
	for _, tok := range tokens {
		if got := text[tok.Start:tok.End]; got != tok.Text {
			t.Errorf("offset mismatch: text[%d:%d]=%q, token=%q",
				tok.Start, tok.End, got, tok.Text)
		}
	}
}

func TestNilAnalyzerIsSafe(t *testing.T) {
	// The server starts without a pack; nothing may panic on that path.
	var a *Analyzer
	if got, err := a.Lookup("ház"); err != nil || got != nil {
		t.Errorf("nil analyzer: got (%v, %v)", got, err)
	}
}

func TestSuffixTableIsPerAnalyzerNotPerProcess(t *testing.T) {
	// Regression: a package-level sync.Once meant the first pack opened in a
	// process supplied the suffix table for every later one — so a pack upgrade
	// would keep analysing against the old morphology, silently.
	a1 := NewAnalyzer(newTestPack(t))
	if _, err := a1.Best("gyümölcshöz"); err != nil {
		t.Fatal(err)
	}

	a2 := NewAnalyzer(newTestPack(t))
	got, err := a2.Best("gyümölcshöz")
	if err != nil {
		t.Fatal(err)
	}
	if got.Lemma != "gyümölcs" {
		t.Errorf("second analyzer did not load its own suffix table: got %q", got.Lemma)
	}
}

// The fallback analyser on deliberately out-of-vocabulary forms. Its contract:
// derive when a known lemma survives the peel, refuse when nothing does — and
// everything it produces is marked derived, never certain.
func TestFallbackOnOutOfVocabularyForms(t *testing.T) {
	p := newTestPack(t)
	a := NewAnalyzer(p)

	cases := []struct {
		form      string
		wantLemma string // "" = must stay unknown
	}{
		// kertnek is not in the wordform table; kert is a lemma; -nek peels.
		{"kertnek", "kert"},
		// Two layers: kertekben = kert + ek?? — 'ek' is not a stored allomorph
		// ('k' is), so the peel goes ben → kertek, k → kerte — not the lemma.
		// Depth-2 with linking vowels is out of the fallback's reach on purpose;
		// it must refuse rather than guess.
		{"kertekben", ""},
		// A proper noun the pack has never seen: no peel reaches a lemma.
		{"Kovácsnak", ""},
		// Pure noise.
		{"zzzzqqq", ""},
	}
	for _, tc := range cases {
		an, err := a.Best(tc.form)
		if err != nil {
			t.Fatal(err)
		}
		if tc.wantLemma == "" {
			if an.Certainty != UnknownAnalysis {
				t.Errorf("%s: certainty = %s (%+v), want unknown — refusing beats guessing",
					tc.form, an.Certainty, an)
			}
			continue
		}
		if an.Lemma != tc.wantLemma {
			t.Errorf("%s: lemma = %q, want %q", tc.form, an.Lemma, tc.wantLemma)
		}
		if an.Certainty != DerivedAnalysis {
			t.Errorf("%s: a fallback analysis must be marked derived, got %s", tc.form, an.Certainty)
		}
	}
}
