package srs

import (
	"sort"
	"time"
)

// Dual-space review emission.
//
// One answer produces ONE card review plus N FEATURE reviews — and only for the
// features the item actually required. That last clause is the whole discipline:
// a cloze on `házaimban` blanks a span carrying Case=Ine, Number[psed]=Plur and
// Person[psor]=1, so those three are graded. The `Number=Sing` on some other
// token in the sentence is not, because the learner never had to choose it.
//
// Grading unrequired features is what would turn the mastery heatmap from a
// diagnostic into noise. See docs/ARCHITECTURE.md §5.2.

// Outcome is what happened on one answer, before it becomes ratings.
type Outcome struct {
	// Correct is the learner's own grade for receptive directions, or the
	// computed result of comparing typed input for productive ones.
	Correct bool
	// UsedHint means the answer arrived only after a reveal. It caps the rating,
	// because retrieval that needed help was not retrieval.
	UsedHint bool
	// Duration is how long the answer took.
	Duration time.Duration
	// MedianDuration is this learner's typical answer time, used to tell fast
	// recall from slow. Zero disables the latency distinction.
	MedianDuration time.Duration
	// PartiallyCorrect marks a typed answer that differed from the target only
	// in ways the diff classified as minor (an accent, a single character).
	PartiallyCorrect bool
}

// DeriveRating turns an outcome into an FSRS grade.
//
// Latency matters because speed is the observable side of automaticity. For
// vowel harmony in particular the target is a motor skill: a learner who answers
// correctly but slowly has not automated it, and scheduling as though they had
// would stop the practice too early. See [R15].
func DeriveRating(o Outcome) Rating {
	if !o.Correct {
		return Again
	}
	if o.UsedHint || o.PartiallyCorrect {
		return Hard
	}
	if o.MedianDuration > 0 && o.Duration > 0 {
		switch {
		case o.Duration < o.MedianDuration:
			return Easy
		case o.Duration > 2*o.MedianDuration:
			// Correct but laboured: it will need to come back sooner.
			return Hard
		}
	}
	return Good
}

// FeatureGrade is one feature review to be written.
type FeatureGrade struct {
	FeatureID string
	Rating    Rating
	Correct   bool
}

// RequiredFeatures returns the feature ids an item genuinely tested.
//
// `targetFeats` are the features on the blanked span. `wrongFeature`, when set,
// names the single feature the answer diff identified as the error — that one is
// marked incorrect and the rest are left ungraded, because a learner who got the
// case right and the harmony wrong should not lose credit for the case.
func RequiredFeatures(targetFeats []string, o Outcome, wrongFeature string) []FeatureGrade {
	if len(targetFeats) == 0 {
		return nil
	}
	ids := append([]string(nil), targetFeats...)
	sort.Strings(ids)

	out := make([]FeatureGrade, 0, len(ids))

	// A precisely attributed error: blame only the feature that was wrong.
	if !o.Correct && wrongFeature != "" {
		for _, id := range ids {
			if id != wrongFeature {
				continue
			}
			out = append(out, FeatureGrade{FeatureID: id, Rating: Again, Correct: false})
		}
		if len(out) > 0 {
			return out
		}
		// The named feature was not among the required ones; fall through and
		// grade them all rather than silently recording nothing.
	}

	rating := DeriveRating(o)
	for _, id := range ids {
		out = append(out, FeatureGrade{FeatureID: id, Rating: rating, Correct: o.Correct})
	}
	return out
}

// RollingAccuracy folds one outcome into an exponentially weighted accuracy.
//
// An EMA rather than a correct/total pair, for a concrete reason: ageing a pair
// of integers truncates. A 1/20 ratio decays to 0/19 and the numerator can never
// climb back, so a learner who turns their performance around stays locked below
// the gate forever. Floating point avoids that, needs no history table, and
// responds to a change in performance within roughly one window.
//
// `size` is the nominal window (`drill.gateWindow`); `total` counts observations
// so callers can refuse to act on an estimate built from three answers.
func RollingAccuracy(accuracy float64, total, size int, wasCorrect bool) (float64, int) {
	if size < 2 {
		size = 2
	}
	observed := 0.0
	if wasCorrect {
		observed = 1.0
	}
	if total == 0 {
		return observed, 1
	}
	alpha := 2.0 / (float64(size) + 1.0)
	return accuracy + alpha*(observed-accuracy), total + 1
}

// GateReady reports whether a rolling estimate is based on enough observations
// to act on. Below this the drill scheduler keeps the topic blocked regardless
// of the accuracy figure, because three lucky answers are not evidence.
func GateReady(total, size int) bool { return total >= size }

// UpdateMedian folds a new duration into a running median estimate.
//
// A true median needs the full history; this is the standard streaming
// approximation, which is accurate enough to separate "fast" from "laboured"
// and costs one integer.
func UpdateMedian(currentMs, observedMs int64) int64 {
	if observedMs <= 0 {
		return currentMs
	}
	if currentMs <= 0 {
		return observedMs
	}
	step := currentMs / 16
	if step < 1 {
		step = 1
	}
	if observedMs > currentMs {
		return currentMs + step
	}
	return currentMs - step
}
