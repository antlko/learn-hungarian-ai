package srs

import (
	"testing"
	"time"
)

func TestDeriveRating(t *testing.T) {
	median := 4 * time.Second

	tests := []struct {
		name string
		out  Outcome
		want Rating
	}{
		{"wrong", Outcome{Correct: false}, Again},
		{"wrong even if fast", Outcome{Correct: false, Duration: time.Second, MedianDuration: median}, Again},
		{"correct after a hint is not retrieval", Outcome{Correct: true, UsedHint: true}, Hard},
		{"correct but only nearly", Outcome{Correct: true, PartiallyCorrect: true}, Hard},
		{"correct and fast", Outcome{Correct: true, Duration: 2 * time.Second, MedianDuration: median}, Easy},
		{"correct at typical speed", Outcome{Correct: true, Duration: 5 * time.Second, MedianDuration: median}, Good},
		{"correct but laboured", Outcome{Correct: true, Duration: 20 * time.Second, MedianDuration: median}, Hard},
		{"no latency data", Outcome{Correct: true}, Good},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeriveRating(tt.out); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOnlyRequiredFeaturesAreGraded(t *testing.T) {
	// The discipline that keeps the heatmap meaningful: grade what the item
	// tested, not everything in the sentence.
	target := []string{"Case=Ine", "Number[psed]=Plur", "Person[psor]=1"}

	got := RequiredFeatures(target, Outcome{Correct: true}, "")
	if len(got) != 3 {
		t.Fatalf("want 3 graded features, got %d", len(got))
	}
	for _, g := range got {
		if !g.Correct || g.Rating != Good {
			t.Errorf("%s: got %v/%v", g.FeatureID, g.Rating, g.Correct)
		}
	}
}

func TestAnAttributedErrorBlamesOnlyTheWrongFeature(t *testing.T) {
	// A learner who chose the right case but the wrong harmony variant must not
	// lose credit for the case. This is what makes the weakness map actionable
	// rather than merely gloomy.
	target := []string{"Case=Ine", "Number=Sing"}

	got := RequiredFeatures(target, Outcome{Correct: false}, "Case=Ine")
	if len(got) != 1 {
		t.Fatalf("want only the wrong feature graded, got %d: %+v", len(got), got)
	}
	if got[0].FeatureID != "Case=Ine" || got[0].Correct {
		t.Errorf("got %+v", got[0])
	}
}

func TestUnattributableErrorGradesEverythingRequired(t *testing.T) {
	// If the diff cannot say which feature failed, recording nothing would lose
	// the signal entirely — so all required features take the hit.
	target := []string{"Case=Ine", "Number=Sing"}
	got := RequiredFeatures(target, Outcome{Correct: false}, "")
	if len(got) != 2 {
		t.Fatalf("want both graded, got %d", len(got))
	}
	for _, g := range got {
		if g.Rating != Again {
			t.Errorf("%s rated %v", g.FeatureID, g.Rating)
		}
	}
}

func TestNamedFeatureOutsideTheTargetSetDoesNotSilentlyDropTheReview(t *testing.T) {
	target := []string{"Case=Ine"}
	got := RequiredFeatures(target, Outcome{Correct: false}, "Mood=Cnd")
	if len(got) != 1 || got[0].FeatureID != "Case=Ine" {
		t.Errorf("a mis-attributed error should still grade the required features, got %+v", got)
	}
}

func TestNoTargetFeaturesGradesNothing(t *testing.T) {
	// A recognition card tests no specific suffix.
	if got := RequiredFeatures(nil, Outcome{Correct: true}, ""); got != nil {
		t.Errorf("want no feature grades, got %+v", got)
	}
}

func TestFeatureGradesAreDeterministicallyOrdered(t *testing.T) {
	target := []string{"Person[psor]=1", "Case=Ine", "Number=Sing"}
	got := RequiredFeatures(target, Outcome{Correct: true}, "")
	for i := 1; i < len(got); i++ {
		if got[i-1].FeatureID > got[i].FeatureID {
			t.Errorf("not sorted: %v", got)
		}
	}
}

func TestRollingAccuracyTracksAChangeInPerformance(t *testing.T) {
	// Regression: an integer correct/total pair truncates when aged, so a
	// learner stuck at 1/20 could never climb back above the gate no matter how
	// many correct answers followed.
	const window = 20
	acc, total := 0.0, 0

	for range window {
		acc, total = RollingAccuracy(acc, total, window, false)
	}
	if acc > 0.05 {
		t.Fatalf("after twenty wrong answers accuracy is %.2f", acc)
	}

	for range 3 * window {
		acc, total = RollingAccuracy(acc, total, window, true)
	}
	if acc < 0.80 {
		t.Errorf("accuracy failed to recover after sustained success: %.2f", acc)
	}
	if total != 4*window {
		t.Errorf("observation count: got %d, want %d", total, 4*window)
	}
}

func TestRollingAccuracyRespondsWithinAboutOneWindow(t *testing.T) {
	// The gate has to notice a real improvement reasonably quickly, or a learner
	// who has genuinely got it stays blocked and loses trust in the system.
	const window = 20
	acc, total := 0.0, 0
	for range window {
		acc, total = RollingAccuracy(acc, total, window, false)
	}
	for range window {
		acc, total = RollingAccuracy(acc, total, window, true)
	}
	if acc < 0.60 {
		t.Errorf("after one full window of success accuracy is only %.2f", acc)
	}
}

func TestGateRefusesToActOnTooFewObservations(t *testing.T) {
	const window = 20
	acc, total := 0.0, 0
	for range 3 {
		acc, total = RollingAccuracy(acc, total, window, true)
	}
	if acc < 0.99 {
		t.Fatalf("three correct answers should read as high accuracy, got %.2f", acc)
	}
	if GateReady(total, window) {
		t.Error("three lucky answers must not open the gate")
	}
	for range window {
		acc, total = RollingAccuracy(acc, total, window, true)
	}
	if !GateReady(total, window) {
		t.Error("gate should be ready after a full window")
	}
}

func TestUpdateMedianConvergesTowardsObservations(t *testing.T) {
	if got := UpdateMedian(0, 5000); got != 5000 {
		t.Errorf("first observation should seed the median, got %d", got)
	}
	if got := UpdateMedian(5000, 0); got != 5000 {
		t.Errorf("a zero observation should be ignored, got %d", got)
	}

	m := int64(10_000)
	for range 400 {
		m = UpdateMedian(m, 2_000)
	}
	if m > 3_000 {
		t.Errorf("median failed to converge downward: %d", m)
	}
}
