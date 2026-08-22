package curriculum

import (
	"slices"
	"testing"
)

func TestLoadReadsTeachOrder(t *testing.T) {
	d, err := Load(newTestPack(t, sample()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := make([]string, 0, len(d.Topics()))
	for _, topic := range d.Topics() {
		got = append(got, topic.Code)
	}
	want := []string{"orthography", "vowel-harmony", "accusative", "locative-grid",
		"past-tense", "definite-conjugation", "focus-word-order"}
	if !slices.Equal(got, want) {
		t.Errorf("teach order = %v, want %v", got, want)
	}
}

func TestSpecialTopicFlags(t *testing.T) {
	d, err := Load(newTestPack(t, sample()))
	if err != nil {
		t.Fatal(err)
	}
	harmony, _ := d.Topic("vowel-harmony")
	if !harmony.MotorSkill() {
		t.Error("vowel harmony is a motor skill — mastery needs latency [R15]")
	}
	if harmony.NeedsListeningVolume() {
		t.Error("harmony is not listening-gated")
	}
	focus, _ := d.Topic("focus-word-order")
	if !focus.NeedsListeningVolume() {
		t.Error("word order is gated behind listening volume [R15]")
	}
	acc, _ := d.Topic("accusative")
	if acc.MotorSkill() || acc.NeedsListeningVolume() {
		t.Error("an ordinary topic carries neither flag")
	}
}

// A broken DAG is a build error we would rather catch at load than discover as a
// learner stuck behind a cycle.
func TestLoadRejectsBrokenGraphs(t *testing.T) {
	cases := map[string][][]any{
		"unknown prerequisite": {{"a", "nonexistent", "", 10}},
		"self reference":       {{"a", "a", "", 10}},
		"cycle": {
			{"a", "b", "", 10},
			{"b", "a", "", 20},
		},
	}
	for name, topics := range cases {
		if _, err := Load(newTestPack(t, topics)); err == nil {
			t.Errorf("%s: Load succeeded, want an error", name)
		}
	}
}

func TestUnlocksAndAncestors(t *testing.T) {
	d, err := Load(newTestPack(t, sample()))
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Unlocks("accusative"); !slices.Equal(got, []string{"locative-grid", "past-tense"}) {
		t.Errorf("Unlocks(accusative) = %v", got)
	}
	got := d.AncestorsOf("definite-conjugation")
	for _, want := range []string{"past-tense", "accusative", "vowel-harmony", "orthography"} {
		if !slices.Contains(got, want) {
			t.Errorf("AncestorsOf(definite-conjugation) missing %q, got %v", want, got)
		}
	}
}

// The feature -> topic join is what makes error-driven follow-up possible: a
// wrong inessive in a reading review has to route to the topic that taught it.
func TestFeatureTopicPrefersTheIntroducingTopic(t *testing.T) {
	topics := append(sample(), []any{"revision", "locative-grid", "Case=Ine", 90})
	d, err := Load(newTestPack(t, topics))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := d.FeatureTopic("Case=Ine")
	if !ok {
		t.Fatal("Case=Ine must map to a topic")
	}
	if got.Code != "locative-grid" {
		t.Errorf("FeatureTopic(Case=Ine) = %q, want the topic that INTRODUCES it", got.Code)
	}
	if _, ok := d.FeatureTopic("Case=Nonexistent"); ok {
		t.Error("an unknown feature must not resolve to a topic")
	}
	if code := d.FeatureTopics()["Case=Ine"]; code != "locative-grid" {
		t.Errorf("FeatureTopics index disagrees with FeatureTopic: %q", code)
	}
}

func TestLoadWithoutAPack(t *testing.T) {
	if _, err := Load(nil); err == nil {
		t.Error("Load(nil) must fail rather than return an empty DAG")
	}
	if _, err := Load(newTestPack(t, nil)); err == nil {
		t.Error("a pack with no topics is not a usable curriculum")
	}
}
