package hvpt

import "tanulom/internal/drill"

// Served-but-unanswered trials.
//
// The same store the grammar drill uses, for the same reason and under the same
// ADR-008 argument: the answer — which member was played — must never reach the
// client, or the trial stops measuring the ear and starts measuring the
// devtools. This file used to be a copy of internal/drill/pending.go and had
// already drifted from it, so it is now that store instantiated for trials;
// the reasoning lives there, with the implementation.

// Pending holds trials that have been served and not yet answered.
type Pending = drill.Served[Trial]

// NewPending builds an empty store.
func NewPending() *Pending {
	return drill.NewServed(
		func(t Trial, id string) Trial { t.ID = id; return t },
		func(t Trial) Trial { return t.WithPlayed("") },
	)
}
