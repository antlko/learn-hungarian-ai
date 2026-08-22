package drill

import (
	"time"

	"tanulom/internal/curriculum"
)

// Resolving the whole curriculum for one learner.
//
// The gate in schedule.go answers "what phase is THIS topic in". This file
// answers it for the graph: a topic's prerequisites are met only once they have
// themselves cleared the competence floor, so the phases have to be computed in
// teach order, in one pass.
//
// A prerequisite counts as met at `interleaved`, not at `mastered`. Requiring
// mastery of every ancestor would serialise a two-year curriculum into a crawl
// for no evidential reason: R5's floor is about whether the learner can absorb
// the load of the topic in front of them, and clearing the floor upstream is
// what establishes that. Mastery is a maintenance state, not a gate.

// TopicState is one topic, fully resolved: where the learner is, what the gate
// says, and whether we can actually generate items for it.
type TopicState struct {
	Topic        curriculum.Topic `json:"topic"`
	Progress     Progress         `json:"progress"`
	Decision     Decision         `json:"decision"`
	Availability Availability     `json:"availability"`
	// Unlocks are the topics this one opens, so the map can show consequences.
	Unlocks []string `json:"unlocks"`
}

// Phase is shorthand for the resolved phase.
func (t TopicState) Phase() Phase { return t.Decision.Phase }

// ResolveInput is everything outside the DAG that the resolution depends on.
type ResolveInput struct {
	Progress map[string]Progress
	Config   GateConfig
	// ListeningHours is the learner's accumulated listening volume from the
	// listen_log ledger; NeedListeningHours is
	// `curriculum.focusUnlockListeningHours`. [R15]
	ListeningHours     float64
	NeedListeningHours float64
	// BlockedLevels are CEFR levels shut by a failed checkpoint — the source
	// plan's "правило отката" ([R14]): a failed assessment stops advance beyond
	// its level until it is retaken and passed. Repair happens below the line,
	// which is why lower levels stay open.
	BlockedLevels map[string]bool
	// Kinds reports item availability per topic. Optional: pass nil to skip.
	Kinds func(curriculum.Topic) Availability
}

// Resolve computes every topic's state in teach order.
func Resolve(dag *curriculum.DAG, in ResolveInput) []TopicState {
	if dag == nil {
		return nil
	}
	resolved := make(map[string]Phase, len(dag.Topics()))
	out := make([]TopicState, 0, len(dag.Topics()))

	for _, t := range dag.Topics() {
		p := in.Progress[t.Code]
		p.TopicCode = t.Code
		if p.Phase == "" {
			p.Phase = Locked
		}

		avail := Availability{Available: true}
		if in.Kinds != nil {
			avail = in.Kinds(t)
		}

		g := Gate{MotorSkill: t.MotorSkill(), Drillable: avail.Available,
			CheckpointBlocked: in.BlockedLevels[t.Level]}
		for _, code := range t.PrereqCodes {
			if !resolved[code].Opens() {
				g.UnmetPrereqs = append(g.UnmetPrereqs, code)
			}
		}
		if t.NeedsListeningVolume() {
			g.Listening = &ListeningGate{
				HeardHours: in.ListeningHours,
				NeedHours:  in.NeedListeningHours,
			}
		}

		d := Evaluate(p, g, in.Config)
		resolved[t.Code] = d.Phase

		out = append(out, TopicState{
			Topic: t, Progress: p, Decision: d,
			Availability: avail, Unlocks: dag.Unlocks(t.Code),
		})
	}
	return out
}

// Persist writes back any topic whose phase the resolution changed.
//
// Phases move as a side effect of OTHER topics advancing — clearing the past
// tense unlocks the definite conjugation — so a resolution can legitimately
// change rows the learner never touched. Only actual changes are written, so a
// page load does not rewrite the whole table.
func Persist(store *Store, userID int64, states []TopicState, stored map[string]Progress, now time.Time) error {
	for _, st := range states {
		prev, existed := stored[st.Topic.Code]
		if existed && prev.Phase == st.Decision.Phase {
			continue
		}
		if !existed && st.Decision.Phase == Locked {
			// A locked topic the learner has never touched needs no row: absent
			// and locked mean the same thing, and writing 28 rows on first load
			// would just be noise.
			continue
		}
		p := Apply(st.Progress, st.Decision, now)
		if err := store.SaveProgress(nil, userID, p); err != nil {
			return err
		}
	}
	return nil
}

// UnlockedTopics returns the codes a learner may work on right now.
func UnlockedTopics(states []TopicState) map[string]bool {
	out := map[string]bool{}
	for _, st := range states {
		if st.Decision.Phase.Servable() {
			out[st.Topic.Code] = true
		}
	}
	return out
}

// Current picks the topic a learner should work on next.
//
// The first servable topic in teach order that is not yet mastered — which is
// the curriculum's own answer, and the reason the pack orders topics at all. A
// fully mastered curriculum falls back to the last topic, so the screen always
// has something to show.
func Current(states []TopicState) (TopicState, bool) {
	var last TopicState
	found := false
	for _, st := range states {
		if !st.Decision.Phase.Servable() || !st.Availability.Available {
			continue
		}
		last, found = st, true
		if st.Decision.Phase != Mastered {
			return st, true
		}
	}
	return last, found
}
