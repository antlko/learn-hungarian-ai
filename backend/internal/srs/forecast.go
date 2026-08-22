package srs

import (
	"fmt"
	"math"
	"time"
)

// Workload forecasting and the new-card governor.
//
// The source brief names the review avalanche as the single most common reason
// people abandon spaced repetition: set the new-card limit too high, and three
// weeks later the daily queue is unsurvivable and the learner quits. The usual
// mitigation is a warning in the documentation, which nobody reads.
//
// Instead, the app simulates the consequence and refuses to apply a limit it can
// show will not fit — while always rendering the curve first, so the decision is
// argued rather than merely enforced. The learner can override; they cannot
// override without seeing it.

// ForecastDay is one day of projected load.
type ForecastDay struct {
	Date     string `json:"date"`
	Reviews  int    `json:"reviews"`
	NewCards int    `json:"newCards"`
	Minutes  int    `json:"minutes"`
}

// Forecast is a projection plus its verdict.
type Forecast struct {
	Days []ForecastDay `json:"days"`
	// PeakReviews is the worst single day in the window.
	PeakReviews int `json:"peakReviews"`
	PeakMinutes int `json:"peakMinutes"`
	// SteadyStateReviews is the load once the queue settles, which is the number
	// that actually decides whether a limit is survivable.
	SteadyStateReviews int  `json:"steadyStateReviews"`
	SteadyStateMinutes int  `json:"steadyStateMinutes"`
	BudgetMinutes      int  `json:"budgetMinutes"`
	Sustainable        bool `json:"sustainable"`
	// SuggestedNewPerDay is the largest limit that fits the stated budget.
	SuggestedNewPerDay int `json:"suggestedNewPerDay"`
}

// ForecastInput describes the learner's current position.
type ForecastInput struct {
	// ExistingDue is the count of already-scheduled cards falling due per day,
	// indexed by days from now.
	ExistingDue []int
	NewPerDay   int
	// CardsPerNote is how many directions each new note spawns. This is why the
	// governor counts CARDS and not notes: "20 new a day" with five directions
	// is a hundred cards a day, and a learner who was not told that will drown.
	CardsPerNote     float64
	SecondsPerReview float64
	DailyMinutes     int
	DesiredRetention float64
	Days             int
}

// Forecast projects daily review load.
//
// The model is deliberately simple and slightly pessimistic: each card review
// generates a future review at an interval that grows geometrically, with a
// lapse rate implied by the desired retention. It is not trying to predict a
// specific day's count — it is answering "does this rate of intake converge to
// something you can sustain", and for that a transparent model beats an accurate
// one nobody can reason about.
func ProjectForecast(in ForecastInput) Forecast {
	out := project(in)
	out.SuggestedNewPerDay = suggestLimit(in)
	return out
}

// project runs the simulation WITHOUT computing a suggestion.
//
// Kept separate because suggestLimit bisects over repeated projections: having
// ProjectForecast call suggestLimit and suggestLimit call ProjectForecast is
// unbounded mutual recursion, which is exactly what the first version did.
func project(in ForecastInput) Forecast {
	days := in.Days
	if days <= 0 {
		days = 90
	}
	if in.CardsPerNote <= 0 {
		in.CardsPerNote = 1
	}
	if in.SecondsPerReview <= 0 {
		in.SecondsPerReview = 8
	}
	retention := in.DesiredRetention
	if retention <= 0 || retention >= 1 {
		retention = 0.9
	}

	// Cards in flight, bucketed by the day they next come due.
	load := make([]float64, days+400)
	for i, n := range in.ExistingDue {
		if i < len(load) {
			load[i] += float64(n)
		}
	}

	newCardsPerDay := float64(in.NewPerDay) * in.CardsPerNote
	lapseRate := 1 - retention

	out := Forecast{Days: make([]ForecastDay, 0, days), BudgetMinutes: in.DailyMinutes}
	start := time.Now()

	// Interval growth per successful review. 2.5 is the classic SM-2 factor and
	// is close to what FSRS produces at 0.9 retention; using it here keeps the
	// projection legible.
	const growth = 2.5

	// Desired retention shortens intervals, and THAT is the mechanism by which a
	// higher target costs more reviews. An earlier version modelled retention
	// only as a lapse rate, which made a higher target look cheaper — exactly
	// backwards, and the kind of error that would have made the governor advise
	// the opposite of the truth.
	//
	// On an exponential forgetting curve the interval to reach recall
	// probability R scales with ln(R). Normalising at the 0.9 default gives 1.0
	// there, ~0.49 at 0.95 (half the interval, so twice the reviews) and ~2.1 at
	// 0.80.
	intervalScale := math.Log(retention) / math.Log(0.9)

	for d := range days {
		reviews := load[d] + newCardsPerDay
		minutes := reviews * in.SecondsPerReview / 60

		out.Days = append(out.Days, ForecastDay{
			Date:     start.AddDate(0, 0, d).Format("2006-01-02"),
			Reviews:  int(math.Round(reviews)),
			NewCards: in.NewPerDay,
			Minutes:  int(math.Round(minutes)),
		})
		if int(math.Round(reviews)) > out.PeakReviews {
			out.PeakReviews = int(math.Round(reviews))
			out.PeakMinutes = int(math.Round(minutes))
		}

		// Reschedule: most cards come back later, a lapse fraction comes back soon.
		succeeded := reviews * retention
		lapsed := reviews * lapseRate

		interval := (1 + float64(d)*0.35) * intervalScale // matures over time
		nextSuccess := d + int(math.Max(1, math.Round(interval*growth/2)))
		nextLapse := d + 1

		if nextSuccess < len(load) {
			load[nextSuccess] += succeeded
		}
		if nextLapse < len(load) {
			load[nextLapse] += lapsed
		}
	}

	// Steady state: the mean of the last fortnight, which is where the curve is
	// heading rather than where the initial burst put it.
	tail := out.Days
	if len(tail) > 14 {
		tail = tail[len(tail)-14:]
	}
	sum, sumMin := 0, 0
	for _, d := range tail {
		sum += d.Reviews
		sumMin += d.Minutes
	}
	if len(tail) > 0 {
		out.SteadyStateReviews = sum / len(tail)
		out.SteadyStateMinutes = sumMin / len(tail)
	}

	out.Sustainable = in.DailyMinutes <= 0 || out.SteadyStateMinutes <= in.DailyMinutes
	return out
}

// suggestLimit finds the largest new-card limit whose steady state fits the
// budget, by bisection over the projection.
func suggestLimit(in ForecastInput) int {
	if in.DailyMinutes <= 0 {
		return in.NewPerDay
	}
	probe := in
	probe.Days = 90

	lo, hi := 0, 100
	for lo < hi {
		mid := (lo + hi + 1) / 2
		probe.NewPerDay = mid
		if project(probe).SteadyStateMinutes <= in.DailyMinutes {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// GovernorVerdict is the answer to "may I raise the new-card limit?".
type GovernorVerdict struct {
	Allowed   bool     `json:"allowed"`
	Requested int      `json:"requested"`
	Suggested int      `json:"suggested"`
	Reason    string   `json:"reason"`
	Forecast  Forecast `json:"forecast"`
}

// CheckNewCardLimit evaluates a proposed limit.
//
// It never silently clamps. Either the limit fits, or the caller receives the
// projection and the suggested alternative so the UI can show the curve and let
// the learner decide with the evidence in front of them.
func CheckNewCardLimit(in ForecastInput, requested int) GovernorVerdict {
	probe := in
	probe.NewPerDay = requested
	f := ProjectForecast(probe)

	v := GovernorVerdict{
		Requested: requested,
		Suggested: f.SuggestedNewPerDay,
		Forecast:  f,
		Allowed:   f.Sustainable,
	}
	if v.Allowed {
		v.Reason = "fits"
		return v
	}
	v.Reason = fmt.Sprintf(
		"%d new notes a day settles at about %d reviews (%d min) daily, "+
			"against a stated budget of %d min",
		requested, f.SteadyStateReviews, f.SteadyStateMinutes, in.DailyMinutes)
	return v
}
