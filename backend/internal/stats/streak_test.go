package stats

import "testing"

// The streak measures OBLIGATIONS MET, and every branch of that sentence has a
// test: activity without clearing the queue does not count, clearing the queue
// without the minutes does not count, an in-progress today does not break the
// run, a gap does.
func TestStreakCountsObligationsNotActivity(t *testing.T) {
	const minutes = 30
	mkDay := func(day string, activeMin int, dueLeft int) Day {
		return Day{Day: day, ActiveMs: int64(activeMin) * 60_000, DueLeft: dueLeft}
	}

	days := []Day{ // newest first, as the store returns them
		mkDay("2026-08-22", 5, 0),  // today: started, not enough minutes yet
		mkDay("2026-08-21", 45, 0), // met
		mkDay("2026-08-20", 40, 0), // met
		mkDay("2026-08-19", 90, 3), // 90 minutes but 3 cards left DUE — not met
		mkDay("2026-08-18", 35, 0), // met
		mkDay("2026-08-17", 31, 0), // met
		mkDay("2026-08-16", 33, 0), // met
	}

	st := ComputeStreak(days, "2026-08-22", minutes)
	if st.TodayMet {
		t.Error("5 minutes is not 30 — today is not met")
	}
	if st.Current != 2 {
		t.Errorf("current = %d, want 2 (the 19th broke the run: due cards were left)", st.Current)
	}
	if st.Best != 3 {
		t.Errorf("best = %d, want the 16th–18th run of 3", st.Best)
	}

	// Today completes: the run extends through it.
	days[0] = mkDay("2026-08-22", 45, 0)
	st = ComputeStreak(days, "2026-08-22", minutes)
	if !st.TodayMet || st.Current != 3 {
		t.Errorf("после выполненного дня: current = %d, met=%v, want 3/true", st.Current, st.TodayMet)
	}
}

// A day with NO row is a day with no work: the streak breaks. This is design,
// not accident — missed days are failure mode №1 in the source plan.
func TestMissingDayBreaksTheStreak(t *testing.T) {
	days := []Day{
		{Day: "2026-08-22", ActiveMs: 60 * 60_000, DueLeft: 0},
		// 2026-08-21 is absent entirely.
		{Day: "2026-08-20", ActiveMs: 60 * 60_000, DueLeft: 0},
		{Day: "2026-08-19", ActiveMs: 60 * 60_000, DueLeft: 0},
	}
	st := ComputeStreak(days, "2026-08-22", 30)
	if st.Current != 1 {
		t.Errorf("current = %d, want 1 — the gap on the 21st broke it", st.Current)
	}
	if st.Best != 2 {
		t.Errorf("best = %d, want 2", st.Best)
	}
}

// Loop 4's attestation is signal for the diagnostic, never a streak input.
func TestHumanAttestationDoesNotFeedTheStreak(t *testing.T) {
	days := []Day{{Day: "2026-08-22", Human: true, ActiveMs: 0, DueLeft: 5}}
	st := ComputeStreak(days, "2026-08-22", 30)
	if st.TodayMet || st.Current != 0 {
		t.Error("attesting live practice must not satisfy the app-side obligations")
	}
}
