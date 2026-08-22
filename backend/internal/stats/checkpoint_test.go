package stats

import "testing"

// The diagnostic maps evidence to the three documented failure modes — and
// ONLY when the data supports them: a diagnostic that always lists all three
// is a horoscope.
func TestDiagnoseMapsTheThreeFailureModes(t *testing.T) {
	healthy := Evidence{MissedDays30: 2, AvgCoverage30: 0.95, HumanDays30: 3}
	if got := Diagnose(healthy); len(got.Findings) != 0 {
		t.Fatalf("healthy evidence produced findings: %+v", got)
	}

	broken := Evidence{MissedDays30: 15, AvgCoverage30: 0.6, HumanDays30: 0}
	got := Diagnose(broken)
	if len(got.Findings) != 3 {
		t.Fatalf("want all three failure modes, got %+v", got)
	}
	keys := map[string]bool{}
	for _, f := range got.Findings {
		keys[f.Key] = true
	}
	for _, want := range []string{
		"checkpoint.diag.missedDays",
		"checkpoint.diag.inputTooHard",
		"checkpoint.diag.noLivePractice",
	} {
		if !keys[want] {
			t.Errorf("missing %s", want)
		}
	}

	onlyGap := Evidence{MissedDays30: 12, AvgCoverage30: 0.93, HumanDays30: 2}
	got = Diagnose(onlyGap)
	if len(got.Findings) != 1 || got.Findings[0].Key != "checkpoint.diag.missedDays" {
		t.Errorf("want exactly the missed-days finding, got %+v", got)
	}
}

// The правило отката: a failed checkpoint blocks its level and above; a later
// pass lifts the block.
func TestBlockedLevels(t *testing.T) {
	if got := BlockedLevels(nil); len(got) != 0 {
		t.Fatalf("no history blocks nothing, got %v", got)
	}

	failedM3 := []Checkpoint{{Code: "m3", Status: "failed"}}
	got := BlockedLevels(failedM3)
	if !got["A2"] || !got["B1"] || !got["B2"] {
		t.Errorf("a failed m3 blocks A2 and above, got %v", got)
	}
	if got["A1"] {
		t.Error("m3 must not block A1 — the learner has to be able to repair")
	}

	// A retake that passes lifts it. History is newest-first.
	retaken := []Checkpoint{
		{Code: "m3", Status: "passed"},
		{Code: "m3", Status: "failed"},
	}
	if got := BlockedLevels(retaken); len(got) != 0 {
		t.Errorf("a later pass lifts the block, got %v", got)
	}

	// m1 gates nothing even when failed: it is a health check.
	m1fail := []Checkpoint{{Code: "m1", Status: "failed"}}
	if got := BlockedLevels(m1fail); len(got) != 0 {
		t.Errorf("m1 gates no level, got %v", got)
	}
}
