package morph

import (
	"reflect"
	"testing"
)

func hintFor(text, word, lemma, upos string) ContextHint {
	i := indexByte(text, word)
	return ContextHint{Start: i, End: i + len(word), Lemma: lemma, UPOS: upos}
}

func varIdx(toks []AnalyzedToken) int {
	for i, t := range toks {
		if t.Text == "vár" {
			return i
		}
	}
	return -1
}

func indexByte(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestContextHintsChooseOnlyAmongAttestedCandidates(t *testing.T) {
	a := NewAnalyzer(newTestPack(t))
	text := "A vár szép. Őt vár."
	toks, _, err := a.AnalyzeText(text)
	if err != nil {
		t.Fatal(err)
	}
	// Default: the frequency prior picks the VERB.
	if got := toks[varIdx(toks)].Analysis; got.UPOS != "VERB" || got.Via != "" {
		t.Fatalf("baseline: %+v", got)
	}
	// The sidecar says the first `vár` is the castle (NOUN).
	hints := []ContextHint{hintFor(text, "vár", "vár", "NOUN")}
	n, err := a.ApplyContextHints(toks, hints)
	if err != nil || n != 1 {
		t.Fatalf("changed=%d err=%v", n, err)
	}
	got := toks[varIdx(toks)].Analysis
	if got.UPOS != "NOUN" || got.Via != ViaContext || got.Certainty != UncertainAnalysis {
		t.Fatalf("want context-chosen NOUN, still uncertain: %+v", got)
	}
	if got.LemmaID != 3 {
		t.Fatalf("must be the pack's own candidate (lemma 3), got %d", got.LemmaID)
	}
}

func TestContextHintNeverIntroducesAnAnalysis(t *testing.T) {
	a := NewAnalyzer(newTestPack(t))
	text := "A vár szép."
	toks, _, _ := a.AnalyzeText(text)
	before := append([]AnalyzedToken(nil), toks...)
	// ADJ / a lemma the pack does not have for this form.
	for _, h := range []ContextHint{
		hintFor(text, "vár", "vár", "ADJ"),
		hintFor(text, "vár", "várni", "VERB"),
	} {
		n, _ := a.ApplyContextHints(toks, []ContextHint{h})
		if n != 0 || !reflect.DeepEqual(toks, before) {
			t.Fatalf("a hint with no matching candidate must change nothing: %+v", h)
		}
	}
}

func TestContextHintsLeaveCertainWordsAlone(t *testing.T) {
	a := NewAnalyzer(newTestPack(t))
	text := "házban"
	toks, _, _ := a.AnalyzeText(text)
	before := append([]AnalyzedToken(nil), toks...)
	n, _ := a.ApplyContextHints(toks, []ContextHint{{Start: 0, End: len(text), Lemma: "kert", UPOS: "NOUN"}})
	if n != 0 || !reflect.DeepEqual(toks, before) {
		t.Fatal("certain readings must never be overridden")
	}
}

func TestNoHintsIsIdentity(t *testing.T) {
	a := NewAnalyzer(newTestPack(t))
	toks, _, _ := a.AnalyzeText("A vár szép.")
	before := append([]AnalyzedToken(nil), toks...)
	if n, err := a.ApplyContextHints(toks, nil); n != 0 || err != nil || !reflect.DeepEqual(toks, before) {
		t.Fatal("no hints must leave tokens byte-for-byte unchanged")
	}
}

func TestContextHintMatchesMultibyteSpans(t *testing.T) {
	a := NewAnalyzer(newTestPack(t))
	text := "Őt várő űr vár."
	toks, _, _ := a.AnalyzeText(text)
	h := hintFor(text, "vár.", "vár", "NOUN")
	h.End-- // the word without its full stop
	if n, _ := a.ApplyContextHints(toks, []ContextHint{h}); n != 1 {
		t.Fatalf("byte offsets after ő/ű must line up, changed=%d", n)
	}
}
