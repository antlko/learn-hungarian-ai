package morph

import "testing"

func words(tokens []Token) []string {
	var out []string
	for _, t := range tokens {
		if t.IsWord {
			out = append(out, t.Text)
		}
	}
	return out
}

func TestTokenizeKeepsHungarianLetters(t *testing.T) {
	got := words(Tokenize("A gyümölcsöt a házaimban ettük."))
	want := []string{"A", "gyümölcsöt", "a", "házaimban", "ettük"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTokenizeHyphenIsWordInternalOnlyBetweenLetters(t *testing.T) {
	// `van-e` is one word (the question clitic); a dangling hyphen is not.
	if got := words(Tokenize("Van-e kenyér?")); len(got) != 2 || got[0] != "Van-e" {
		t.Errorf("got %v, want [Van-e kenyér]", got)
	}
	if got := words(Tokenize("ház- és kertépítés")); got[0] != "ház" {
		t.Errorf("dangling hyphen should not join: got %v", got)
	}
}

func TestTokenOffsetsAreExactWithMultibyteRunes(t *testing.T) {
	text := "Őt láttam."
	tokens := Tokenize(text)
	for _, tok := range tokens {
		if got := text[tok.Start:tok.End]; got != tok.Text {
			t.Errorf("offsets wrong for %q: text[%d:%d] = %q", tok.Text, tok.Start, tok.End, got)
		}
	}
}

func TestSplitSentences(t *testing.T) {
	tokens := Tokenize("Ez egy ház. Az ott egy kert! Tényleg?")
	got := SplitSentences(tokens)
	if len(got) != 3 {
		t.Fatalf("want 3 sentences, got %d: %+v", len(got), got)
	}
}

func TestOrdinalDotDoesNotEndASentence(t *testing.T) {
	// Hungarian writes ordinals and dates with a trailing dot. Splitting on it
	// would shred every date in a news article.
	for _, text := range []string{
		"2026. augusztus 22. volt a határidő.",
		"A 3. emeleten lakom.",
		"1848. március 15.",
	} {
		tokens := Tokenize(text)
		if got := SplitSentences(tokens); len(got) != 1 {
			t.Errorf("%q split into %d sentences, want 1", text, len(got))
		}
	}
}

func TestSentencesCoverEveryToken(t *testing.T) {
	tokens := Tokenize("Első mondat. Második! Harmadik? Negyedik")
	sentences := SplitSentences(tokens)
	covered := 0
	for _, s := range sentences {
		if s.Start != covered {
			t.Fatalf("gap or overlap at %d (sentence starts %d)", covered, s.Start)
		}
		covered = s.End
	}
	if covered != len(tokens) {
		t.Errorf("only %d of %d tokens covered", covered, len(tokens))
	}
}

func TestPunctuationIsNotAWord(t *testing.T) {
	for _, tok := range Tokenize("Szia, világ!") {
		if tok.Text == "," || tok.Text == "!" {
			if tok.IsWord {
				t.Errorf("%q marked as a word", tok.Text)
			}
		}
	}
}

func TestEmptyAndWhitespaceOnly(t *testing.T) {
	if got := Tokenize(""); len(got) != 0 {
		t.Errorf("empty string produced %d tokens", len(got))
	}
	if got := Tokenize("   \n\t "); len(got) != 0 {
		t.Errorf("whitespace produced %d tokens", len(got))
	}
	if got := SplitSentences(nil); len(got) != 0 {
		t.Errorf("nil tokens produced %d sentences", len(got))
	}
}
