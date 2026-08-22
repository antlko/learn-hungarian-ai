package curriculum

import (
	"slices"
	"testing"
)

func ladderOf(t *testing.T) []Topic {
	t.Helper()
	d, err := Load(newTestPack(t, sample()))
	if err != nil {
		t.Fatal(err)
	}
	return d.Ladder(nil)
}

// The listening-gated topic is excluded: probing it could only produce a result
// we would have to ignore, and asking a question whose answer we discard is a
// waste of the learner's attention.
func TestLadderSkipsListeningGatedTopics(t *testing.T) {
	got := ladderOf(t)
	for _, topic := range got {
		if topic.Code == "focus-word-order" {
			t.Fatal("the listening-gated topic must not be probed")
		}
	}
	if len(got) != 6 {
		t.Errorf("ladder length = %d, want 6", len(got))
	}
}

func TestLadderRespectsTheDrillableFilter(t *testing.T) {
	d, err := Load(newTestPack(t, sample()))
	if err != nil {
		t.Fatal(err)
	}
	got := d.Ladder(func(topic Topic) bool { return topic.Code != "orthography" })
	for _, topic := range got {
		if topic.Code == "orthography" {
			t.Fatal("a topic with no generator would stall the ladder forever")
		}
	}
}

func TestNextProbeWalksInOrder(t *testing.T) {
	ladder := ladderOf(t)
	results := map[string]TopicResult{}

	st := NextProbe(ladder, results)
	if st.Done || st.Next.Code != "orthography" {
		t.Fatalf("first probe = %q done=%v", st.Next.Code, st.Done)
	}
	if st.Remaining != ProbesPerTopic {
		t.Errorf("remaining = %d, want %d", st.Remaining, ProbesPerTopic)
	}

	// A partially answered topic keeps its place and asks for the rest.
	results["orthography"] = TopicResult{Code: "orthography", Answered: 1, Correct: 1}
	st = NextProbe(ladder, results)
	if st.Next.Code != "orthography" || st.Remaining != ProbesPerTopic-1 {
		t.Errorf("partial topic: next=%q remaining=%d", st.Next.Code, st.Remaining)
	}

	results["orthography"] = TopicResult{Code: "orthography", Answered: 3, Correct: 3}
	if st = NextProbe(ladder, results); st.Next.Code != "vowel-harmony" {
		t.Errorf("after a pass, next = %q", st.Next.Code)
	}
}

// One failure can be a bad item; two in a row is a level. Continuing past that
// point is asking a beginner about the causative.
func TestLadderStopsAfterConsecutiveFailures(t *testing.T) {
	ladder := ladderOf(t)
	results := map[string]TopicResult{
		"orthography":   {Code: "orthography", Answered: 3, Correct: 3},
		"vowel-harmony": {Code: "vowel-harmony", Answered: 3, Correct: 0},
		"accusative":    {Code: "accusative", Answered: 3, Correct: 1},
	}
	if st := NextProbe(ladder, results); !st.Done {
		t.Errorf("two consecutive failures must end the test, next = %q", st.Next.Code)
	}

	// A single failure between passes does not stop it.
	results["vowel-harmony"] = TopicResult{Code: "vowel-harmony", Answered: 3, Correct: 3}
	if st := NextProbe(ladder, results); st.Done {
		t.Error("one isolated failure must not end the test")
	}
}

// The seed must never claim more than was observed. Three items is three items.
func TestSeedFromCarriesItsOwnUncertainty(t *testing.T) {
	ladder := ladderOf(t)
	results := map[string]TopicResult{
		"orthography":   {Code: "orthography", Answered: 3, Correct: 3},
		"vowel-harmony": {Code: "vowel-harmony", Answered: 3, Correct: 2},
		"accusative":    {Code: "accusative", Answered: 3, Correct: 0},
	}
	seeds := SeedFrom(ladder, results)
	byCode := map[string]Seed{}
	for _, s := range seeds {
		byCode[s.Code] = s
	}

	if s := byCode["orthography"]; s.Phase != "interleaved" || s.Observations != 3 {
		t.Errorf("passed topic seeded as %+v", s)
	}
	if s := byCode["orthography"]; s.Accuracy != 1 {
		t.Errorf("accuracy = %v, want the observed 3/3", s.Accuracy)
	}
	if s := byCode["accusative"]; s.Phase != "blocked" {
		t.Errorf("a failed topic must start in blocked practice, got %q", s.Phase)
	}
	for _, s := range seeds {
		if s.Phase == "mastered" {
			t.Errorf("%s: nobody masters a topic in three items", s.Code)
		}
		if s.Observations > ProbesPerTopic {
			t.Errorf("%s: seeded %d observations from %d probes",
				s.Code, s.Observations, ProbesPerTopic)
		}
	}
	// Topics after the stopping point are untouched — they open through the gate.
	if _, seeded := byCode["definite-conjugation"]; seeded {
		t.Error("an unprobed topic must not be seeded")
	}
}

// A learner who handled the conditional has demonstrated the accusative more
// convincingly than three accusative items would.
func TestSeedFromInfersSkippedPrerequisites(t *testing.T) {
	ladder := ladderOf(t)
	results := map[string]TopicResult{
		"orthography":   {Code: "orthography", Answered: 3, Correct: 0},
		"vowel-harmony": {Code: "vowel-harmony", Answered: 3, Correct: 3},
	}
	byCode := map[string]Seed{}
	for _, s := range SeedFrom(ladder, results) {
		byCode[s.Code] = s
	}
	if s, ok := byCode["orthography"]; !ok || s.Phase != "interleaved" {
		t.Errorf("a topic implied by a later pass must open: %+v", s)
	}
	if byCode["orthography"].Observations != 0 {
		t.Error("an inferred seed must carry zero observations — we did not measure it")
	}
}

func TestTopicResultBar(t *testing.T) {
	if (TopicResult{Answered: 3, Correct: 2}).Passed() != true {
		t.Error("2 of 3 passes")
	}
	if (TopicResult{Answered: 3, Correct: 1}).Passed() != false {
		t.Error("1 of 3 fails")
	}
	if (TopicResult{Answered: 2, Correct: 2}).Passed() != false {
		t.Error("an incomplete topic has not passed")
	}
	if !slices.Contains([]bool{true}, (TopicResult{Answered: 3}).Complete()) {
		t.Error("three answers is complete regardless of the score")
	}
}
