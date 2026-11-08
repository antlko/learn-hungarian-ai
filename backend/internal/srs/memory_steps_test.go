package srs

import (
	"testing"
	"time"
)

// The learning ladder the review card prints must be the ladder the scheduler
// actually walks. These tests exist because the alternative — reading
// «вернётся через ~10 мин» and guessing — is the complaint that produced the
// feature, and a ladder that drifts from FSRS would be a worse version of the
// same problem.

func TestANewCardHasNoStepsUntilItIsAnswered(t *testing.T) {
	sched := NewScheduler(0.9)
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)

	m := MemoryFor(NewCardState(now), sched, now, true)
	if m.Stage != StageNew {
		t.Errorf("stage = %q, want new", m.Stage)
	}
	if m.StepsTotal != 0 || m.StepsLeft != 0 {
		t.Errorf("a card nobody has answered is on no step: got %d/%d", m.StepsLeft, m.StepsTotal)
	}
}

// The exact sequence the learner reported: answer a new word correctly and it
// comes back in ten minutes. The screen must be able to say that this is the
// LAST step, and that one more correct answer ends it.
func TestGoodTwiceWalksTheLadderAndSaysWhenItEnds(t *testing.T) {
	sched := NewScheduler(0.9)
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	total := len(sched.LearningSteps())
	if total == 0 {
		t.Skip("this build has no learning steps")
	}

	// First Good: onto the minute-scale steps.
	st, _, err := sched.Review(NewCardState(now), Good, now)
	if err != nil {
		t.Fatal(err)
	}
	m := MemoryFor(st, sched, now, true)
	if m.Stage != StageLearning {
		t.Fatalf("stage after one Good = %q, want learning", m.Stage)
	}
	if m.StepsTotal != total {
		t.Errorf("stepsTotal = %d, want %d (the scheduler's own steps)", m.StepsTotal, total)
	}
	if m.StepsLeft != st.RemainingSteps {
		t.Errorf("stepsLeft = %d but the card's RemainingSteps is %d — "+
			"the ladder must report the scheduler's position, not its own count",
			m.StepsLeft, st.RemainingSteps)
	}
	if !m.GraduatesOnGood {
		t.Error("with one step left, the next Good graduates the card and the screen must say so")
	}

	// Second Good: off the steps, onto days.
	now = now.Add(10 * time.Minute)
	st2, _, err := sched.Review(st, Good, now)
	if err != nil {
		t.Fatal(err)
	}
	m2 := MemoryFor(st2, sched, now, true)
	if m2.Stage != StageYoung && m2.Stage != StageMature {
		t.Fatalf("stage after graduating = %q, want young or mature", m2.Stage)
	}
	if m2.StepsTotal != 0 || m2.StepsLeft != 0 {
		t.Errorf("a graduated card is on no step: got %d/%d", m2.StepsLeft, m2.StepsTotal)
	}
	if m2.GraduatesOnGood {
		t.Error("a card already scheduled in days cannot graduate again")
	}
	if st2.ScheduledDays == 0 {
		t.Error("a graduated card should be scheduled in days")
	}
}

// Again on a learning card puts the whole ladder back. The screen saying
// «шаг 2 из 2» after a miss would be a lie in the learner's favour, which is
// the kind this project cares about most.
func TestAgainResetsTheLadder(t *testing.T) {
	sched := NewScheduler(0.9)
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	total := len(sched.LearningSteps())
	if total < 2 {
		t.Skip("this build has fewer than two learning steps")
	}

	st, _, err := sched.Review(NewCardState(now), Good, now)
	if err != nil {
		t.Fatal(err)
	}
	if MemoryFor(st, sched, now, true).StepsLeft != total-1 {
		t.Fatalf("setup: expected to be one step in")
	}

	now = now.Add(10 * time.Minute)
	missed, _, err := sched.Review(st, Again, now)
	if err != nil {
		t.Fatal(err)
	}
	m := MemoryFor(missed, sched, now, true)
	if m.StepsLeft != total {
		t.Errorf("after Again the ladder restarts: stepsLeft = %d, want %d", m.StepsLeft, total)
	}
	if m.GraduatesOnGood {
		t.Error("a card at the bottom of the ladder does not graduate on the next Good")
	}
}

// Free practice schedules nothing, so it may not claim a card is about to
// graduate either — ADR-019.
func TestPracticeReportsNoGraduation(t *testing.T) {
	sched := NewScheduler(0.9)
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	st, _, err := sched.Review(NewCardState(now), Good, now)
	if err != nil {
		t.Fatal(err)
	}
	m := MemoryFor(st, sched, now, false)
	if m.Preview != nil {
		t.Error("practice must carry no preview")
	}
	if m.GraduatesOnGood {
		t.Error("practice schedules nothing, so it cannot promise a graduation")
	}
	// The ladder itself is a fact about the stored card, not a schedule, so it
	// is still reported: «шаг 2 из 2» is true in practice too.
	if m.StepsTotal == 0 {
		t.Error("the card's position on the ladder is a fact and survives practice")
	}
}
