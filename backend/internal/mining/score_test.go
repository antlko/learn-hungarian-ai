package mining

import "testing"

// tok builds a word token.
func tok(i int, form string, lemma int64, feats ...string) TokenView {
	return TokenView{Index: i, Form: form, IsWord: true, LemmaID: lemma, FeatureIDs: feats}
}

// learner builds a Context where the given lemmas and features are known.
func learner(lemmas []int64, feats []string) Context {
	ls := map[int64]bool{}
	for _, l := range lemmas {
		ls[l] = true
	}
	fs := map[string]bool{}
	for _, f := range feats {
		fs[f] = true
	}
	return Context{
		KnownLemma:    func(id int64) bool { return ls[id] },
		KnownFeature:  func(f string) bool { return fs[f] },
		FrequencyRank: func(id int64) int { return int(id) * 100 },
	}
}

func sentence(n int, text string, tokens ...TokenView) Candidate {
	return Candidate{SentIdx: n, Text: text, Tokens: tokens}
}

func TestFeaturesCountAsUnknownElements(t *testing.T) {
	// The case the whole design exists for. `Elmentem volna.` uses only familiar
	// words but two unfamiliar grammatical facts. A word-counting scorer calls
	// it trivial; this one must not.
	ctx := learner([]int64{1, 2}, []string{"Tense=Past"})

	s := Score(sentence(0, "Elmentem volna.",
		tok(0, "Elmentem", 1, "Tense=Past", "Mood=Cnd"),
		tok(1, "volna", 2, "Mood=Cnd"),
	), ctx, DefaultParams())

	if len(s.UnknownLemmas) != 0 {
		t.Fatalf("both words are known; got unknown lemmas %v", s.UnknownLemmas)
	}
	if len(s.UnknownFeatures) == 0 {
		t.Fatal("the unknown MOOD was not counted — this is the bug the two state spaces exist to prevent")
	}
	if s.UnknownWeighted <= 0 {
		t.Errorf("weighted unknowns = %v, want > 0", s.UnknownWeighted)
	}
}

func TestSweetSpotBeatsBothExtremes(t *testing.T) {
	ctx := learner([]int64{1, 2, 3, 4, 5, 6, 7, 8}, []string{"Case=Nom"})
	p := DefaultParams()

	nothingNew := Score(sentence(0, "a b c d e f g",
		tok(0, "a", 1, "Case=Nom"), tok(1, "b", 2, "Case=Nom"), tok(2, "c", 3, "Case=Nom"),
		tok(3, "d", 4, "Case=Nom"), tok(4, "e", 5, "Case=Nom"), tok(5, "f", 6, "Case=Nom"),
		tok(6, "g", 7, "Case=Nom"),
	), ctx, p)

	justRight := Score(sentence(1, "a b c d e f x",
		tok(0, "a", 1, "Case=Nom"), tok(1, "b", 2, "Case=Nom"), tok(2, "c", 3, "Case=Nom"),
		tok(3, "d", 4, "Case=Nom"), tok(4, "e", 5, "Case=Nom"), tok(5, "f", 6, "Case=Nom"),
		tok(6, "x", 99, "Case=Nom"),
	), ctx, p)

	overwhelming := Score(sentence(2, "p q r s t u v",
		tok(0, "p", 91), tok(1, "q", 92), tok(2, "r", 93), tok(3, "s", 94),
		tok(4, "t", 95), tok(5, "u", 96), tok(6, "v", 97),
	), ctx, p)

	if justRight.Score <= nothingNew.Score {
		t.Errorf("a sentence with nothing new (%.2f) outranked one at n+1 (%.2f)",
			nothingNew.Score, justRight.Score)
	}
	if justRight.Score <= overwhelming.Score {
		t.Errorf("an overwhelming sentence (%.2f) outranked one at n+1 (%.2f)",
			overwhelming.Score, justRight.Score)
	}
}

func TestBelowTheCoverageFloorIsPenalisedNotMerelyUnrewarded(t *testing.T) {
	// Below ~90% this is decoding, not comprehensible input. It has to sort
	// below "nothing new", or a beginner's queue fills with impossible material.
	ctx := learner([]int64{1}, nil)
	p := DefaultParams()

	hopeless := Score(sentence(0, "x y z w",
		tok(0, "x", 91), tok(1, "y", 92), tok(2, "z", 93), tok(3, "w", 94),
	), ctx, p)

	if hopeless.Score > 0 {
		t.Errorf("a sentence at %.0f%% coverage scored %.2f, want <= 0",
			hopeless.Coverage*100, hopeless.Score)
	}
}

func TestFrequentUnknownWordsAreWorthMore(t *testing.T) {
	// Learning a word you will meet weekly beats learning one you will meet once.
	ctx := learner([]int64{1, 2, 3, 4, 5, 6}, []string{"Case=Nom"})
	p := DefaultParams()

	common := func(id int64) Scored {
		return Score(sentence(0, "a b c d e f new",
			tok(0, "a", 1, "Case=Nom"), tok(1, "b", 2, "Case=Nom"), tok(2, "c", 3, "Case=Nom"),
			tok(3, "d", 4, "Case=Nom"), tok(4, "e", 5, "Case=Nom"), tok(5, "f", 6, "Case=Nom"),
			tok(6, "new", id, "Case=Nom"),
		), ctx, p)
	}
	// Both lemmas must be UNKNOWN, or the sentences differ in how much is new
	// and the sweet-spot term dominates. FrequencyRank is id*100, so lemma 20
	// is rank 2000 and lemma 400 is rank 40000.
	frequent, rare := common(20), common(400)
	if frequent.UnknownWeighted != rare.UnknownWeighted {
		t.Fatalf("test setup: the two sentences differ in novelty (%.2f vs %.2f)",
			frequent.UnknownWeighted, rare.UnknownWeighted)
	}
	if frequent.Score <= rare.Score {
		t.Errorf("a rare new word (%.3f) outranked a frequent one (%.3f)",
			rare.Score, frequent.Score)
	}
}

func TestCurrentTopicRaisesRelevance(t *testing.T) {
	base := learner([]int64{1, 2, 3, 4, 5, 6}, []string{"Case=Nom"})
	focused := base
	focused.TopicFeatures = map[string]bool{"Case=Ine": true}
	p := DefaultParams()

	s := sentence(0, "a b c d e f g",
		tok(0, "a", 1, "Case=Nom"), tok(1, "b", 2, "Case=Nom"), tok(2, "c", 3, "Case=Nom"),
		tok(3, "d", 4, "Case=Nom"), tok(4, "e", 5, "Case=Nom"), tok(5, "f", 6, "Case=Nom"),
		tok(6, "g", 7, "Case=Ine"),
	)
	if Score(s, focused, p).Score <= Score(s, base, p).Score {
		t.Error("a sentence exercising the current topic should outrank the same sentence when it is not being studied")
	}
}

func TestDuplicatesArePenalised(t *testing.T) {
	ctx := learner([]int64{1, 2, 3, 4, 5, 6}, []string{"Case=Nom"})
	p := DefaultParams()
	s := sentence(0, "A ház  nagy.", tok(0, "A", 1, "Case=Nom"), tok(1, "ház", 7, "Case=Nom"))

	fresh := Score(s, ctx, p)
	// Whitespace and case must not defeat the check.
	ctx.SeenText = map[string]bool{"a ház nagy.": true}
	repeat := Score(s, ctx, p)

	if repeat.Score >= fresh.Score {
		t.Errorf("a duplicate (%.2f) was not penalised against a fresh one (%.2f)",
			repeat.Score, fresh.Score)
	}
}

func TestLengthWindow(t *testing.T) {
	p := DefaultParams()
	if lengthFit(10, p.MinLength, p.MaxLength) != 1 {
		t.Error("a sentence inside the window should fit perfectly")
	}
	if lengthFit(2, p.MinLength, p.MaxLength) >= 1 {
		t.Error("a two-word sentence carries too little context")
	}
	if lengthFit(40, p.MinLength, p.MaxLength) >= lengthFit(15, p.MinLength, p.MaxLength) {
		t.Error("a 40-word sentence should fit worse than a 15-word one")
	}
	if got := lengthFit(500, p.MinLength, p.MaxLength); got < 0 {
		t.Errorf("length fit went negative: %v", got)
	}
}

func TestRankOrdersAndTrims(t *testing.T) {
	ctx := learner([]int64{1, 2, 3, 4, 5, 6}, []string{"Case=Nom"})
	known := []TokenView{
		tok(0, "a", 1, "Case=Nom"), tok(1, "b", 2, "Case=Nom"), tok(2, "c", 3, "Case=Nom"),
		tok(3, "d", 4, "Case=Nom"), tok(4, "e", 5, "Case=Nom"), tok(5, "f", 6, "Case=Nom"),
	}
	cands := []Candidate{
		{SentIdx: 0, Text: "all known", Tokens: known},
		{SentIdx: 1, Text: "one new", Tokens: append(append([]TokenView{}, known...), tok(6, "x", 20, "Case=Nom"))},
		{SentIdx: 2, Text: "impossible", Tokens: []TokenView{tok(0, "p", 91), tok(1, "q", 92)}},
	}

	got := Rank(cands, ctx, DefaultParams(), 10)
	if len(got) == 0 {
		t.Fatal("everything was filtered out")
	}
	if got[0].SentIdx != 1 {
		t.Errorf("best candidate is sentence %d, want 1 (the n+1 one)", got[0].SentIdx)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Score < got[i].Score {
			t.Error("results are not sorted by score")
		}
	}
	for _, s := range got {
		if s.SentIdx == 2 {
			t.Error("the impossible sentence should have been dropped")
		}
	}

	if trimmed := Rank(cands, ctx, DefaultParams(), 1); len(trimmed) != 1 {
		t.Errorf("limit ignored: got %d", len(trimmed))
	}
}

func TestCoverageReportsBothNumbers(t *testing.T) {
	// Lemma coverage overstates readability for an agglutinative language: a
	// learner can know every word and still be stopped by the suffixes.
	known := map[int64]string{1: "known", 2: "known"}
	knownFeat := func(f string) bool { return f == "Case=Nom" }

	c := Compute([]TokenView{
		tok(0, "ház", 1, "Case=Nom"),
		tok(1, "házaimban", 2, "Case=Ine", "Person[psor]=1"),
	}, known, knownFeat)

	if c.LemmaCoverage != 1.0 {
		t.Errorf("lemma coverage: got %.2f, want 1.00 — both words are known", c.LemmaCoverage)
	}
	if c.ElementCoverage >= 1.0 {
		t.Errorf("element coverage: got %.2f — the unknown suffixes were not counted",
			c.ElementCoverage)
	}
	if len(c.UnknownFeatures) != 2 {
		t.Errorf("unknown features: got %v", c.UnknownFeatures)
	}
}

func TestCoverageVerdicts(t *testing.T) {
	cases := []struct {
		element float64
		want    Verdict
	}{
		{0.99, VerdictComfortable},
		{0.96, VerdictWithHelp},
		{0.80, VerdictTooHard},
	}
	for _, tc := range cases {
		got := Coverage{ElementCoverage: tc.element}.Judge()
		if got != tc.want {
			t.Errorf("element coverage %.2f judged %q, want %q", tc.element, got, tc.want)
		}
	}
}

func TestUnresolvedTokensDoNotCountAsKnown(t *testing.T) {
	// A token the pack cannot analyse is not evidence of knowledge OR of a gap.
	c := Compute([]TokenView{
		tok(0, "ház", 1, "Case=Nom"),
		{Index: 1, Form: "Kovácsné", IsWord: true},
	}, map[int64]string{1: "known"}, func(string) bool { return true })

	if c.Unresolved != 1 {
		t.Errorf("unresolved: got %d, want 1", c.Unresolved)
	}
	if c.LemmaCoverage != 0.5 {
		t.Errorf("lemma coverage: got %.2f, want 0.50", c.LemmaCoverage)
	}
}

func TestShortNPlusOneSentencesAreNotPenalisedByTheReadingThreshold(t *testing.T) {
	// Regression: the scorer used the 90% READING threshold as its floor. A
	// seven-word sentence with one unknown word is 86% coverage, so every
	// short n+1 sentence — exactly what the scorer exists to find — was being
	// penalised, and "nothing new" outranked it.
	ctx := learner([]int64{1, 2, 3, 4, 5, 6}, []string{"Case=Nom"})
	p := DefaultParams()

	if p.MinCoverage >= MinimalComprehension {
		t.Fatalf("the mining floor (%.2f) must sit below the reading threshold (%.2f)",
			p.MinCoverage, MinimalComprehension)
	}

	base := []TokenView{
		tok(0, "a", 1, "Case=Nom"), tok(1, "b", 2, "Case=Nom"), tok(2, "c", 3, "Case=Nom"),
		tok(3, "d", 4, "Case=Nom"), tok(4, "e", 5, "Case=Nom"), tok(5, "f", 6, "Case=Nom"),
	}
	oneNew := Score(sentence(0, "one new",
		append(append([]TokenView{}, base...), tok(6, "x", 20, "Case=Nom"))...), ctx, p)

	if oneNew.Coverage >= MinimalComprehension {
		t.Fatalf("test setup: coverage %.2f is not below the reading threshold", oneNew.Coverage)
	}
	if oneNew.Score <= 0 {
		t.Errorf("a short n+1 sentence scored %.2f", oneNew.Score)
	}
}
