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
	const text = "Ez egy ház. Az ott egy kert! Tényleg?"
	tokens := Tokenize(text)
	got := SplitSentences(text, tokens)
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
		if got := SplitSentences(text, tokens); len(got) != 1 {
			t.Errorf("%q split into %d sentences, want 1", text, len(got))
		}
	}
}

func TestSentencesCoverEveryToken(t *testing.T) {
	const text = "Első mondat. Második! Harmadik? Negyedik"
	tokens := Tokenize(text)
	sentences := SplitSentences(text, tokens)
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
	if got := SplitSentences("", nil); len(got) != 0 {
		t.Errorf("nil tokens produced %d sentences", len(got))
	}
}

// A heading carries no terminal punctuation, so without a line-break rule it
// welded itself to the sentence below. A card mined from the first line of a
// learner's text then asked about `Egy átlagos nap Budapesten Reggel hét órakor
// ____.` — a title and a sentence run together, which is not something any
// Hungarian speaker wrote.
func TestALineBreakEndsASentence(t *testing.T) {
	const text = "Egy átlagos nap Budapesten\n\nReggel hét órakor felkelek."
	tokens := Tokenize(text)
	got := SplitSentences(text, tokens)
	if len(got) != 2 {
		t.Fatalf("want the title and the sentence apart, got %d: %+v", len(got), got)
	}
	if first := tokens[got[0].End-1].Text; first != "Budapesten" {
		t.Errorf("the title runs to %q, want it to end at Budapesten", first)
	}
}

// Every line of a dialogue or a list is its own unit even when it ends in a
// comma — or in nothing at all.
func TestEachLineOfAListIsItsOwnSentence(t *testing.T) {
	const text = "Bevásárlólista\nkenyér\nTej\nSajt"
	tokens := Tokenize(text)
	if got := SplitSentences(text, tokens); len(got) != 3 {
		// `kenyér` is lowercase, so it stays with the heading above it: a
		// single newline alone is not evidence of a boundary.
		t.Errorf("split into %d, want 3: %+v", len(got), got)
	}
}

// The failure the capital-letter guard exists to prevent. The reader invites
// the learner to paste subtitles, which wrap mid-sentence; splitting on every
// newline would cut those sentences in half and mine cards from the halves.
func TestAHardWrappedSentenceIsNotSplit(t *testing.T) {
	for _, text := range []string{
		"Nem tudom, hogy mit\ncsináljak most.",
		"A házban van egy\nnagy könyv.",
	} {
		tokens := Tokenize(text)
		if got := SplitSentences(text, tokens); len(got) != 1 {
			t.Errorf("%q split into %d sentences, want 1: %+v", text, len(got), got)
		}
	}
}

// A blank line is a paragraph break and always ends the sentence, whatever
// case the next line starts in.
func TestABlankLineAlwaysEndsASentence(t *testing.T) {
	const text = "első bekezdés\n\nmásodik bekezdés"
	tokens := Tokenize(text)
	if got := SplitSentences(text, tokens); len(got) != 2 {
		t.Errorf("split into %d, want 2: %+v", len(got), got)
	}
}

// A line break straight after a full stop must not produce an empty sentence
// between the two boundaries.
func TestALineBreakAfterAFullStopMakesNoEmptySentence(t *testing.T) {
	const text = "Első mondat.\nMásodik mondat."
	tokens := Tokenize(text)
	got := SplitSentences(text, tokens)
	if len(got) != 2 {
		t.Fatalf("split into %d, want 2: %+v", len(got), got)
	}
	for _, s := range got {
		if s.End <= s.Start {
			t.Errorf("empty sentence %+v", s)
		}
	}
}
