package srs

import (
	"fmt"
	"time"
)

// Replaying history through the scheduler.
//
// The review log is complete on purpose: replaying it must reproduce the stored
// FSRS state bit for bit. That property is what makes two things safe:
//
//   - UNDO. Reverting an answer is "delete the last log row and replay the
//     rest" — no stored-previous-state bookkeeping that can drift, no partial
//     rollback bugs. The log is the source of truth; state is a cache of it.
//   - The property test in replay_test.go, which catches any change that breaks
//     the correspondence — including an FSRS upgrade that alters scheduling.
//
// Determinism note: interval fuzz is seeded from (review time, reps,
// difficulty × stability), so replaying with the exact stored timestamps
// reproduces even the fuzzed due dates. This is why review_time is stored to
// the millisecond.
//
// Bit for bit holds PER ARCHITECTURE. go-fsrs draws the fuzz from a
// floating-point generator (Alea), and Go fuses multiply-adds on arm64 but not
// on amd64, so the same seed draws a different number on each — the generator
// is a chaotic map, and one ulp becomes a different draw — while stability and
// difficulty differ in their last bit. A data directory moved between an amd64
// and an arm64 host still replays, but an undo there lands a fuzzed due date
// within the fuzz range of the stored one, not on it. A test must therefore
// never pin a fuzzed day: a difference that lives only in the fuzz passes on
// one CPU and fails on the other (reconcile_test.go, which failed only in CI).

// ReplayStep is one recorded answer, the minimum needed to re-derive state.
type ReplayStep struct {
	Rating     Rating
	ReviewTime time.Time
	Correct    bool
	DurationMs int64
}

// ReplayCard folds a card's full history into its state.
func (s *Scheduler) ReplayCard(steps []ReplayStep) (CardState, error) {
	if len(steps) == 0 {
		// Never reviewed: fresh state due at zero time; the caller decides what
		// "now" means for a card returned to newness.
		return NewCardState(time.Time{}), nil
	}
	state := NewCardState(steps[0].ReviewTime)
	for i, st := range steps {
		next, _, err := s.Review(state, st.Rating, st.ReviewTime)
		if err != nil {
			return CardState{}, fmt.Errorf("replay step %d: %w", i, err)
		}
		state = next
	}
	return state, nil
}

// ReplayFeature folds a feature's full history into its state — the FSRS half
// AND the rolling counters the gate reads. `gateWindow` is the EMA window; the
// current configured value is used, which means a window change re-ages history
// under the new setting. That is the intended semantics of changing it.
func (s *Scheduler) ReplayFeature(featureID string, steps []ReplayStep, gateWindow int) (FeatureState, error) {
	fs := FeatureState{FeatureID: featureID, State: NewCardState(time.Time{})}
	if len(steps) == 0 {
		return fs, nil
	}
	fs.State = NewCardState(steps[0].ReviewTime)
	for i, st := range steps {
		next, _, err := s.Review(fs.State, st.Rating, st.ReviewTime)
		if err != nil {
			return fs, fmt.Errorf("replay feature step %d: %w", i, err)
		}
		fs.State = next
		fs.Accuracy, fs.Total = RollingAccuracy(fs.Accuracy, fs.Total, gateWindow, st.Correct)
		if st.Correct {
			fs.MedianMs = UpdateMedian(fs.MedianMs, st.DurationMs)
		}
	}
	return fs, nil
}
