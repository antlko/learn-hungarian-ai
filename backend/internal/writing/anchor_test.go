package writing

import (
	"strings"
	"testing"
	"time"
)

func hl(segs []Segment, i int) []string {
	var out []string
	for _, s := range segs {
		for _, e := range s.Errors {
			if e == i {
				out = append(out, s.Text)
			}
		}
	}
	return out
}

func concat(segs []Segment) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(s.Text)
	}
	return b.String()
}

func TestAnchorExactAndNth(t *testing.T) {
	text := "A ház nagy. A ház kicsi."
	segs, ok := Anchor(text, []Error{{Quote: "ház", Nth: 2}, {Quote: "ház", Nth: 9}, {Quote: "kicsi", Nth: 1}})
	if !ok[0] || !ok[1] || !ok[2] {
		t.Fatalf("anchored = %v", ok)
	}
	if concat(segs) != text {
		t.Errorf("segments do not rebuild the text: %q", concat(segs))
	}
	// NTH 2 is the second "ház"; NTH 9 falls back to the first.
	if segs[0].Text != "A " || segs[1].Text != "ház" || len(segs[1].Errors) != 1 || segs[1].Errors[0] != 1 {
		t.Errorf("first segments = %+v", segs[:2])
	}
	if got := hl(segs, 0); len(got) != 1 || got[0] != "ház" {
		t.Errorf("error 0 = %v", got)
	}
	var second Segment
	for _, s := range segs {
		for _, e := range s.Errors {
			if e == 0 {
				second = s
			}
		}
	}
	if !strings.HasSuffix(text[:strings.Index(text, "kicsi")], second.Text+" ") {
		t.Errorf("NTH 2 picked the wrong occurrence")
	}
}

func TestAnchorMultibyte(t *testing.T) {
	text := "Őszinte örömmel mentem az egyetemre, és főztem húst."
	segs, ok := Anchor(text, []Error{{Quote: "főztem húst", Nth: 1}, {Quote: "örömmel", Nth: 1}})
	if !ok[0] || !ok[1] {
		t.Fatalf("anchored = %v", ok)
	}
	if concat(segs) != text {
		t.Errorf("rebuilt %q", concat(segs))
	}
	if strings.Join(hl(segs, 0), "") != "főztem húst" {
		t.Errorf("highlight = %v", hl(segs, 0))
	}
	for _, s := range segs {
		if !validUTF8(s.Text) {
			t.Errorf("segment cut inside a rune: %q", s.Text)
		}
	}
}

func validUTF8(s string) bool { return strings.ToValidUTF8(s, "�") == s }

func TestAnchorFoldsCaseWhitespaceQuotes(t *testing.T) {
	text := "Szeretem   a „kávét”\n és a Teát."
	segs, ok := Anchor(text, []Error{{Quote: `"kávét"`, Nth: 1}, {Quote: "szeretem a", Nth: 1}, {Quote: "TEÁT", Nth: 1}})
	if !ok[0] || !ok[1] || !ok[2] {
		t.Fatalf("anchored = %v", ok)
	}
	if got := strings.Join(hl(segs, 0), ""); got != "„kávét”" {
		t.Errorf("quotes: %q", got)
	}
	if got := strings.Join(hl(segs, 1), ""); got != "Szeretem   a" {
		t.Errorf("whitespace: %q", got)
	}
	if got := strings.Join(hl(segs, 2), ""); got != "Teát" {
		t.Errorf("case: %q", got)
	}
}

func TestAnchorDiacriticFallbackOnlyWhenUnique(t *testing.T) {
	segs, ok := Anchor("Ez a kenyer jo.", []Error{{Quote: "kenyér", Nth: 1}, {Quote: "jó", Nth: 1}})
	if !ok[0] || !ok[1] || strings.Join(hl(segs, 0), "") != "kenyer" {
		t.Fatalf("unique diacritic match failed: %v %+v", ok, segs)
	}
	// "ma" would match both "ma" and "má" folded: ambiguous, so not anchored.
	_, ok = Anchor("ma má", []Error{{Quote: "mà", Nth: 1}})
	if ok[0] {
		t.Error("ambiguous diacritic match anchored")
	}
}

func TestAnchorNotFoundAndEmpty(t *testing.T) {
	segs, ok := Anchor("Jó nap.", []Error{{Quote: "zzz", Nth: 1}, {Quote: "  ", Nth: 1}})
	if ok[0] || ok[1] || len(segs) != 1 || len(segs[0].Errors) != 0 || segs[0].Text != "Jó nap." {
		t.Errorf("segs=%+v ok=%v", segs, ok)
	}
	if segs, _ := Anchor("", []Error{{Quote: "a"}}); segs != nil {
		t.Errorf("empty text gave %v", segs)
	}
}

func TestAnchorOverlaps(t *testing.T) {
	segs, ok := Anchor("Nem tudom a szót.", []Error{{Quote: "tudom a", Nth: 1}, {Quote: "a szót", Nth: 1}})
	if !ok[0] || !ok[1] {
		t.Fatal("not anchored")
	}
	if concat(segs) != "Nem tudom a szót." {
		t.Errorf("rebuilt %q", concat(segs))
	}
	var both string
	for _, s := range segs {
		if len(s.Errors) == 2 {
			both = s.Text
		}
	}
	if both != "a" {
		t.Errorf("overlap segment = %q, want a", both)
	}
}

func TestAnchorNormalisesToNFC(t *testing.T) {
	decomposed := "Ez a kávé." // á, é as combining marks
	segs, ok := Anchor(decomposed, []Error{{Quote: "kávé", Nth: 1}})
	if !ok[0] || strings.Join(hl(segs, 0), "") != "kávé" {
		t.Errorf("segs=%+v ok=%v", segs, ok)
	}
}

// Hostile input stays cheap: 60 errors with long near-miss quotes against a
// full-size text must not cost seconds (the check is rebuilt on read).
func TestAnchorIsBoundedOnAdversarialInput(t *testing.T) {
	text := strings.Repeat("a", 6000)
	var errs []Error
	for i := 0; i < 60; i++ {
		q := strings.Repeat("a", 1999) + "b"
		if i%2 == 1 {
			q = strings.Repeat("a", 119) + "b" // within the folded-stage bound
		}
		errs = append(errs, Error{N: i + 1, Quote: q, Nth: 1})
	}
	start := time.Now()
	_, anchored := Anchor(text, errs)
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("anchoring took %v", d)
	}
	for i, ok := range anchored {
		if ok {
			t.Errorf("error %d anchored a quote that is not in the text", i)
		}
	}
}
