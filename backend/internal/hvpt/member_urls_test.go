package hvpt

import "testing"

// The after-answer replay has to hand back BOTH members from the SAME voice at
// the SAME rate: tör from anna against tőr from imre is a talker comparison,
// not a phoneme comparison, and the whole point of the pair is the phoneme.
func TestMemberURLsPrefersTheTrialsTalkerAndRate(t *testing.T) {
	inv := &Inventory{Clips: map[int64]map[string][]Clip{
		7: {
			"a": {
				{PairID: 7, Member: "a", TalkerID: "anna", Rate: 1.0, Path: "hvpt/7_a_anna_100.ogg"},
				{PairID: 7, Member: "a", TalkerID: "imre", Rate: 1.0, Path: "hvpt/7_a_imre_100.ogg"},
				{PairID: 7, Member: "a", TalkerID: "imre", Rate: 0.85, Path: "hvpt/7_a_imre_85.ogg"},
			},
			"b": {
				{PairID: 7, Member: "b", TalkerID: "anna", Rate: 1.0, Path: "hvpt/7_b_anna_100.ogg"},
				{PairID: 7, Member: "b", TalkerID: "imre", Rate: 0.85, Path: "hvpt/7_b_imre_85.ogg"},
			},
		},
	}}

	urls := inv.MemberURLs(7, "imre", 0.85)
	if urls["a"] != "/media/hvpt/7_a_imre_85.ogg" {
		t.Errorf("a = %q, want imre at 0.85", urls["a"])
	}
	if urls["b"] != "/media/hvpt/7_b_imre_85.ogg" {
		t.Errorf("b = %q, want imre at 0.85", urls["b"])
	}

	// A talker with no clip for one member falls back to whoever has it,
	// because a missing comparison is worse than a cross-voice one.
	urls = inv.MemberURLs(7, "berta", 1.0)
	if urls["a"] == "" || urls["b"] == "" {
		t.Errorf("fallback must still return both members, got %+v", urls)
	}

	// An unknown pair returns nothing rather than inventing paths.
	if got := inv.MemberURLs(99, "anna", 1.0); len(got) != 0 {
		t.Errorf("unknown pair: got %+v", got)
	}
}
