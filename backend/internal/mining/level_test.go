package mining

import "testing"

// The estimator's promises: the band the words demand, raised by the grammar
// the text uses, nudged by sentence length — and an honest refusal when too
// little of the text resolves. ADR-023.

func lvlTokens(n int, lemmaFrom int64, wordsPerSentence int) []TokenView {
	out := make([]TokenView, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, TokenView{
			Index: i, SentIdx: i / wordsPerSentence, IsWord: true,
			LemmaID: lemmaFrom + int64(i%10),
		})
	}
	return out
}

func inputs(cefr map[int64]string, topics map[string]string) LevelInputs {
	return LevelInputs{
		CEFRForLemma:  func(id int64) string { return cefr[id] },
		TopicLevelFor: func(f string) string { return topics[f] },
	}
}

func TestAPureA1TextIsA1(t *testing.T) {
	cefr := map[int64]string{}
	for i := int64(0); i < 10; i++ {
		cefr[100+i] = "A1"
	}
	est := EstimateLevel(lvlTokens(100, 100, 10), inputs(cefr, nil))
	if est.Level != "A1" || est.LexicalLevel != "A1" {
		t.Fatalf("pure A1 text judged %q (lexical %q)", est.Level, est.LexicalLevel)
	}
}

func TestGrammarCeilingRaisesTheVerdict(t *testing.T) {
	cefr := map[int64]string{}
	for i := int64(0); i < 10; i++ {
		cefr[100+i] = "A1"
	}
	toks := lvlTokens(100, 100, 10)
	// Conditional mood on four tokens: enough evidence for its topic's level.
	for i := 0; i < 4; i++ {
		toks[i].FeatureIDs = []string{"Mood=Cnd"}
	}
	est := EstimateLevel(toks, inputs(cefr, map[string]string{"Mood=Cnd": "B1"}))
	if est.Level != "B1" || est.GrammarLevel != "B1" {
		t.Fatalf("A1 words under B1 grammar judged %q (grammar %q)", est.Level, est.GrammarLevel)
	}

	// Two stray hits are not evidence: one conditional does not make a text B1.
	toks2 := lvlTokens(100, 100, 10)
	toks2[0].FeatureIDs = []string{"Mood=Cnd"}
	toks2[1].FeatureIDs = []string{"Mood=Cnd"}
	est = EstimateLevel(toks2, inputs(cefr, map[string]string{"Mood=Cnd": "B1"}))
	if est.Level != "A1" {
		t.Fatalf("two stray hits raised the verdict to %q", est.Level)
	}
}

func TestLongSentencesBumpOneStep(t *testing.T) {
	cefr := map[int64]string{}
	for i := int64(0); i < 10; i++ {
		cefr[100+i] = "A2"
	}
	est := EstimateLevel(lvlTokens(100, 100, 25), inputs(cefr, nil))
	if est.Level != "B1" {
		t.Fatalf("A2 words in 25-word sentences judged %q, want the one-step bump", est.Level)
	}
}

func TestAnUnresolvedTextRefusesAVerdict(t *testing.T) {
	toks := lvlTokens(100, 100, 10)
	for i := range toks {
		if i%3 != 0 {
			toks[i].LemmaID = 0 // two thirds unresolved
		}
	}
	est := EstimateLevel(toks, inputs(map[int64]string{}, nil))
	if est.Level != "" {
		t.Fatalf("an unresolvable text was judged %q; the honest answer is no answer", est.Level)
	}
	if est.Resolved >= 0.5 {
		t.Fatalf("resolved share = %v, setup broken", est.Resolved)
	}
}

func TestALemmaOffTheFrequencyListCountsAsC2(t *testing.T) {
	cefr := map[int64]string{}
	for i := int64(0); i < 10; i++ {
		if i < 8 {
			cefr[100+i] = "A1"
		}
		// two of ten lemmas have no band: rare by construction
	}
	est := EstimateLevel(lvlTokens(100, 100, 10), inputs(cefr, nil))
	// 80% A1 + 20% off-list: the 90% share is only reached at C2.
	if est.LexicalLevel != "C2" {
		t.Fatalf("off-list words judged lexical %q, want C2", est.LexicalLevel)
	}
}
