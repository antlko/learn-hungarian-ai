package srs

import (
	"math/rand"
	"testing"
	"time"
)

// THE property test: for any sequence of answers, live scheduling and replaying
// the log afterwards land on the identical state. This is the contract that
// makes undo-by-replay sound, and it is the test that catches an FSRS upgrade
// changing behaviour under our feet.
func TestReplayReproducesLiveStateExactly(t *testing.T) {
	rnd := rand.New(rand.NewSource(20260822))
	sched := NewScheduler(0.9)

	for trial := 0; trial < 200; trial++ {
		n := 1 + rnd.Intn(30)
		now := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC).
			Add(time.Duration(rnd.Intn(1000)) * time.Hour)

		live := NewCardState(now)
		var steps []ReplayStep
		for i := 0; i < n; i++ {
			rating := Rating(1 + rnd.Intn(4))
			next, _, err := sched.Review(live, rating, now)
			if err != nil {
				t.Fatal(err)
			}
			live = next
			steps = append(steps, ReplayStep{Rating: rating, ReviewTime: now})
			// Sometimes early, sometimes on time, sometimes late — fuzz kicks in
			// past 2.5 days, so the gaps must cross that boundary.
			gap := time.Duration(1+rnd.Intn(21*24)) * time.Hour
			now = now.Add(gap)
		}

		replayed, err := sched.ReplayCard(steps)
		if err != nil {
			t.Fatal(err)
		}
		if !replayed.Due.Equal(live.Due) {
			t.Fatalf("trial %d: due %v != %v — fuzz must be deterministic under replay",
				trial, replayed.Due, live.Due)
		}
		if replayed.Stability != live.Stability || replayed.Difficulty != live.Difficulty {
			t.Fatalf("trial %d: S/D (%v,%v) != (%v,%v)", trial,
				replayed.Stability, replayed.Difficulty, live.Stability, live.Difficulty)
		}
		if replayed.Reps != live.Reps || replayed.Lapses != live.Lapses ||
			replayed.State != live.State || replayed.ScheduledDays != live.ScheduledDays ||
			replayed.RemainingSteps != live.RemainingSteps {
			t.Fatalf("trial %d: counters diverged: %+v vs %+v", trial, replayed, live)
		}
		if !replayed.LastReview.Equal(live.LastReview) {
			t.Fatalf("trial %d: last review diverged", trial)
		}
	}
}

// The feature replay reproduces the rolling counters too — the gate reads them,
// so an undo that restored FSRS state but not the EMA would leave the two
// halves of morph_state disagreeing about the same history.
func TestReplayFeatureReproducesRollingCounters(t *testing.T) {
	rnd := rand.New(rand.NewSource(7))
	sched := NewScheduler(0.9)
	const window = 20

	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	live := FeatureState{FeatureID: "Case=Ine", State: NewCardState(now)}
	var steps []ReplayStep
	for i := 0; i < 60; i++ {
		rating := Rating(1 + rnd.Intn(4))
		correct := rating != Again
		dur := int64(500 + rnd.Intn(4000))

		next, _, err := sched.Review(live.State, rating, now)
		if err != nil {
			t.Fatal(err)
		}
		live.State = next
		live.Accuracy, live.Total = RollingAccuracy(live.Accuracy, live.Total, window, correct)
		if correct {
			live.MedianMs = UpdateMedian(live.MedianMs, dur)
		}
		steps = append(steps, ReplayStep{Rating: rating, ReviewTime: now, Correct: correct, DurationMs: dur})
		now = now.Add(time.Duration(1+rnd.Intn(72)) * time.Hour)
	}

	replayed, err := sched.ReplayFeature("Case=Ine", steps, window)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Accuracy != live.Accuracy || replayed.Total != live.Total || replayed.MedianMs != live.MedianMs {
		t.Fatalf("counters diverged: %+v vs %+v", replayed, live)
	}
	if !replayed.State.Due.Equal(live.State.Due) || replayed.State.Stability != live.State.Stability {
		t.Fatal("FSRS state diverged")
	}
}

// Undo semantics in miniature: replaying all-but-the-last step equals the state
// as it stood before the last answer.
func TestReplayAllButLastIsUndo(t *testing.T) {
	sched := NewScheduler(0.9)
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)

	state := NewCardState(now)
	var steps []ReplayStep
	var beforeLast CardState
	for i, r := range []Rating{Good, Good, Hard, Good, Easy} {
		if i == 4 {
			beforeLast = state
		}
		next, _, err := sched.Review(state, r, now)
		if err != nil {
			t.Fatal(err)
		}
		state = next
		steps = append(steps, ReplayStep{Rating: r, ReviewTime: now})
		now = now.Add(96 * time.Hour)
	}

	undone, err := sched.ReplayCard(steps[:len(steps)-1])
	if err != nil {
		t.Fatal(err)
	}
	if !undone.Due.Equal(beforeLast.Due) || undone.Stability != beforeLast.Stability ||
		undone.Reps != beforeLast.Reps || undone.RemainingSteps != beforeLast.RemainingSteps {
		t.Fatalf("undo-by-replay diverged: %+v vs %+v", undone, beforeLast)
	}
}
