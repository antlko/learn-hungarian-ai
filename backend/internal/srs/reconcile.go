package srs

import (
	"database/sql"
	"fmt"

	"tanulom/internal/db"
)

// One-time repair after migration 00011 (ADR-020).
//
// Until RemainingSteps was persisted, every card and feature loaded from the
// database re-entered the scheduler with 0 remaining learning steps, so
// Again/Hard on a Learning or Relearning item graduated it days out instead of
// repeating the minutes-scale step. The stored state is therefore wrong
// wherever that path was taken — and the replay invariant ("the log replays to
// stored state bit for bit") broke the moment the mapping was fixed, because
// live answers had folded through the lossy version.
//
// ReconcileSteps restores the invariant the way undo already trusts: the log is
// the source of truth and state is a cache of it. It replays every card from
// review_log and every feature from feature_review with the complete scheduler
// and overwrites what differs. Deterministic (fuzz is seeded from the stored
// millisecond timestamps), so running it twice changes nothing.
//
// It deliberately replays under the CURRENT desiredRetention and gate window —
// the same assumption UndoLast has always made about replaying under the
// current engine.

// ReconcileSteps replays all card and feature history, overwriting stored state
// that disagrees. Returns how many cards and features actually changed.
func ReconcileSteps(sqlDB *sql.DB, sched *Scheduler, gateWindow int) (int, int, error) {
	tx, err := sqlDB.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after a successful commit

	store := &Store{db: sqlDB}

	cardsChanged, err := reconcileCards(tx, store, sched)
	if err != nil {
		return 0, 0, err
	}
	featuresChanged, err := reconcileFeatures(tx, store, sched, gateWindow)
	if err != nil {
		return 0, 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit steps reconciliation: %w", err)
	}
	return cardsChanged, featuresChanged, nil
}

func reconcileCards(tx *sql.Tx, store *Store, sched *Scheduler) (int, error) {
	ids, err := distinctInt64s(tx, `SELECT DISTINCT card_id FROM review_log ORDER BY card_id`)
	if err != nil {
		return 0, fmt.Errorf("list reviewed cards: %w", err)
	}
	changed := 0
	for _, cardID := range ids {
		steps, err := cardSteps(tx, cardID)
		if err != nil {
			return changed, err
		}
		replayed, err := sched.ReplayCard(steps)
		if err != nil {
			return changed, fmt.Errorf("card %d: %w", cardID, err)
		}
		stored, err := cardStateTx(tx, cardID)
		if err != nil {
			return changed, err
		}
		if sameCardState(stored, replayed) {
			continue
		}
		if err := store.SaveCard(tx, Card{ID: cardID, State: replayed}); err != nil {
			return changed, err
		}
		changed++
	}
	return changed, nil
}

func reconcileFeatures(tx *sql.Tx, store *Store, sched *Scheduler, gateWindow int) (int, error) {
	rows, err := tx.Query(`SELECT DISTINCT user_id, feature_id FROM feature_review ORDER BY user_id, feature_id`)
	if err != nil {
		return 0, fmt.Errorf("list reviewed features: %w", err)
	}
	type key struct {
		userID    int64
		featureID string
	}
	var keys []key
	for rows.Next() {
		var k key
		if err := rows.Scan(&k.userID, &k.featureID); err != nil {
			rows.Close()
			return 0, err
		}
		keys = append(keys, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	changed := 0
	for _, k := range keys {
		steps, err := featureSteps(tx, k.userID, k.featureID)
		if err != nil {
			return changed, err
		}
		replayed, err := sched.ReplayFeature(k.featureID, steps, gateWindow)
		if err != nil {
			return changed, fmt.Errorf("feature %s: %w", k.featureID, err)
		}
		stored, err := store.FeatureState(tx, k.userID, k.featureID, replayed.State.Due)
		if err != nil {
			return changed, err
		}
		if sameCardState(stored.State, replayed.State) &&
			stored.Accuracy == replayed.Accuracy && stored.Total == replayed.Total &&
			stored.MedianMs == replayed.MedianMs {
			continue
		}
		if err := store.SaveFeatureState(tx, k.userID, replayed); err != nil {
			return changed, err
		}
		changed++
	}
	return changed, nil
}

// cardStateTx loads a card's stored state inside the transaction. Store.Card
// reads from the pool, which with SetMaxOpenConns(1) deadlocks under an open
// transaction — see the querier comment in store.go.
func cardStateTx(tx *sql.Tx, cardID int64) (CardState, error) {
	var st CardState
	var due string
	var last sql.NullString
	err := tx.QueryRow(`
		SELECT due, stability, difficulty, scheduled_days, reps, lapses, state,
		       last_review, remaining_steps
		  FROM card WHERE id = ?`, cardID,
	).Scan(&due, &st.Stability, &st.Difficulty, &st.ScheduledDays, &st.Reps,
		&st.Lapses, &st.State, &last, &st.RemainingSteps)
	if err != nil {
		return CardState{}, fmt.Errorf("load card %d state: %w", cardID, err)
	}
	st.Due = db.ParseTime(sql.NullString{String: due, Valid: true})
	st.LastReview = db.ParseTime(last)
	return st, nil
}

// sameCardState compares through the storage format: times round-trip through
// db.FmtTime, so a nanosecond difference the database cannot hold is not a
// difference.
func sameCardState(a, b CardState) bool {
	return db.FmtTime(a.Due) == db.FmtTime(b.Due) &&
		a.Stability == b.Stability &&
		a.Difficulty == b.Difficulty &&
		a.ScheduledDays == b.ScheduledDays &&
		a.Reps == b.Reps &&
		a.Lapses == b.Lapses &&
		a.State == b.State &&
		db.FmtTime(a.LastReview) == db.FmtTime(b.LastReview) &&
		a.RemainingSteps == b.RemainingSteps
}

func distinctInt64s(tx *sql.Tx, query string) ([]int64, error) {
	rows, err := tx.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
