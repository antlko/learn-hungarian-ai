package morph

import (
	"strings"
	"testing"
)

// The segmenter's contract: it may only claim boundaries the analysis
// justifies, and everything it cannot justify comes back as one honest
// unlabelled ending. The cases below include the ones where refusing to
// segment IS the correct answer.

func segmentOf(t *testing.T, a *Analyzer, form string) []Segment {
	t.Helper()
	an, err := a.Best(form)
	if err != nil {
		t.Fatalf("Best(%q): %v", form, err)
	}
	return a.SegmentForm(form, an)
}

func joinSegs(segs []Segment) string {
	parts := make([]string, 0, len(segs))
	for _, s := range segs {
		parts = append(parts, s.Form)
	}
	return strings.Join(parts, "+")
}

func TestSegmentSimpleCase(t *testing.T) {
	p := newTestPack(t)
	a := NewAnalyzer(p)

	segs := segmentOf(t, a, "házban")
	if joinSegs(segs) != "ház+ban" {
		t.Fatalf("házban = %s, want ház+ban", joinSegs(segs))
	}
	if segs[0].Kind != SegStem {
		t.Errorf("first segment kind = %s, want stem", segs[0].Kind)
	}
	if segs[1].Kind != SegSuffix || segs[1].FeatureID != "Case=Ine" {
		t.Errorf("suffix segment = %+v, want Case=Ine", segs[1])
	}
	// The rule comes from REVERSE lookup: the surface `ban` is the back variant.
	if segs[1].AllomorphRule != "back" {
		t.Errorf("allomorph rule = %q, want back", segs[1].AllomorphRule)
	}
}

// The whole reason the segmenter exists: ház+a+i+m+ban, four boundaries, all
// verifiable from the analysis.
func TestSegmentStackedPossessive(t *testing.T) {
	p := newTestPack(t)
	a := NewAnalyzer(p)

	segs := segmentOf(t, a, "házaimban")
	if got := joinSegs(segs); got != "ház+a+i+m+ban" {
		t.Fatalf("házaimban = %s, want ház+a+i+m+ban", got)
	}
	kinds := []SegmentKind{SegStem, SegLinking, SegSuffix, SegSuffix, SegSuffix}
	feats := []string{"", "", "Number[psed]=Plur", "Person[psor]=1", "Case=Ine"}
	for i, s := range segs {
		if s.Kind != kinds[i] {
			t.Errorf("segment %d kind = %s, want %s", i, s.Kind, kinds[i])
		}
		if s.FeatureID != feats[i] {
			t.Errorf("segment %d feature = %q, want %q", i, s.FeatureID, feats[i])
		}
	}
}

func TestSegmentLinkingVowel(t *testing.T) {
	p := newTestPack(t)
	a := NewAnalyzer(p)

	segs := segmentOf(t, a, "házat")
	if got := joinSegs(segs); got != "ház+a+t" {
		t.Fatalf("házat = %s, want ház+a+t — the linking vowel is neither stem nor suffix", got)
	}
	if segs[1].Kind != SegLinking || segs[1].FeatureID != "" {
		t.Errorf("linking segment must carry no feature: %+v", segs[1])
	}
}

// A fused verb ending must stay fused. `látta` is lát + [Past·3sg·Def] and any
// split of `ta` into pieces would be an invented boundary.
func TestSegmentFusedEndingStaysFused(t *testing.T) {
	p := newTestPack(t)
	a := NewAnalyzer(p)

	segs := segmentOf(t, a, "látta")
	if len(segs) != 2 {
		t.Fatalf("látta = %s (%d segments), want stem + one fused ending", joinSegs(segs), len(segs))
	}
	if segs[0].Form != "lát" || segs[0].Kind != SegStem {
		t.Errorf("stem = %+v", segs[0])
	}
	if segs[1].Kind != SegEnding {
		t.Fatalf("tail kind = %s, want ending — no invented boundaries", segs[1].Kind)
	}
	// The ending lists what it carries — the marked features only. Definiteness
	// has no exponent in the table, so the honest list here is Tense=Past.
	found := false
	for _, id := range segs[1].FeatureIDs {
		if id == "Tense=Past" {
			found = true
		}
	}
	if !found {
		t.Errorf("ending features = %v, want Tense=Past listed", segs[1].FeatureIDs)
	}
}

// An unknown form gets one stem segment and no claims at all.
func TestSegmentUnknownFormClaimsNothing(t *testing.T) {
	p := newTestPack(t)
	a := NewAnalyzer(p)

	an, _ := a.Best("xylophonzzz")
	segs := a.SegmentForm("xylophonzzz", an)
	if len(segs) != 1 || segs[0].Kind != SegStem {
		t.Errorf("unknown form: %v — must be a single unlabelled piece", segs)
	}
}

// The guard that keeps a peel from eating the lemma: `csónak` (boat) ends in
// the dative's `nak` but IS the nominative; peeling would leave `csó`.
func TestSegmentDoesNotEatTheLemma(t *testing.T) {
	p := newTestPack(t)
	a := NewAnalyzer(p)

	segs := segmentOf(t, a, "csónak")
	if got := joinSegs(segs); got != "csónak" {
		t.Fatalf("csónak = %s — the nominative must stay whole", got)
	}
}

func TestExplainLines(t *testing.T) {
	p := newTestPack(t)
	a := NewAnalyzer(p)

	lines := a.Explain(segmentOf(t, a, "házban"))
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d", len(lines))
	}
	suffix := lines[1]
	if suffix.LabelRU == "" || suffix.LabelEN == "" {
		t.Errorf("both locales, always: %+v", suffix)
	}
	if !strings.Contains(suffix.RuleRU, "ban") || !strings.Contains(suffix.RuleEN, "back") {
		t.Errorf("the rule must name the harmony class and the variant: %+v", suffix)
	}
}

// An invariant suffix gets no rule line: "-t is -t" teaches nothing.
func TestExplainSkipsInvariantRule(t *testing.T) {
	p := newTestPack(t)
	a := NewAnalyzer(p)

	lines := a.Explain(segmentOf(t, a, "házat"))
	last := lines[len(lines)-1]
	if last.FeatureID != "Case=Acc" {
		t.Fatalf("last line = %+v", last)
	}
	if last.RuleRU != "" || last.RuleEN != "" {
		t.Errorf("invariant -t must carry no allomorph rule, got %+v", last)
	}
}
