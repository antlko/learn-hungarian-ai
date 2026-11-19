package writing

import (
	"strings"
	"testing"
)

const block = `TANULOM-CHECK 1
LEVEL: A2
SUMMARY: Good text.
STRENGTH: Clear order.
STRENGTH: Nice words.

[ERROR]
QUOTE: boltban mentem
NTH: 1
TYPE: case
FIX: boltba mentem
HINT: Where to?
WHY: -ban is location,
-ba is direction.
RULE: Illative. Movement inside.
EXAMPLE: A parkba megyek. — I go to the park.

[ERROR]
QUOTE: egy kenyér
NTH: 2
TYPE: case
FIX: egy kenyeret
HINT: Object?
WHY: Needs -t.
RULE: Accusative.
EXAMPLE: Levelet írok. — I write a letter.

[VOCAB]
- vásárol — more formal than vesz
2. étterem — restaurant

[CORRECTED]
Tegnap a boltba mentem.

Vettem egy kenyeret.

[END]
`

func checkFull(t *testing.T, r Result) {
	t.Helper()
	if r.Status != StatusOK || r.Truncated || r.Dropped != 0 || r.NoHeader || r.Version != 1 {
		t.Fatalf("meta = %+v", r)
	}
	if r.Level != "A2" || r.Summary != "Good text." || len(r.Strengths) != 2 {
		t.Errorf("head = %q %q %v", r.Level, r.Summary, r.Strengths)
	}
	if len(r.Errors) != 2 {
		t.Fatalf("errors = %d", len(r.Errors))
	}
	e := r.Errors[0]
	if e.N != 1 || e.Quote != "boltban mentem" || e.Fix != "boltba mentem" || e.Type != "case" || e.Nth != 1 ||
		e.Why != "-ban is location,\n-ba is direction." || !strings.HasPrefix(e.Example, "A parkba") {
		t.Errorf("error 1 = %+v", e)
	}
	if r.Errors[1].N != 2 || r.Errors[1].Nth != 2 {
		t.Errorf("error 2 = %+v", r.Errors[1])
	}
	if len(r.Vocab) != 2 || r.Vocab[0] != "vásárol — more formal than vesz" || r.Vocab[1] != "étterem — restaurant" {
		t.Errorf("vocab = %q", r.Vocab)
	}
	if r.Corrected != "Tegnap a boltba mentem.\n\nVettem egy kenyeret." {
		t.Errorf("corrected = %q", r.Corrected)
	}
}

func TestParseForgivingInputs(t *testing.T) {
	bold := strings.NewReplacer("QUOTE:", "**QUOTE:**", "FIX:", "**FIX:**", "TYPE:", "**TYPE:**", "WHY:", "- **WHY**:", "SUMMARY:", "**SUMMARY**:").Replace(block)
	cases := map[string]string{
		"raw block":     block,
		"fenced":        "```text\n" + block + "```",
		"chatter":       "Sure! Here is the review:\n\n```text\n" + block + "```\n\nHope this helps! Summary: none.",
		"other fence":   "```\n" + block + "```",
		"no fence":      "Here you go:\n" + block + "\nLet me know.",
		"chatter first": "SUMMARY of what I did: nothing\n```python\nprint(1)\n```\n```text\n" + block + "```",
		"bold keys":     bold,
		"CRLF":          strings.ReplaceAll(block, "\n", "\r\n"),
		"BOM":           "\xef\xbb\xbf" + block,
		"think":         "<think>hmm [ERROR] QUOTE: x</think>\n" + block,
		"missing close": "```text\n" + block,
		"lowercase":     strings.NewReplacer("QUOTE:", "quote:", "FIX:", "Fix:", "[ERROR]", "[error]").Replace(block),
		"numbered hdr":  strings.Replace(block, "[ERROR]", "### [ERROR 1]", 1),
		"bullets":       strings.NewReplacer("QUOTE:", "- QUOTE:", "NTH:", "- NTH:", "TYPE:", "• TYPE:", "FIX:", "* FIX:").Replace(block),
		"fullwidth":     strings.ReplaceAll(block, "QUOTE:", "QUOTE："),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) { checkFull(t, Parse(in)) })
	}
}

func TestParseRule1NFC(t *testing.T) {
	r := Parse("TANULOM-CHECK 1\n[ERROR]\nQUOTE: kávé\nFIX: kávét\n[END]")
	if len(r.Errors) != 1 || r.Errors[0].Quote != "kávé" {
		t.Errorf("not NFC: %+v", r.Errors)
	}
}

func TestParseRule2Header(t *testing.T) {
	r := Parse("[ERROR]\nQUOTE: a\nFIX: b\n[END]")
	if !r.NoHeader || r.Status != StatusPartial || len(r.Errors) != 1 || r.Version != 0 {
		t.Errorf("no header: %+v", r)
	}
	r = Parse("Some chatter: yes\nSUMMARY: Hi\n[END]")
	if !r.NoHeader || r.Summary != "Hi" {
		t.Errorf("start at first key: %+v", r)
	}
	r = Parse("TANULOM-CHECK 2\nSUMMARY: x\n[END]")
	if r.Version != 2 || r.Status != StatusOK || r.Summary != "x" {
		t.Errorf("version 2: %+v", r)
	}
}

func TestParseRule3EndStops(t *testing.T) {
	r := Parse("TANULOM-CHECK 1\n[ERROR]\nQUOTE: a\nFIX: b\n[END]\n[ERROR]\nQUOTE: c\nFIX: d\n")
	if len(r.Errors) != 1 || r.Truncated {
		t.Errorf("after END: %+v", r)
	}
	r = Parse("TANULOM-CHECK 1\n[ERROR]\nQUOTE: a\nFIX: b\n")
	if !r.Truncated || r.Status != StatusPartial || len(r.Errors) != 1 {
		t.Errorf("no END: %+v", r)
	}
}

func TestParseRule4KeysIgnoredInCorrectedAndVocab(t *testing.T) {
	r := Parse("TANULOM-CHECK 1\n[VOCAB]\nFIX: not a key\n[CORRECTED]\nSUMMARY: part of the text\nQUOTE: too\n[END]")
	if len(r.Errors) != 0 || r.Summary != "" || len(r.Vocab) != 1 || r.Vocab[0] != "FIX: not a key" {
		t.Errorf("vocab/corrected: %+v", r)
	}
	if r.Corrected != "SUMMARY: part of the text\nQUOTE: too" {
		t.Errorf("corrected = %q", r.Corrected)
	}
}

func TestParseRule5Continuation(t *testing.T) {
	r := Parse("TANULOM-CHECK 1\nSUMMARY: one\ntwo\n\nthree\nSTRENGTH: a\nb\n[END]")
	if r.Summary != "one\ntwo\nthree" || len(r.Strengths) != 1 || r.Strengths[0] != "a\nb" {
		t.Errorf("continuation: %q %q", r.Summary, r.Strengths)
	}
}

func TestParseRule6Normalising(t *testing.T) {
	cases := []struct {
		nth, typ string
		wantNth  int
		wantType string
	}{
		{"3", "vowel length", 3, "vowel_length"}, {"abc", "Case (illative)", 1, "case"},
		{"0", "nonsense", 1, "other"}, {"-2", "", 1, "other"}, {"2.", "Missing-Word", 2, "missing_word"},
	}
	for _, c := range cases {
		r := Parse("TANULOM-CHECK 1\n[ERROR]\nQUOTE: «ház»\nNTH: " + c.nth + "\nTYPE: " + c.typ + "\nFIX: \"házba\"\n[END]")
		if len(r.Errors) != 1 {
			t.Fatalf("%+v", r)
		}
		e := r.Errors[0]
		if e.Quote != "ház" || e.Fix != "házba" || e.Nth != c.wantNth || e.Type != c.wantType {
			t.Errorf("%v -> %+v", c, e)
		}
	}
	r := Parse("TANULOM-CHECK 1\n[ERROR]\nQUOTE: `ház`\nFIX: ‘házba’\n[END]")
	if r.Errors[0].Quote != "ház" || r.Errors[0].Fix != "házba" {
		t.Errorf("backticks/quotes: %+v", r.Errors[0])
	}
}

func TestParseRule7Dropped(t *testing.T) {
	in := "TANULOM-CHECK 1\n[ERROR]\nFIX: x\n\n[ERROR]\nQUOTE: a\n\n[ERROR]\nQUOTE: extra b\nTYPE: extra_word\nFIX:\n\n[ERROR]\nQUOTE: ok\nFIX: fine\n[END]"
	r := Parse(in)
	if r.Dropped != 2 || len(r.Errors) != 2 || r.Status != StatusPartial {
		t.Fatalf("%+v", r)
	}
	if r.Errors[0].Type != "extra_word" || r.Errors[0].Fix != "" || r.Errors[0].N != 1 || r.Errors[1].N != 2 {
		t.Errorf("errors = %+v", r.Errors)
	}
}

func TestParseTruncatedMidError(t *testing.T) {
	r := Parse("```text\nTANULOM-CHECK 1\nSUMMARY: s\n[ERROR]\nQUOTE: a\nFIX: b\nWHY: ok\n\n[ERROR]\nQUOTE: c\nTYPE: ca")
	if r.Status != StatusPartial || !r.Truncated || len(r.Errors) != 1 || r.Dropped != 1 || r.Summary != "s" {
		t.Errorf("%+v", r)
	}
}

func TestParseImplicitErrorBlocks(t *testing.T) {
	// No [ERROR] headers: a repeated QUOTE opens a new block.
	r := Parse("TANULOM-CHECK 1\nQUOTE: a\nFIX: b\nQUOTE: c\nFIX: d\n[END]")
	if len(r.Errors) != 2 || r.Errors[1].Quote != "c" {
		t.Errorf("%+v", r.Errors)
	}
}

func TestParseRule8Caps(t *testing.T) {
	var b strings.Builder
	b.WriteString("TANULOM-CHECK 1\n")
	for i := 0; i < 7; i++ {
		b.WriteString("STRENGTH: s\n")
	}
	for i := 0; i < 65; i++ {
		b.WriteString("[ERROR]\nQUOTE: a\nFIX: b\n")
	}
	b.WriteString("[ERROR]\nQUOTE: long\nFIX: b\nWHY: " + strings.Repeat("é", 2500) + "\n[VOCAB]\n")
	for i := 0; i < 30; i++ {
		b.WriteString("- w\n")
	}
	b.WriteString("[CORRECTED]\n" + strings.Repeat("x", 13000) + "\n[END]\n")
	r := Parse(b.String())
	if len(r.Errors) != 60 || r.Dropped != 6 || len(r.Strengths) != 5 || len(r.Vocab) != 20 || r.Status != StatusPartial {
		t.Errorf("errors=%d dropped=%d strengths=%d vocab=%d", len(r.Errors), r.Dropped, len(r.Strengths), len(r.Vocab))
	}
	if n := len([]rune(r.Corrected)); n != maxCorrectedRun {
		t.Errorf("corrected runes = %d", n)
	}
	// the over-long WHY sits in error 66, which is dropped; check the field cap on a kept one.
	r = Parse("TANULOM-CHECK 1\n[ERROR]\nQUOTE: a\nFIX: b\nWHY: " + strings.Repeat("é", 2500) + "\n[END]")
	if n := len([]rune(r.Errors[0].Why)); n != maxFieldRunes {
		t.Errorf("why runes = %d", n)
	}
}

func TestParseRule9Status(t *testing.T) {
	for name, in := range map[string]string{
		"empty": "", "prose": "I'm sorry, I cannot help with that.", "json": `{"errors": []}`,
		"markdown": "## Errors\n1. **boltban** should be boltba",
	} {
		if r := Parse(in); r.Status != StatusFailed {
			t.Errorf("%s: status %q", name, r.Status)
		}
	}
	// A header alone with no keys is not an answer either.
	if r := Parse("TANULOM-CHECK 1"); r.Status != StatusFailed {
		t.Errorf("header only: %q", r.Status)
	}
	// No-error answer.
	r := Parse("```text\nTANULOM-CHECK 1\nLEVEL: B1\nSUMMARY: No errors found.\n\n[CORRECTED]\nJó nap.\n\n[END]\n```")
	if r.Status != StatusOK || len(r.Errors) != 0 || r.Level != "B1" || r.Corrected != "Jó nap." {
		t.Errorf("no-error answer: %+v", r)
	}
}
