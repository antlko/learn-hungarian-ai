package drill

import "testing"

// Parity with langpack/pipeline/harmony.py, which does the same job at build
// time. The cases are deliberately the same ones, because the two
// implementations disagreeing is a bug that would show up as the app marking a
// generated form wrong — and the whole point of the pack is that the two agree.

func TestClassify(t *testing.T) {
	cases := []struct {
		word string
		want Class
	}{
		{"ház", Back}, {"asztal", Back}, {"magyar", Back},
		{"kert", FrontUnrounded}, {"kenyér", FrontUnrounded},
		{"_", FrontUnrounded}, // no vowels at all
		{"köd", FrontRounded}, {"gyümölcs", FrontRounded}, {"tükör", FrontRounded},
		// Rounded then unrounded: the vowel nearest the suffix wins.
		{"könyvek", FrontUnrounded},
		// Neutral vowels must NOT override a back one. The classic error.
		{"papír", Back}, {"kocsi", Back}, {"Balaton", Back}, {"kávé", Back},
		// Antiharmonic: front-looking, back-behaving.
		{"híd", Back}, {"cél", Back}, {"víz", Back}, {"férj", Back},
		// `i`/`í` are transparent to rounding; `e`/`é` are not.
		{"könyv", FrontRounded}, {"tükrei", FrontUnrounded},
		{"ötven", FrontUnrounded}, {"öt", FrontRounded}, {"ing", FrontUnrounded},
	}
	for _, c := range cases {
		if got := Classify(c.word); got != c.want {
			t.Errorf("Classify(%q) = %v, want %v", c.word, got, c.want)
		}
	}
}

func TestClassifyIsCaseInsensitive(t *testing.T) {
	if Classify("BALATON") != Classify("balaton") {
		t.Error("classification must not depend on case")
	}
}

func TestPickTwoAndThreeFormSuffixes(t *testing.T) {
	ine := map[string]string{"back": "ban", "front_unrounded": "ben", "front_rounded": "ben"}
	all := map[string]string{"back": "hoz", "front_unrounded": "hez", "front_rounded": "höz"}

	if got := Pick(ine, FrontRounded); got != "ben" {
		t.Errorf("front rounded shares the front form of a two-form suffix, got %q", got)
	}
	for class, want := range map[Class]string{Back: "hoz", FrontUnrounded: "hez", FrontRounded: "höz"} {
		if got := Pick(all, class); got != want {
			t.Errorf("Pick(-hoz, %v) = %q, want %q", class, got, want)
		}
	}
}

// The examples from the source brief §2.2, end to end.
func TestRealSuffixSelection(t *testing.T) {
	ine := map[string]string{"back": "ban", "front_unrounded": "ben", "front_rounded": "ben"}
	all := map[string]string{"back": "hoz", "front_unrounded": "hez", "front_rounded": "höz"}
	abl := map[string]string{"back": "tól", "front_unrounded": "től", "front_rounded": "től"}

	for _, c := range []struct{ stem, ine, all, abl string }{
		{"ház", "ban", "hoz", "tól"},
		{"kert", "ben", "hez", "től"},
		{"köd", "ben", "höz", "től"},
	} {
		h := Classify(c.stem)
		if got := Pick(ine, h); got != c.ine {
			t.Errorf("%s inessive = %q, want %q", c.stem, got, c.ine)
		}
		if got := Pick(all, h); got != c.all {
			t.Errorf("%s allative = %q, want %q", c.stem, got, c.all)
		}
		if got := Pick(abl, h); got != c.abl {
			t.Errorf("%s ablative = %q, want %q", c.stem, got, c.abl)
		}
	}
}

// Regression: scanning backwards for *any* rounded vowel got this wrong, and it
// is a real alternation rather than an edge case.
func TestPluralFlipsTheSuffix(t *testing.T) {
	all := map[string]string{"back": "hoz", "front_unrounded": "hez", "front_rounded": "höz"}
	if got := Pick(all, Classify("könyv")); got != "höz" {
		t.Errorf("könyv takes -höz, got %q", got)
	}
	if got := Pick(all, Classify("könyvek")); got != "hez" {
		t.Errorf("könyvek takes -hez, got %q", got)
	}
}

func TestDistinctAllomorphs(t *testing.T) {
	two := map[string]string{"back": "ban", "front_unrounded": "ben", "front_rounded": "ben"}
	three := map[string]string{"back": "hoz", "front_unrounded": "hez", "front_rounded": "höz"}
	none := map[string]string{"back": "t", "front_unrounded": "t", "front_rounded": "t"}
	empty := map[string]string{"back": "", "front_unrounded": "", "front_rounded": ""}

	if got := Distinct(two); len(got) != 2 {
		t.Errorf("-ban/-ben has 2 distinct forms, got %v", got)
	}
	if got := Distinct(three); len(got) != 3 {
		t.Errorf("-hoz/-hez/-höz has 3 distinct forms, got %v", got)
	}
	// An invariant suffix cannot be an allomorph CHOICE, and a generator that
	// built an item from one would be asking a question with one right answer
	// offered three times.
	if got := Distinct(none); len(got) != 1 {
		t.Errorf("the accusative -t has 1 distinct form, got %v", got)
	}
	if got := Distinct(empty); len(got) != 0 {
		t.Errorf("a feature with no exponent has no allomorphs, got %v", got)
	}
}

func TestEndsInVowelAndLengthening(t *testing.T) {
	for word, want := range map[string]bool{"alma": true, "ház": false, "": false} {
		if got := EndsInVowel(word); got != want {
			t.Errorf("EndsInVowel(%q) = %v", word, got)
		}
	}
	for word, want := range map[string]string{
		"alma": "almá", "epe": "epé", "ház": "ház", "kocsi": "kocsi", "": "",
	} {
		if got := LengthenFinalVowel(word); got != want {
			t.Errorf("LengthenFinalVowel(%q) = %q, want %q", word, got, want)
		}
	}
}

// Automaticity is the motor-skill mastery test, and its failure modes matter
// more than its success: a learner who is accurate but slow has NOT automated
// the skill, and one with no measured latency has not demonstrated anything.
func TestMeasureAutomaticity(t *testing.T) {
	const budget, window = 2500, 50

	fastAndRight := MeasureAutomaticity(1800, 0.95, 60, window, budget, 0.90)
	if !fastAndRight.Automated {
		t.Error("fast, accurate and well-evidenced should count as automated")
	}

	slowButRight := MeasureAutomaticity(4000, 0.95, 60, window, budget, 0.90)
	if slowButRight.Automated {
		t.Error("correct but laboured is not automaticity — this is the whole of R15")
	}
	if !slowButRight.Accurate || slowButRight.Fast {
		t.Error("the verdict must say WHICH half failed, for the UI to explain it")
	}

	never := MeasureAutomaticity(0, 0.99, 60, window, budget, 0.90)
	if never.Automated || never.Fast {
		t.Error("no measured latency is not the same as being fast")
	}

	thin := MeasureAutomaticity(1200, 1.0, 4, window, budget, 0.90)
	if thin.Automated {
		t.Error("four fast answers are not evidence of an automated skill")
	}
}
