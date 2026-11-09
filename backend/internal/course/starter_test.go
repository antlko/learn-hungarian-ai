package course

import (
	"testing"

	_ "modernc.org/sqlite"

	"tanulom/internal/lang"
)

// The syllabus the binary actually ships: load it out of the embedded starter
// and check the shape ADR-050 promises. This is the test that fails when the
// course source changed and `make starter` was never run.
func TestEmbeddedStarterCarriesTheTracksAndTiers(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := lang.EnsurePack(dir); err != nil {
		t.Fatalf("install starter: %v", err)
	}
	pack, err := lang.Open(lang.Path(dir, lang.StarterVersion))
	if err != nil {
		t.Fatalf("open starter: %v", err)
	}
	defer pack.Close()

	s, err := Load(pack)
	if err != nil {
		t.Fatal(err)
	}
	var core, theme, coreWords, themeWords int
	tierUnits := map[string]int{}
	tierWords := map[string]int{}
	for _, u := range s.Units {
		switch u.Track {
		case TrackCore:
			core++
			coreWords += len(u.Words)
			tierUnits[u.Tier]++
			tierWords[u.Tier] += len(u.Words)
			if u.Tier == TierAdvanced && u.Month < 13 || u.Tier == TierIntermediate && (u.Month < 7 || u.Month > 12) ||
				u.Tier == TierBasic && u.Month > 6 {
				t.Errorf("unit %s: tier %s but month %d", u.Code, u.Tier, u.Month)
			}
		case TrackTheme:
			theme++
			themeWords += len(u.Words)
		default:
			t.Errorf("unit %s has track %q", u.Code, u.Track)
		}
		if n := len(u.Words); n < 15 || n > 60 {
			t.Errorf("unit %s has %d words", u.Code, n)
		}
	}
	if core != 64 || theme != 12 {
		t.Fatalf("core=%d theme=%d, want 64/12", core, theme)
	}
	if tierUnits[TierBasic] != 24 || tierUnits[TierIntermediate] != 20 || tierUnits[TierAdvanced] != 20 {
		t.Errorf("units per tier = %v, want 24/20/20", tierUnits)
	}
	if tierWords[TierBasic] < 1300 || tierWords[TierIntermediate] < 600 || tierWords[TierAdvanced] < 600 {
		t.Errorf("words per tier = %v", tierWords)
	}
	if themeWords < 12*15 {
		t.Errorf("theme words = %d", themeWords)
	}

	// A fresh learner: next is the first core unit, the course is 64 long, and
	// no theme unit is next or counts.
	plan := s.Resolve(nil, func(string) bool { return false }, nil)
	if plan.Total != 64 || plan.Completed != 0 || plan.HandedOver {
		t.Errorf("fresh plan = total %d completed %d handedOver %v",
			plan.Total, plan.Completed, plan.HandedOver)
	}
	if first, _ := s.Unit(plan.Current); first.IsTheme() || first.Month != 1 {
		t.Errorf("next = %q (%s), want a month-1 core unit", plan.Current, first.Track)
	}

	// Everything in the deck: all 64 core units done, hand over, and the themes
	// are done too but still not counted.
	all := s.Resolve(nil, func(string) bool { return true }, nil)
	if !all.HandedOver || all.Completed != 64 || all.Current != "" {
		t.Errorf("all-in-play plan = %+v", all)
	}
	// Only the core units in the deck: handed over regardless of the themes.
	coreOnly := map[string]bool{}
	for _, u := range s.Units {
		if !u.IsTheme() {
			for _, w := range u.Words {
				coreOnly[w.Lemma] = true
			}
		}
	}
	p := s.Resolve(nil, func(l string) bool { return coreOnly[l] }, nil)
	if !p.HandedOver || p.Current != "" {
		t.Errorf("core-only plan: handedOver=%v current=%q", p.HandedOver, p.Current)
	}
}
