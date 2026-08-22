package morph

import "testing"

func TestExactMatch(t *testing.T) {
	d := DiffForms("házban", "házban", nil)
	if !d.Match || d.Class != NoError {
		t.Errorf("got %+v", d)
	}
	// Case and surrounding whitespace are not errors.
	if !DiffForms("  Házban ", "házban", nil).Match {
		t.Error("normalisation should absorb case and padding")
	}
}

func TestVowelLengthIsItsOwnErrorClass(t *testing.T) {
	// `hat` and `hát` are different words, so this is not a typo — and its
	// remedy is ear training, not more writing. Mislabelling it as a spelling
	// slip would send the learner to the wrong drill.
	for _, tc := range []struct{ typed, expected string }{
		{"hat", "hát"},
		{"tör", "tőr"},
		{"kor", "kór"},
		{"hazban", "házban"},
		{"ot", "ót"},
	} {
		d := DiffForms(tc.typed, tc.expected, nil)
		if d.Match {
			t.Errorf("%q vs %q reported as a match", tc.typed, tc.expected)
		}
		if d.Class != ErrVowelLength {
			t.Errorf("%q vs %q: class %q, want vowel_length", tc.typed, tc.expected, d.Class)
		}
		if d.NearMatch {
			t.Errorf("%q vs %q marked a near match; length is phonemic", tc.typed, tc.expected)
		}
	}
}

func TestHarmonyErrorNamesTheFeature(t *testing.T) {
	// The learner picked the right case and the wrong allomorph. That is a
	// motor-skill failure, and the feature it belongs to is what should be
	// drilled — so the diff has to name it.
	a := NewAnalyzer(newTestPack(t))

	d := DiffForms("házben", "házban", a)
	if d.Class != ErrHarmony {
		t.Fatalf("class: got %q, want harmony (%+v)", d.Class, d)
	}
	if d.WrongFeature != "Case=Ine" {
		t.Errorf("wrongFeature: got %q, want Case=Ine", d.WrongFeature)
	}
}

func TestThreeFormSuffixHarmonyError(t *testing.T) {
	a := NewAnalyzer(newTestPack(t))
	// `gyümölcshez` for `gyümölcshöz`: the rounded variant was needed.
	d := DiffForms("gyümölcshez", "gyümölcshöz", a)
	if d.Class != ErrHarmony || d.WrongFeature != "Case=All" {
		t.Errorf("got class %q feature %q", d.Class, d.WrongFeature)
	}
}

func TestWrongCaseIsDistinguishedFromWrongHarmony(t *testing.T) {
	// `házat` (accusative) for `házban` (inessive) is a different mistake from
	// `házben`, and needs the rule restated rather than more repetitions.
	a := NewAnalyzer(newTestPack(t))

	d := DiffForms("házat", "házban", a)
	if d.Class == ErrHarmony {
		t.Errorf("a wrong case was classified as a harmony slip: %+v", d)
	}
	if d.WrongFeature != "" && d.WrongFeature != "Case=Ine" {
		t.Errorf("wrongFeature: got %q", d.WrongFeature)
	}
}

func TestWrongWordIsAStemError(t *testing.T) {
	a := NewAnalyzer(newTestPack(t))
	d := DiffForms("kertben", "házban", a)
	if d.Class != ErrStem {
		t.Errorf("class: got %q, want stem (%+v)", d.Class, d)
	}
	if d.WrongFeature != "" {
		t.Error("a vocabulary failure must not blame a morphological feature")
	}
}

func TestSingleCharacterSlipIsANearMatch(t *testing.T) {
	// A doubled letter is a typing slip, not a knowledge failure, and should
	// cost less than a wrong answer.
	d := DiffForms("házbban", "házban", nil)
	if !d.NearMatch || d.Class != ErrSpelling {
		t.Errorf("got %+v", d)
	}
}

func TestEmptyAnswerIsNotANearMatch(t *testing.T) {
	d := DiffForms("", "házban", nil)
	if d.Match || d.NearMatch {
		t.Errorf("an empty answer should be plainly wrong: %+v", d)
	}
}

func TestCommonAffixesLocateTheDifferingSpan(t *testing.T) {
	d := DiffForms("házben", "házban", nil)
	// "házb" is shared, then e/a differs, then "n" is shared.
	if d.CommonPrefix != 4 {
		t.Errorf("commonPrefix: got %d, want 4 (házb)", d.CommonPrefix)
	}
	if d.CommonSuffix != 1 {
		t.Errorf("commonSuffix: got %d, want 1 (n)", d.CommonSuffix)
	}
}

func TestDiffDegradesWithoutAnalyzer(t *testing.T) {
	// The review loop must work before a language pack is installed.
	d := DiffForms("házben", "házban", nil)
	if d.Match {
		t.Error("should still detect the difference")
	}
	if d.WrongFeature != "" {
		t.Error("without a pack no feature can be named — and none should be invented")
	}
}

func TestHintsAreKeysNotProse(t *testing.T) {
	// Prose in the backend would mean one locale silently wins. The frontend
	// renders these keys, so both locales stay honest.
	d := DiffForms("hat", "hát", nil)
	if d.Hint == "" {
		t.Fatal("no hint emitted")
	}
	for _, r := range d.Hint {
		if r > 127 {
			t.Errorf("hint %q looks like prose, not a key", d.Hint)
			break
		}
	}
	if d.HintArgs["expected"] != "hát" {
		t.Errorf("hint args: got %v", d.HintArgs)
	}
}

func TestEditDistanceBound(t *testing.T) {
	cases := []struct {
		a, b string
		max  int
		want bool
	}{
		{"ház", "ház", 1, true},
		{"ház", "hááz", 1, true},
		{"ház", "kert", 1, false},
		{"házban", "házbban", 1, true},
		{"házban", "kertben", 1, false},
	}
	for _, tc := range cases {
		if got := editDistanceAtMost(tc.a, tc.b, tc.max); got != tc.want {
			t.Errorf("editDistanceAtMost(%q,%q,%d) = %v", tc.a, tc.b, tc.max, got)
		}
	}
}

func TestVowelQualityIsSeparateFromLength(t *testing.T) {
	// `tor` vs `tőr` differs in rounding AND length, so it is not a length
	// error — and it is certainly not a typo. Both classes route to ear
	// training, but to different minimal-pair contrasts.
	for _, tc := range []struct {
		typed, expected string
		want            ErrorClass
	}{
		{"tör", "tőr", ErrVowelLength},  // pure length
		{"tor", "tör", ErrVowelQuality}, // pure rounding
		{"tor", "tőr", ErrVowelQuality}, // both
		{"tud", "tüd", ErrVowelQuality},
	} {
		d := DiffForms(tc.typed, tc.expected, nil)
		if d.Class != tc.want {
			t.Errorf("%q vs %q: class %q, want %q", tc.typed, tc.expected, d.Class, tc.want)
		}
		if d.NearMatch {
			t.Errorf("%q vs %q marked a near match; vowel contrasts are phonemic",
				tc.typed, tc.expected)
		}
	}
}
