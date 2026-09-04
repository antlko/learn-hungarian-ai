package srs

import "testing"

// The bug this pins: target_start/target_end are BYTE offsets into UTF-8, and
// the client used to slice `hu` with them as if they were JavaScript string
// indices. One accented letter before the target and the blank landed inside
// the following word — `Jó éjszakát, Timmy!` rendered as `Jó é_____immy!`.
func TestSplitTargetUsesByteOffsets(t *testing.T) {
	const hu = "Jó éjszakát, Timmy!"
	// "éjszakát" occupies bytes [4,14): é and á are two bytes each.
	got := splitTarget(hu, 4, 14)
	if got == nil {
		t.Fatal("expected a split")
	}
	if got.Before != "Jó " || got.After != ", Timmy!" {
		t.Fatalf("split = %q + ___ + %q, want %q + ___ + %q",
			got.Before, got.After, "Jó ", ", Timmy!")
	}
}

func TestSplitTargetRefusesUnusableSpans(t *testing.T) {
	const hu = "Jó éjszakát, Timmy!"
	for _, tc := range []struct {
		name       string
		start, end int
	}{
		{"unlocated target", -1, -1},
		{"empty span", 4, 4},
		{"inverted span", 14, 4},
		{"past the end", 4, len(hu) + 1},
	} {
		if got := splitTarget(hu, tc.start, tc.end); got != nil {
			t.Errorf("%s: got %+v, want nil", tc.name, got)
		}
	}
}
