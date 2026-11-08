package srs

import (
	"fmt"
	"time"

	fsrs "github.com/open-spaced-repetition/go-fsrs/v4"
)

// Rating mirrors the FSRS grades. Re-exported so the rest of the app never
// imports the FSRS package directly and the dependency stays swappable.
type Rating int

const (
	Again Rating = 1
	Hard  Rating = 2
	Good  Rating = 3
	Easy  Rating = 4
)

func (r Rating) Valid() bool { return r >= Again && r <= Easy }

func (r Rating) String() string {
	switch r {
	case Again:
		return "again"
	case Hard:
		return "hard"
	case Good:
		return "good"
	case Easy:
		return "easy"
	}
	return "unknown"
}

// CardState is the persisted FSRS state of one card or one feature. The two
// state spaces store the same shape because they run the same scheduler.
type CardState struct {
	Due           time.Time
	Stability     float64
	Difficulty    float64
	ScheduledDays uint64
	Reps          uint64
	Lapses        uint64
	State         int
	LastReview    time.Time
	// RemainingSteps is FSRS-6's position within the learning/relearning steps.
	// It MUST round-trip through storage: dropped, the scheduler sees 0 on every
	// load and Again/Hard on a Learning card graduates it a day out instead of
	// repeating the minutes-scale step. Migration 00011, ADR-020.
	RemainingSteps int
}

// IsNew reports whether the card has never been reviewed.
func (c CardState) IsNew() bool { return c.State == int(fsrs.New) && c.Reps == 0 }

// Scheduler wraps FSRS-6.
type Scheduler struct {
	fsrs   *fsrs.FSRS
	params fsrs.Parameters
}

// NewScheduler builds a scheduler at the given desired retention.
//
// v1.0 ships FSRS-6 DEFAULT parameters for every learner: go-fsrs's optimizer is
// still in development, so per-user parameter fitting is deferred to v1.3 via an
// optional sidecar. This limitation is surfaced in the UI, not just in docs.
// See ADR-003 and .claude/research/03-fsrs.md.
func NewScheduler(desiredRetention float64) *Scheduler {
	p := fsrs.DefaultParam()
	if desiredRetention > 0 {
		p.RequestRetention = desiredRetention
	}
	// Fuzz spreads due dates so a big import does not all come back on one day.
	p.EnableFuzz = true
	return &Scheduler{fsrs: fsrs.NewFSRS(p), params: p}
}

// DesiredRetention returns the configured target.
func (s *Scheduler) DesiredRetention() float64 { return s.params.RequestRetention }

// Retrievability estimates the probability of recalling this item right now.
//
// This is what makes an element count as "known" for n+1 selection: a lemma with
// 200 days of stability reviewed yesterday is known; the same lemma unreviewed
// for a year is not, though nothing about the stored card changed.
//
// It delegates to the library rather than reimplementing the forgetting curve,
// so it cannot drift out of step with the scheduler that produced the state.
func (s *Scheduler) Retrievability(c CardState, now time.Time) float64 {
	if c.Stability <= 0 || c.LastReview.IsZero() {
		return 0
	}
	r, err := s.fsrs.Retrievability(toFSRS(c, now), now)
	if err != nil {
		return 0
	}
	return r
}

// Known reports whether an element counts as known at a retrievability threshold.
// The threshold is `mining.knownThreshold` in config.yaml.
func (s *Scheduler) Known(c CardState, threshold float64, now time.Time) bool {
	return s.Retrievability(c, now) >= threshold
}

// Review applies a rating and returns the new state plus the log entry.
func (s *Scheduler) Review(cur CardState, rating Rating, now time.Time) (CardState, LogEntry, error) {
	if !rating.Valid() {
		return cur, LogEntry{}, fmt.Errorf("invalid rating %d", rating)
	}
	card := toFSRS(cur, now)

	info, err := s.fsrs.Next(card, now, fsrs.Rating(rating))
	if err != nil {
		return cur, LogEntry{}, fmt.Errorf("schedule review: %w", err)
	}

	next := fromFSRS(info.Card)
	log := LogEntry{
		Rating:          rating,
		State:           int(info.ReviewLog.State),
		Due:             info.ReviewLog.Due,
		Stability:       info.ReviewLog.Stability,
		Difficulty:      info.ReviewLog.Difficulty,
		ScheduledDays:   info.ReviewLog.ScheduledDays,
		ElapsedDays:     elapsedDays(cur.LastReview, now),
		LastElapsedDays: cur.ScheduledDays,
		ReviewTime:      now,
	}
	return next, log, nil
}

// PreviewNext returns the whole state each rating would produce, without
// storing anything.
//
// Preview reads it for the due dates, and Memory reads it to answer "would
// this answer take the card off the minutes scale". Both go through here so
// the two can never disagree about what the scheduler would actually do —
// the alternative is a second copy of FSRS's step logic in our code, which
// ADR-003 exists to forbid.
func (s *Scheduler) PreviewNext(cur CardState, now time.Time) (map[Rating]CardState, error) {
	out := make(map[Rating]CardState, 4)
	for _, r := range []Rating{Again, Hard, Good, Easy} {
		next, _, err := s.Review(cur, r, now)
		if err != nil {
			return nil, err
		}
		out[r] = next
	}
	return out, nil
}

// Preview returns what each rating would schedule, for the "1d / 3d / 8d / 20d"
// hints under the answer buttons.
func (s *Scheduler) Preview(cur CardState, now time.Time) (map[Rating]time.Time, error) {
	states, err := s.PreviewNext(cur, now)
	if err != nil {
		return nil, err
	}
	out := make(map[Rating]time.Time, 4)
	for r, st := range states {
		out[r] = st.Due
	}
	return out, nil
}

// LearningSteps and RelearningSteps are the minute-scale delays a card walks
// before it starts being scheduled in days — FSRS-6's defaults, {1, 10} and
// {10}.
//
// Exposed so the UI can say how many are left rather than printing a bare
// «вернётся через ~10 мин» and leaving the learner to guess what ends it.
// Read off the library's own parameters: restating the numbers here is how
// the screen and the scheduler drift apart.
func (s *Scheduler) LearningSteps() []float64   { return s.params.LearningSteps }
func (s *Scheduler) RelearningSteps() []float64 { return s.params.RelearningSteps }

// LogEntry is one row of review_log.
type LogEntry struct {
	Rating          Rating
	State           int
	Due             time.Time
	Stability       float64
	Difficulty      float64
	ScheduledDays   uint64
	ElapsedDays     uint64
	LastElapsedDays uint64
	ReviewTime      time.Time
	DurationMs      int64
	Answer          string
	Correct         bool
}

// NewCardState returns the state of a never-reviewed card, due immediately.
func NewCardState(now time.Time) CardState {
	return fromFSRS(fsrs.NewCard(now))
}

func toFSRS(c CardState, now time.Time) fsrs.Card {
	due := c.Due
	if due.IsZero() {
		due = now
	}
	return fsrs.Card{
		Due:            due,
		Stability:      c.Stability,
		Difficulty:     c.Difficulty,
		ScheduledDays:  c.ScheduledDays,
		Reps:           c.Reps,
		Lapses:         c.Lapses,
		State:          fsrs.State(c.State),
		LastReview:     c.LastReview,
		RemainingSteps: c.RemainingSteps,
	}
}

func fromFSRS(c fsrs.Card) CardState {
	return CardState{
		Due:            c.Due,
		Stability:      c.Stability,
		Difficulty:     c.Difficulty,
		ScheduledDays:  c.ScheduledDays,
		Reps:           c.Reps,
		Lapses:         c.Lapses,
		State:          int(c.State),
		LastReview:     c.LastReview,
		RemainingSteps: c.RemainingSteps,
	}
}

func elapsedDays(last, now time.Time) uint64 {
	if last.IsZero() {
		return 0
	}
	d := now.Sub(last).Hours() / 24
	if d < 0 {
		return 0
	}
	return uint64(d)
}
