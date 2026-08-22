package srs

import "testing"

func defaults() map[Direction]float64 {
	return map[Direction]float64{
		Recognize: 1.0, Produce: 1.4, Listen: 1.0, Cloze: 1.6, Spell: 1.2,
	}
}

func TestOnlyClozeGradesFeatures(t *testing.T) {
	// Attribution is the whole point: blanking exactly one span makes it sound
	// to say WHICH suffix was wrong. Grading features from a free production
	// task would credit or blame suffixes the learner never had to choose.
	for _, d := range AllDirections {
		want := d == Cloze
		if got := d.GradesFeatures(); got != want {
			t.Errorf("%s.GradesFeatures() = %v, want %v", d, got, want)
		}
	}
}

func TestProductiveDirections(t *testing.T) {
	productive := map[Direction]bool{Produce: true, Cloze: true, Spell: true}
	for _, d := range AllDirections {
		if got := d.Productive(); got != productive[d] {
			t.Errorf("%s.Productive() = %v", d, got)
		}
	}
}

func TestPlanDropsAudioDirectionsWhenNoTTS(t *testing.T) {
	// A direction that cannot be presented must be absent, not shown broken.
	plan := PlanFor(GoalConversation, defaults(), false)
	for _, d := range plan.Directions {
		if d.NeedsAudio() {
			t.Errorf("%s planned without audio available", d)
		}
	}
	if len(plan.Directions) != 3 {
		t.Errorf("want recognize/produce/cloze, got %v", plan.Directions)
	}
}

func TestGoalReweightsDirections(t *testing.T) {
	reading := PlanFor(GoalReading, defaults(), true)
	for _, d := range reading.Directions {
		if d == Spell {
			t.Error("a reading-focused learner should not get dictation spelling")
		}
	}

	exam := PlanFor(GoalExam, defaults(), true)
	if len(exam.Directions) != 5 {
		t.Errorf("exam prep should use every direction, got %v", exam.Directions)
	}

	// Conversation should weight production above bare recognition.
	conv := PlanFor(GoalConversation, defaults(), true)
	if conv.Weights[Produce] <= conv.Weights[Recognize] {
		t.Errorf("produce (%v) should outweigh recognize (%v) for conversation",
			conv.Weights[Produce], conv.Weights[Recognize])
	}
}

func TestPlanIsOrderedByWeight(t *testing.T) {
	plan := PlanFor(GoalConversation, defaults(), true)
	for i := 1; i < len(plan.Directions); i++ {
		prev, cur := plan.Directions[i-1], plan.Directions[i]
		if plan.Weights[prev] < plan.Weights[cur] {
			t.Errorf("plan not ordered by weight: %s(%v) before %s(%v)",
				prev, plan.Weights[prev], cur, plan.Weights[cur])
		}
	}
}

func TestDisabledDirectionIsNotPlanned(t *testing.T) {
	cfg := defaults()
	cfg[Cloze] = 0
	plan := PlanFor(GoalExam, cfg, true)
	for _, d := range plan.Directions {
		if d == Cloze {
			t.Error("a direction disabled in config was still planned")
		}
	}
}

func TestUnknownGoalFallsBackRatherThanEmptying(t *testing.T) {
	plan := PlanFor("fluency-in-3-months", defaults(), true)
	if len(plan.Directions) == 0 {
		t.Fatal("an unknown goal produced no directions at all")
	}
}
