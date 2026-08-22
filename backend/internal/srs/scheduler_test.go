package srs

import (
	"testing"
	"time"
)

var base = time.Date(2026, 8, 22, 9, 0, 0, 0, time.UTC)

func TestNewCardIsDueImmediately(t *testing.T) {
	c := NewCardState(base)
	if !c.IsNew() {
		t.Error("a fresh card should report IsNew")
	}
	if c.Due.After(base) {
		t.Errorf("a new card should be due now, got %v", c.Due)
	}
}

func TestReviewAdvancesState(t *testing.T) {
	s := NewScheduler(0.90)
	c := NewCardState(base)

	next, log, err := s.Review(c, Good, base)
	if err != nil {
		t.Fatal(err)
	}
	if next.Reps != 1 {
		t.Errorf("reps: got %d, want 1", next.Reps)
	}
	if !next.Due.After(base) {
		t.Errorf("due should move forward, got %v", next.Due)
	}
	if log.Rating != Good {
		t.Errorf("log rating: got %v", log.Rating)
	}
	if log.ReviewTime != base {
		t.Errorf("log time: got %v, want %v", log.ReviewTime, base)
	}
}

func TestAgainCountsALapseOnAMatureCard(t *testing.T) {
	s := NewScheduler(0.90)
	c := NewCardState(base)

	// Get the card into Review state first.
	now := base
	for range 3 {
		var err error
		c, _, err = s.Review(c, Easy, now)
		if err != nil {
			t.Fatal(err)
		}
		now = c.Due
	}
	before := c.Lapses

	c, _, err := s.Review(c, Again, now)
	if err != nil {
		t.Fatal(err)
	}
	if c.Lapses <= before {
		t.Errorf("forgetting a mature card should count a lapse: %d -> %d", before, c.Lapses)
	}
}

func TestBetterRatingsScheduleFurtherOut(t *testing.T) {
	s := NewScheduler(0.90)
	c := NewCardState(base)
	c, _, _ = s.Review(c, Good, base)
	now := c.Due

	preview, err := s.Preview(c, now)
	if err != nil {
		t.Fatal(err)
	}
	// Again <= Hard <= Good <= Easy. Equality is allowed at short intervals
	// because learning steps are fixed minutes.
	order := []Rating{Again, Hard, Good, Easy}
	for i := 1; i < len(order); i++ {
		prev, cur := preview[order[i-1]], preview[order[i]]
		if cur.Before(prev) {
			t.Errorf("%v (%v) scheduled before %v (%v)", order[i], cur, order[i-1], prev)
		}
	}
}

func TestRetrievabilityDecaysWithTime(t *testing.T) {
	s := NewScheduler(0.90)
	c := NewCardState(base)
	c, _, _ = s.Review(c, Easy, base)

	justAfter := s.Retrievability(c, c.LastReview.Add(time.Hour))
	muchLater := s.Retrievability(c, c.LastReview.Add(365*24*time.Hour))

	if justAfter <= muchLater {
		t.Errorf("retrievability should decay: %v right after, %v a year later",
			justAfter, muchLater)
	}
	if justAfter > 1.0 || muchLater < 0 {
		t.Errorf("retrievability out of range: %v, %v", justAfter, muchLater)
	}
}

func TestKnownDependsOnTimeNotJustStability(t *testing.T) {
	// The point of using retrievability rather than a status flag: the same
	// stored card is "known" today and not known a year from now.
	s := NewScheduler(0.90)
	c := NewCardState(base)
	c, _, _ = s.Review(c, Easy, base)

	if !s.Known(c, 0.80, c.LastReview.Add(time.Minute)) {
		t.Error("a just-reviewed card should count as known")
	}
	if s.Known(c, 0.80, c.LastReview.Add(5*365*24*time.Hour)) {
		t.Error("a card unreviewed for five years should not count as known")
	}
}

func TestNewCardHasZeroRetrievability(t *testing.T) {
	s := NewScheduler(0.90)
	if got := s.Retrievability(NewCardState(base), base); got != 0 {
		t.Errorf("an unreviewed card should have 0 retrievability, got %v", got)
	}
}

func TestInvalidRatingIsRejected(t *testing.T) {
	s := NewScheduler(0.90)
	if _, _, err := s.Review(NewCardState(base), Rating(9), base); err == nil {
		t.Error("want an error for an out-of-range rating")
	}
}

func TestDesiredRetentionIsHonoured(t *testing.T) {
	// A higher retention target must schedule shorter intervals.
	low := NewScheduler(0.80)
	high := NewScheduler(0.95)

	cl, _, _ := low.Review(NewCardState(base), Good, base)
	ch, _, _ := high.Review(NewCardState(base), Good, base)
	for range 4 {
		cl, _, _ = low.Review(cl, Good, cl.Due)
		ch, _, _ = high.Review(ch, Good, ch.Due)
	}
	if !ch.Due.Before(cl.Due) && ch.ScheduledDays >= cl.ScheduledDays {
		t.Errorf("retention 0.95 should schedule sooner than 0.80: %d vs %d days",
			ch.ScheduledDays, cl.ScheduledDays)
	}
}
