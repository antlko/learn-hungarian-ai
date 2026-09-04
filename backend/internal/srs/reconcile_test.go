package srs

import (
	"testing"
	"time"
)

// legacyReview reproduces what the pre-00011 code stored: each answer folded
// through a load that had LOST RemainingSteps. The scheduler itself was always
// correct — the zero arrived through the lossy mapping.
func legacyReview(t *testing.T, e *Engine, cardID int64, state CardState, r Rating, correct bool, now time.Time) CardState {
	t.Helper()
	state.RemainingSteps = 0 // the load dropped it
	next, log, err := e.scheduler.Review(state, r, now)
	if err != nil {
		t.Fatal(err)
	}
	log.Correct = correct
	if err := e.store.SaveCard(nil, Card{ID: cardID, State: next}); err != nil {
		t.Fatal(err)
	}
	if err := e.store.AppendLog(nil, cardID, log); err != nil {
		t.Fatal(err)
	}
	return next
}

// ReconcileSteps turns a database written by the lossy code into exactly what a
// fresh replay of its log produces — and a second run touches nothing.
func TestReconcileRepairsLossyHistory(t *testing.T) {
	e, uid := newEngine(t)
	t0 := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)
	cardID := oneCardNote(t, e, uid, t0)

	// Good puts the card on its 10-minute step; the legacy fold then wrongly
	// graduates it on the Again that follows. Two more answers on top.
	chain := []struct {
		r   Rating
		gap time.Duration
	}{
		{Good, 0},
		{Again, 10 * time.Minute},
		{Good, time.Minute},
		{Good, 24 * time.Hour},
	}
	state := NewCardState(t0)
	now := t0
	var steps []ReplayStep
	for _, st := range chain {
		now = now.Add(st.gap)
		state = legacyReview(t, e, cardID, state, st.r, st.r != Again, now)
		steps = append(steps, ReplayStep{Rating: st.r, ReviewTime: now, Correct: st.r != Again})
	}

	// The lossy fold and the true replay must actually disagree, or this test
	// pins nothing.
	truth, err := e.scheduler.ReplayCard(steps)
	if err != nil {
		t.Fatal(err)
	}
	if sameCardState(state, truth) {
		t.Fatal("setup failed: the lossy chain coincides with the true replay")
	}

	// A feature written the same lossy way.
	fstate := FeatureState{FeatureID: "Case=Ine", State: NewCardState(t0)}
	fnow := t0
	var fsteps []ReplayStep
	for _, st := range chain {
		fnow = fnow.Add(st.gap)
		fstate.State.RemainingSteps = 0
		next, _, err := e.scheduler.Review(fstate.State, st.r, fnow)
		if err != nil {
			t.Fatal(err)
		}
		fstate.State = next
		correct := st.r != Again
		fstate.Accuracy, fstate.Total = RollingAccuracy(fstate.Accuracy, fstate.Total, 20, correct)
		if err := e.store.SaveFeatureState(nil, uid, fstate); err != nil {
			t.Fatal(err)
		}
		if err := e.store.AppendFeatureReview(nil, uid, FeatureGrade{
			FeatureID: "Case=Ine", Rating: st.r, Correct: correct,
		}, cardID, 0, fnow, "review"); err != nil {
			t.Fatal(err)
		}
		fsteps = append(fsteps, ReplayStep{Rating: st.r, ReviewTime: fnow, Correct: correct})
	}

	cards, features, err := ReconcileSteps(e.store.db, e.scheduler, 20)
	if err != nil {
		t.Fatal(err)
	}
	if cards != 1 {
		t.Fatalf("expected exactly the one lossy card to change, got %d", cards)
	}
	if features != 1 {
		t.Fatalf("expected exactly the one lossy feature to change, got %d", features)
	}

	repaired, err := e.store.Card(uid, cardID)
	if err != nil {
		t.Fatal(err)
	}
	if !sameCardState(repaired.State, truth) {
		t.Fatalf("reconcile did not land on the replay truth:\nstored %+v\nreplay %+v",
			repaired.State, truth)
	}

	ftruth, err := e.scheduler.ReplayFeature("Case=Ine", fsteps, 20)
	if err != nil {
		t.Fatal(err)
	}
	frepaired, err := e.store.FeatureState(nil, uid, "Case=Ine", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !sameCardState(frepaired.State, ftruth.State) || frepaired.Total != ftruth.Total {
		t.Fatalf("feature reconcile diverged:\nstored %+v\nreplay %+v", frepaired, ftruth)
	}

	// Idempotence: replay is deterministic, so a second pass changes nothing.
	cards, features, err = ReconcileSteps(e.store.db, e.scheduler, 20)
	if err != nil {
		t.Fatal(err)
	}
	if cards != 0 || features != 0 {
		t.Fatalf("second reconcile should be a no-op, changed %d cards / %d features", cards, features)
	}
}
