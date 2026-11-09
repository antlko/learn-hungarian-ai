package stats

import "testing"

func sizes() map[string]int {
	return map[string]int{"A1": 100, "A2": 100, "B1": 100, "B2": 100, "C1": 100}
}

func topics(level string, phases ...string) []TopicPhase {
	var out []TopicPhase
	for _, p := range phases {
		out = append(out, TopicPhase{Level: level, Drillable: true, Phase: p})
	}
	return out
}

func TestEstimateLevel(t *testing.T) {
	cases := []struct {
		name       string
		in         LevelInput
		label      string
		confidence string
		cappedBy   string
	}{
		{"empty deck", LevelInput{Sizes: sizes()}, "A1.1", ConfidenceLow, ""},
		{"no bands in pack", LevelInput{}, "A1.1", ConfidenceLow, ""},
		{"share 0.6 is three quarters of the way: .3",
			LevelInput{Sizes: sizes(), Counted: map[string]int{"A1": 60}, CountedLemmas: 60},
			"A1.3", ConfidenceLow, ""},
		{"share 0.2 is .1",
			LevelInput{Sizes: sizes(), Counted: map[string]int{"A1": 20}, CountedLemmas: 20},
			"A1.1", ConfidenceLow, ""},
		{"share 0.4 is .2",
			LevelInput{Sizes: sizes(), Counted: map[string]int{"A1": 40}, CountedLemmas: 40},
			"A1.2", ConfidenceLow, ""},
		{"share just under 0.80 stays in the band at .3",
			LevelInput{Sizes: sizes(), Counted: map[string]int{"A1": 79}, CountedLemmas: 79},
			"A1.3", ConfidenceLow, ""},
		{"share exactly 0.80 reaches A1, A2 is the working band",
			LevelInput{Sizes: sizes(), Counted: map[string]int{"A1": 80}, CountedLemmas: 80},
			"A2.1", ConfidenceLow, ""},
		{"estimate, not low, from 100 lemmas",
			LevelInput{Sizes: sizes(), Counted: map[string]int{"A1": 80, "A2": 20}, CountedLemmas: 100},
			"A2.1", ConfidenceEstimate, ""},
		{"vocabulary done, grammar missing: stays at .3",
			LevelInput{Sizes: sizes(), Counted: map[string]int{"A1": 95}, CountedLemmas: 95,
				Topics: topics("A1", "blocked", "blocked", "interleaved")},
			"A1.3", ConfidenceLow, ""},
		{"half the drillable topics is enough",
			LevelInput{Sizes: sizes(), Counted: map[string]int{"A1": 95}, CountedLemmas: 95,
				Topics: topics("A1", "blocked", "mastered")},
			"A2.1", ConfidenceLow, ""},
		{"rule-card-only topics never hold a band back",
			LevelInput{Sizes: sizes(), Counted: map[string]int{"A1": 95}, CountedLemmas: 95,
				Topics: []TopicPhase{{Level: "A1", Drillable: false, Phase: "explained"}}},
			"A2.1", ConfidenceLow, ""},
		{"failed m3 blocks A2 though vocabulary and grammar pass",
			LevelInput{Sizes: sizes(), Counted: map[string]int{"A1": 90, "A2": 90}, CountedLemmas: 180,
				Blocked: map[string]bool{"A2": true, "B1": true, "B2": true}},
			"A2.3", ConfidenceEstimate, "m3"},
		{"everything reached tops out at C1+",
			LevelInput{Sizes: sizes(), CountedLemmas: 500,
				Counted: map[string]int{"A1": 100, "A2": 100, "B1": 100, "B2": 100, "C1": 100}},
			"C1+", ConfidenceEstimate, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EstimateLevel(tc.in)
			if got.Label != tc.label || got.Confidence != tc.confidence || got.CappedBy != tc.cappedBy {
				t.Errorf("got %q/%s/capped %q, want %q/%s/capped %q",
					got.Label, got.Confidence, got.CappedBy, tc.label, tc.confidence, tc.cappedBy)
			}
			if got.MethodKey != LevelMethodKey || len(got.Bands) != len(LevelBands) {
				t.Errorf("methodKey %q, %d bands", got.MethodKey, len(got.Bands))
			}
		})
	}
}

// Grammar is named by the band, not guessed by the screen.
func TestEstimateLevelNamesGrammarAsTheLimit(t *testing.T) {
	got := EstimateLevel(LevelInput{Sizes: sizes(), Counted: map[string]int{"A1": 95}, CountedLemmas: 95,
		Topics: topics("A1", "blocked", "blocked")})
	a1 := got.Bands[0]
	if a1.GrammarReady || a1.Reached || a1.GrammarDrillable != 2 || a1.Share < 0.8 {
		t.Errorf("A1 = %+v", a1)
	}
}

func TestEstimateLevelCoverage(t *testing.T) {
	got := EstimateLevel(LevelInput{Sizes: sizes(), FreqCounted: 710, FreqTotal: 1000})
	if got.Coverage != 0.71 {
		t.Errorf("coverage = %v, want 0.71", got.Coverage)
	}
	if EstimateLevel(LevelInput{}).Coverage != 0 {
		t.Error("no frequency data must read 0, not NaN")
	}
}
