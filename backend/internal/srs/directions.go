// Package srs schedules reviews.
//
// It wraps FSRS-6 unmodified and adds exactly one thing: the application of that
// scheduler to TWO independent state spaces — cards and morphological features.
// No scheduling maths is written here. See ADR-003.
package srs

import "sort"

// Direction is one way of reviewing a note. Each carries independent FSRS state,
// because transfer-appropriate processing means you get back the direction you
// trained: practising recognition builds recognition and little else. [R12]
type Direction string

const (
	// Recognize: HU -> L1. The cheapest and weakest; used to bootstrap a new word.
	Recognize Direction = "recognize"
	// Produce: L1 -> HU, typed. Form recall, which is what speaking needs.
	Produce Direction = "produce"
	// Listen: audio -> meaning. Aural recognition is its own skill.
	Listen Direction = "listen"
	// Cloze: the target form blanked in its sentence. The highest-value
	// direction, because it is the only one that grades morphological features.
	Cloze Direction = "cloze"
	// Spell: audio -> typed Hungarian. This is what actually trains a/á and ö/ő;
	// a learner who never types `ő` never internalises it. [R15]
	Spell Direction = "spell"
)

// AllDirections in a stable order, for iteration and for the UI.
var AllDirections = []Direction{Recognize, Produce, Listen, Cloze, Spell}

// Valid reports whether d is a known direction.
func (d Direction) Valid() bool {
	for _, k := range AllDirections {
		if k == d {
			return true
		}
	}
	return false
}

// Productive reports whether the direction requires the learner to produce
// Hungarian rather than recognise it. Productive answers are typed, get a
// morpheme-level diff, and are the ones that transfer to speech.
func (d Direction) Productive() bool {
	return d == Produce || d == Cloze || d == Spell
}

// GradesFeatures reports whether an answer in this direction should emit reviews
// into the morphological state space.
//
// Only cloze does. Producing a whole sentence from an L1 prompt exercises the
// features too, but attributing a failure to a specific suffix is guesswork when
// the learner had to construct everything; blanking exactly one span makes the
// attribution sound. Grading features we cannot cleanly attribute is what would
// turn the mastery heatmap into noise.
func (d Direction) GradesFeatures() bool {
	return d == Cloze
}

// NeedsAudio reports whether the direction cannot be presented without audio.
// These directions are skipped when no TTS is available, rather than shown broken.
func (d Direction) NeedsAudio() bool {
	return d == Listen || d == Spell
}

// Goals re-weight which directions a new note generates.
const (
	GoalReading      = "reading"
	GoalConversation = "conversation"
	GoalExam         = "exam"
)

// goalWeights multiply the configured per-direction weight.
//
// A reading-focused learner does not need to type `ő` from dictation; an
// exam candidate does. Zero means "do not generate this direction at all".
var goalWeights = map[string]map[Direction]float64{
	GoalReading: {
		Recognize: 1.4, Produce: 0.6, Listen: 0.8, Cloze: 1.2, Spell: 0.0,
	},
	GoalConversation: {
		Recognize: 1.0, Produce: 1.3, Listen: 1.3, Cloze: 1.2, Spell: 0.7,
	},
	GoalExam: {
		Recognize: 1.0, Produce: 1.2, Listen: 1.1, Cloze: 1.3, Spell: 1.2,
	},
}

// DirectionPlan is the set of directions to create for a new note, in the order
// they should be introduced.
type DirectionPlan struct {
	Directions []Direction
	Weights    map[Direction]float64
}

// PlanFor computes which directions a new note should spawn.
//
// `enabled` and `configured` come from config.yaml; `goal` scales them; and
// `audioAvailable` removes the directions that cannot be presented. The result
// is ordered by weight so the cheapest useful direction is introduced first.
func PlanFor(goal string, configured map[Direction]float64, audioAvailable bool) DirectionPlan {
	scale, ok := goalWeights[goal]
	if !ok {
		scale = goalWeights[GoalConversation]
	}

	plan := DirectionPlan{Weights: map[Direction]float64{}}
	for _, d := range AllDirections {
		base, present := configured[d]
		if !present || base <= 0 {
			continue
		}
		if d.NeedsAudio() && !audioAvailable {
			continue
		}
		w := base * scale[d]
		if w <= 0 {
			continue
		}
		plan.Directions = append(plan.Directions, d)
		plan.Weights[d] = w
	}

	sort.SliceStable(plan.Directions, func(i, j int) bool {
		return plan.Weights[plan.Directions[i]] > plan.Weights[plan.Directions[j]]
	})
	return plan
}
