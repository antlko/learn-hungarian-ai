package morph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tanulom/internal/lang"
)

// The segmenter against the real pack: reassembly and no-invention checks over
// thousands of attested forms. Skipped when the pack is absent (CI).
func realPackAnalyzer(t *testing.T) *Analyzer {
	t.Helper()
	path := os.Getenv("TANULOM_TEST_PACK")
	if path == "" {
		path = filepath.Join("..", "..", "..", "data", "langpack-hu-v1.sqlite")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no language pack at %s", path)
	}
	p, err := lang.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	return NewAnalyzer(p)
}

// Two invariants over a broad sample:
//  1. REASSEMBLY — concatenating the segments must reproduce the form exactly.
//  2. NO INVENTION — a suffix segment's feature must be one the analysis carries.
func TestRealPackSegmentationInvariants(t *testing.T) {
	a := realPackAnalyzer(t)

	rows, err := a.pack.DB().Query(`
		SELECT DISTINCT w.form FROM wordform w
		 WHERE w.source = 'wiktionary' AND length(w.form) >= 3
		 ORDER BY w.rowid LIMIT 4000`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	checked, segmented := 0, 0
	for rows.Next() {
		var form string
		if err := rows.Scan(&form); err != nil {
			t.Fatal(err)
		}
		an, err := a.Best(form)
		if err != nil {
			t.Fatal(err)
		}
		segs := a.SegmentForm(form, an)
		checked++

		var b strings.Builder
		for _, s := range segs {
			b.WriteString(s.Form)
		}
		if !strings.EqualFold(b.String(), form) {
			t.Fatalf("%q reassembles to %q — segments must cover the form exactly", form, b.String())
		}
		for _, s := range segs {
			if s.Kind == SegSuffix {
				if got, want := an.Feats[keyOf(s.FeatureID)], valueOf(s.FeatureID); got != want {
					t.Fatalf("%q: segment claims %s which the analysis does not carry (%v)",
						form, s.FeatureID, an.Feats)
				}
			}
		}
		if len(segs) > 1 {
			segmented++
		}
	}
	if checked < 1000 {
		t.Fatalf("only %d forms checked", checked)
	}
	// Not a quality gate, a sanity floor: if almost nothing segments, the
	// feature-guided peel is broken even though reassembly passes.
	if segmented*10 < checked {
		t.Fatalf("only %d/%d forms produced any split — the peel is not working", segmented, checked)
	}
	t.Logf("segmented %d/%d attested forms with verified boundaries", segmented, checked)
}

func keyOf(featureID string) string   { k, _, _ := strings.Cut(featureID, "="); return k }
func valueOf(featureID string) string { _, v, _ := strings.Cut(featureID, "="); return v }

// Spot checks with known-correct segmentations, including the flagship.
func TestRealPackSegmentSpots(t *testing.T) {
	a := realPackAnalyzer(t)
	cases := map[string]string{
		"házban":   "ház+ban",
		"házon":    "ház+on", // the attested superessive, not the generated házan
		"házakban": "ház+a+k+ban",
		"könyvhöz": "könyv+höz",
		"kertben":  "kert+ben",
	}
	for form, want := range cases {
		an, err := a.Best(form)
		if err != nil {
			t.Fatal(err)
		}
		segs := a.SegmentForm(form, an)
		parts := make([]string, 0, len(segs))
		for _, s := range segs {
			parts = append(parts, s.Form)
		}
		if got := strings.Join(parts, "+"); got != want {
			t.Errorf("%s = %s, want %s", form, got, want)
		}
	}
}
