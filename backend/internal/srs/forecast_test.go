package srs

import (
	"testing"
	"time"
)

func baseInput() ForecastInput {
	return ForecastInput{
		NewPerDay:        20,
		CardsPerNote:     3,
		SecondsPerReview: 8,
		DailyMinutes:     25,
		DesiredRetention: 0.9,
		Days:             90,
	}
}

func TestForecastProducesTheRequestedWindow(t *testing.T) {
	f := ProjectForecast(baseInput())
	if len(f.Days) != 90 {
		t.Fatalf("want 90 days, got %d", len(f.Days))
	}
	if f.Days[0].Date == "" {
		t.Error("days should be dated")
	}
}

func TestMoreNewCardsMeansMoreLoad(t *testing.T) {
	small := baseInput()
	small.NewPerDay = 5
	big := baseInput()
	big.NewPerDay = 50

	if ProjectForecast(big).SteadyStateReviews <= ProjectForecast(small).SteadyStateReviews {
		t.Error("raising the new-card limit should raise steady-state load")
	}
}

func TestGovernorRefusesAnUnsustainableLimitAndSuggestsOne(t *testing.T) {
	// 20 new notes a day at three cards each, against a 25-minute budget, is the
	// avalanche the brief warns about.
	v := CheckNewCardLimit(baseInput(), 20)
	if v.Allowed {
		t.Fatalf("20 new/day in 25 min should not be allowed: steady state %d min",
			v.Forecast.SteadyStateMinutes)
	}
	if v.Suggested >= 20 {
		t.Errorf("suggestion %d does not reduce the load", v.Suggested)
	}
	if v.Reason == "" {
		t.Error("a refusal must explain itself")
	}
	// The projection has to come back with the refusal — the learner sees the
	// curve, not just a "no".
	if len(v.Forecast.Days) == 0 {
		t.Error("no forecast returned with the verdict")
	}
}

func TestGovernorAllowsAModestLimit(t *testing.T) {
	in := baseInput()
	in.DailyMinutes = 90
	v := CheckNewCardLimit(in, 5)
	if !v.Allowed {
		t.Errorf("5 new/day in 90 min should fit; steady state was %d min",
			v.Forecast.SteadyStateMinutes)
	}
}

func TestSuggestedLimitActuallyFits(t *testing.T) {
	// Whatever the governor suggests must itself pass the check, or the advice
	// is worse than useless.
	for _, minutes := range []int{15, 30, 60, 120} {
		in := baseInput()
		in.DailyMinutes = minutes
		v := CheckNewCardLimit(in, 100)

		probe := in
		probe.NewPerDay = v.Suggested
		if got := ProjectForecast(probe); !got.Sustainable && v.Suggested > 0 {
			t.Errorf("budget %d min: suggested %d, which projects %d min",
				minutes, v.Suggested, got.SteadyStateMinutes)
		}
	}
}

func TestGovernorCountsCardsNotNotes(t *testing.T) {
	// "20 new a day" with five directions is a hundred cards a day. A learner
	// who was not told that will drown, so the governor must see the multiplier.
	one := baseInput()
	one.CardsPerNote = 1
	five := baseInput()
	five.CardsPerNote = 5

	if ProjectForecast(five).SteadyStateReviews <= ProjectForecast(one).SteadyStateReviews {
		t.Error("cards per note is not affecting the projection")
	}
}

func TestNoBudgetMeansNoRefusal(t *testing.T) {
	in := baseInput()
	in.DailyMinutes = 0
	if v := CheckNewCardLimit(in, 100); !v.Allowed {
		t.Error("with no stated budget the governor has nothing to enforce")
	}
}

func TestExistingBacklogIsIncluded(t *testing.T) {
	in := baseInput()
	in.NewPerDay = 0
	clean := ProjectForecast(in)

	in.ExistingDue = make([]int, 10)
	for i := range in.ExistingDue {
		in.ExistingDue[i] = 200
	}
	backlogged := ProjectForecast(in)

	if backlogged.PeakReviews <= clean.PeakReviews {
		t.Error("an existing backlog should raise the projected peak")
	}
}

func TestHigherRetentionCostsMoreReviews(t *testing.T) {
	low := baseInput()
	low.DesiredRetention = 0.80
	high := baseInput()
	high.DesiredRetention = 0.95

	if ProjectForecast(high).SteadyStateReviews < ProjectForecast(low).SteadyStateReviews {
		t.Error("higher desired retention should cost more reviews, not fewer")
	}
}

func TestProjectForecastTerminates(t *testing.T) {
	// Regression: ProjectForecast computed its suggestion by calling
	// suggestLimit, which bisected by calling ProjectForecast — unbounded mutual
	// recursion that blew the stack on the very first call.
	done := make(chan Forecast, 1)
	go func() { done <- ProjectForecast(baseInput()) }()

	select {
	case f := <-done:
		if len(f.Days) == 0 {
			t.Error("empty projection")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ProjectForecast did not terminate")
	}
}
