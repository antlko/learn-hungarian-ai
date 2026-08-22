package curriculum

import "sort"

// The placement test.
//
// A learner who already speaks some Hungarian should not have to grind through
// the alphabet to reach the conditional, and a learner who does not should not
// be dropped into the middle of the curriculum. So: a short adaptive ladder up
// the DAG, seeding each topic it probes.
//
// Two commitments make this honest rather than a shortcut.
//
//  1. **Placement answers are real answers.** Each probe is a normal drill item,
//     graded through the normal path and written to `feature_review` with
//     origin='placement'. Nothing is fabricated into the FSRS state.
//  2. **The seed carries its own uncertainty.** A topic passed on three items is
//     seeded with an accuracy of three items, not with a forged twenty. The gate
//     then treats the estimate as thin — which it is — and keeps collecting.
//     Inflating the counters would let a lucky placement mark a topic mastered.

// Probes per topic, and how many must be right to count as passed.
//
// Three is a compromise: fewer is noise, more turns placement into homework and
// learners abandon it. Two of three is roughly the gate threshold, which keeps
// placement and the gate saying the same thing.
const (
	ProbesPerTopic = 3
	ProbesToPass   = 2
	// MaxFailures ends the ladder after this many consecutive failed topics.
	// One failure can be a bad item; two in a row is a level.
	MaxFailures = 2
	// MaxTopics bounds the whole test. Twelve topics at three items is
	// thirty-six answers, which is about as long as anyone will sit for.
	MaxTopics = 12
)

// TopicResult is how a learner did on one probed topic.
type TopicResult struct {
	Code     string `json:"code"`
	Answered int    `json:"answered"`
	Correct  int    `json:"correct"`
}

// Passed reports whether the topic clears the probe bar.
func (r TopicResult) Passed() bool {
	return r.Answered >= ProbesPerTopic && r.Correct >= ProbesToPass
}

// Complete reports whether the topic has been fully probed.
func (r TopicResult) Complete() bool { return r.Answered >= ProbesPerTopic }

// Accuracy is the observed share correct, or 0 with nothing answered.
func (r TopicResult) Accuracy() float64 {
	if r.Answered == 0 {
		return 0
	}
	return float64(r.Correct) / float64(r.Answered)
}

// Ladder is the ordered list of topics a placement test walks.
//
// `drillable` filters to the topics that can actually produce items — probing a
// topic we cannot generate for would stall the ladder forever.
func (d *DAG) Ladder(drillable func(Topic) bool) []Topic {
	var out []Topic
	for _, t := range d.topics {
		if drillable != nil && !drillable(t) {
			continue
		}
		if t.NeedsListeningVolume() {
			// Locked behind listening volume regardless of skill, so probing it
			// could only produce a result we would have to ignore. [R15]
			continue
		}
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TeachOrder < out[j].TeachOrder })
	if len(out) > MaxTopics {
		out = out[:MaxTopics]
	}
	return out
}

// PlacementState is where a running test has got to.
type PlacementState struct {
	// Next is the topic to probe now. Valid only when Done is false.
	Next Topic `json:"next"`
	// Remaining counts probes still owed for Next.
	Remaining int  `json:"remaining"`
	Done      bool `json:"done"`
	// Index and Total drive the progress indicator.
	Index int `json:"index"`
	Total int `json:"total"`
}

// NextProbe walks the ladder against the results so far.
//
// The ladder stops early on two consecutive failed topics: continuing past that
// point is asking a beginner about the causative, which is demoralising and
// tells us nothing we did not already know.
func NextProbe(ladder []Topic, results map[string]TopicResult) PlacementState {
	st := PlacementState{Total: len(ladder)}
	failures := 0
	for i, t := range ladder {
		r := results[t.Code]
		if !r.Complete() {
			st.Next = t
			st.Remaining = ProbesPerTopic - r.Answered
			st.Index = i
			return st
		}
		if r.Passed() {
			failures = 0
			continue
		}
		failures++
		if failures >= MaxFailures {
			break
		}
	}
	st.Done = true
	st.Index = len(ladder)
	return st
}

// Seed is the initial state to write for one topic after placement.
type Seed struct {
	Code string `json:"code"`
	// Phase is "interleaved" for a passed topic and "blocked" for a probed but
	// failed one. Never "mastered": nobody masters a topic in three items, and
	// claiming otherwise would skip the maintenance the FSRS state exists for.
	Phase string `json:"phase"`
	// Accuracy and Observations are what was ACTUALLY observed. The gate reads
	// Observations and refuses to act on a thin estimate, which is exactly the
	// behaviour we want out of a three-item probe.
	Accuracy     float64 `json:"accuracy"`
	Observations int     `json:"observations"`
}

// SeedFrom turns placement results into initial topic state.
//
// Topics BEFORE the last passed one are seeded as passed too, even if they were
// not probed: the DAG says they are prerequisites, and a learner who handled the
// conditional has demonstrated the accusative more convincingly than three
// accusative items would. Topics after the stopping point are left untouched and
// open normally through the gate.
func SeedFrom(ladder []Topic, results map[string]TopicResult) []Seed {
	lastPassed := -1
	for i, t := range ladder {
		if results[t.Code].Passed() {
			lastPassed = i
		}
	}

	out := make([]Seed, 0, len(ladder))
	for i, t := range ladder {
		r, probed := results[t.Code]
		switch {
		case r.Passed():
			out = append(out, Seed{
				Code: t.Code, Phase: "interleaved",
				Accuracy: r.Accuracy(), Observations: r.Answered,
			})
		case i < lastPassed:
			// Implied by a later pass. Seeded with zero observations, so the
			// gate knows it is inferred rather than measured and will not, for
			// instance, demote it on the strength of an estimate we never made.
			out = append(out, Seed{Code: t.Code, Phase: "interleaved"})
		case probed && r.Complete():
			out = append(out, Seed{
				Code: t.Code, Phase: "blocked",
				Accuracy: r.Accuracy(), Observations: r.Answered,
			})
		}
	}
	return out
}
