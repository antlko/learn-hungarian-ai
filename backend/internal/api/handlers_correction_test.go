package api

import (
	"net/http"
	"testing"

	"tanulom/internal/morph"
)

// The correction loop end to end: report → overlay → visible as an override →
// retractable. The pack itself must never change (it is read-only by contract).
func TestCorrectionOverlaysTheBreakdown(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)

	// Before: the pack's own reading.
	var before BreakdownResponse
	if resp := c.json(http.MethodGet, "/api/breakdown?form=h%C3%A1zban", nil, &before); resp.StatusCode != 200 {
		t.Fatalf("breakdown: %d", resp.StatusCode)
	}
	if before.Analysis.Lemma != "ház" || before.Analysis.Certainty == morph.CorrectedAnalysis {
		t.Fatalf("unexpected baseline analysis: %+v", before.Analysis)
	}
	if len(before.Segments) < 2 {
		t.Fatalf("házban should segment, got %v", before.Segments)
	}

	// Report a correction.
	resp, _ := c.do(http.MethodPost, "/api/morph/correction", CorrectionRequest{
		Form: "házban", Lemma: "házbani", Feats: "Case=Ade", Note: "testing",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create correction: %d", resp.StatusCode)
	}

	// After: the override is applied AND visibly marked as an override.
	var after BreakdownResponse
	c.json(http.MethodGet, "/api/breakdown?form=h%C3%A1zban", nil, &after)
	if after.Analysis.Lemma != "házbani" {
		t.Errorf("lemma = %q, want the corrected one", after.Analysis.Lemma)
	}
	if after.Analysis.Certainty != morph.CorrectedAnalysis {
		t.Error("an override must be marked as one, not presented as the dictionary")
	}
	if after.Analysis.Feats["Case"] != "Ade" {
		t.Errorf("feats = %v", after.Analysis.Feats)
	}

	// Re-reporting replaces rather than erroring: the latest opinion wins.
	c.do(http.MethodPost, "/api/morph/correction", CorrectionRequest{Form: "házban", Note: "just wrong"})
	var list struct {
		Corrections []CorrectionDTO `json:"corrections"`
	}
	c.json(http.MethodGet, "/api/morph/corrections", nil, &list)
	if len(list.Corrections) != 1 {
		t.Fatalf("want 1 correction after upsert, got %d", len(list.Corrections))
	}
	if list.Corrections[0].Note != "just wrong" {
		t.Errorf("note = %q", list.Corrections[0].Note)
	}

	// A pure report (no replacement) must be accepted — "this is wrong, I don't
	// know what's right" is signal for the next pack build.
	if list.Corrections[0].Lemma != "" {
		t.Errorf("an empty correction must store empty, got %q", list.Corrections[0].Lemma)
	}

	// Retract.
	c.do(http.MethodDelete, "/api/morph/correction?form=h%C3%A1zban", nil)
	var restored BreakdownResponse
	c.json(http.MethodGet, "/api/breakdown?form=h%C3%A1zban", nil, &restored)
	if restored.Analysis.Certainty == morph.CorrectedAnalysis {
		t.Error("deleting the correction must restore the pack's reading")
	}
}

func TestBreakdownExplainsInBothLocales(t *testing.T) {
	t.Parallel()
	_, c := newPackedServer(t)
	var got BreakdownResponse
	c.json(http.MethodGet, "/api/breakdown?form=k%C3%B6nyvben", nil, &got)
	for _, line := range got.Explain {
		if line.Kind != "suffix" {
			continue
		}
		if line.LabelRU == "" || line.LabelEN == "" {
			t.Errorf("both locales, always: %+v", line)
		}
	}
}
