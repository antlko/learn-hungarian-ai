package hvpt

import (
	"database/sql"
	"fmt"

	"tanulom/internal/db"
	"tanulom/internal/srs"
)

// One-time repair after migration 00011 — the ear trainer's half of ADR-020.
//
// Contrast state runs through the same FSRS scheduler as cards, so the lost
// RemainingSteps field broke it the same way: a wrong trial on a contrast still
// in Learning graduated it a day out instead of repeating the 10-minute step.
// hvpt_trial carries millisecond timestamps and the rating rule is mechanical
// (wrong -> Again, right -> Good — the exact rule Store.Answer applies), so the
// trial log replays deterministically.

// ReconcileSteps replays every contrast from hvpt_trial with the complete
// scheduler, overwriting stored state that disagrees. Returns how many changed.
func ReconcileSteps(sqlDB *sql.DB, sched *srs.Scheduler, gateWindow int) (int, error) {
	tx, err := sqlDB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after a successful commit

	type key struct {
		userID   int64
		contrast string
	}
	rows, err := tx.Query(`SELECT DISTINCT user_id, contrast FROM hvpt_trial ORDER BY user_id, contrast`)
	if err != nil {
		return 0, fmt.Errorf("list trained contrasts: %w", err)
	}
	var keys []key
	for rows.Next() {
		var k key
		if err := rows.Scan(&k.userID, &k.contrast); err != nil {
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
		replayed, err := replayContrast(tx, sched, gateWindow, k.userID, k.contrast)
		if err != nil {
			return changed, err
		}
		stored, found, err := contrastStateTx(tx, k.userID, k.contrast)
		if err != nil {
			return changed, err
		}
		if found && sameContrastState(stored, replayed) {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO hvpt_contrast_state (user_id, contrast, due, stability, difficulty,
			    scheduled_days, reps, lapses, state, last_review, recent_accuracy, recent_total,
			    remaining_steps)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(user_id, contrast) DO UPDATE SET
			  due=excluded.due, stability=excluded.stability, difficulty=excluded.difficulty,
			  scheduled_days=excluded.scheduled_days, reps=excluded.reps,
			  lapses=excluded.lapses, state=excluded.state, last_review=excluded.last_review,
			  recent_accuracy=excluded.recent_accuracy, recent_total=excluded.recent_total,
			  remaining_steps=excluded.remaining_steps`,
			k.userID, k.contrast, db.FmtTime(replayed.State.Due), replayed.State.Stability,
			replayed.State.Difficulty, replayed.State.ScheduledDays, replayed.State.Reps,
			replayed.State.Lapses, replayed.State.State, db.FmtTime(replayed.State.LastReview),
			replayed.Accuracy, replayed.Total, replayed.State.RemainingSteps); err != nil {
			return changed, fmt.Errorf("save contrast %s: %w", k.contrast, err)
		}
		changed++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit hvpt steps reconciliation: %w", err)
	}
	return changed, nil
}

func replayContrast(tx *sql.Tx, sched *srs.Scheduler, gateWindow int, userID int64, contrast string) (ContrastState, error) {
	rows, err := tx.Query(`
		SELECT correct, created_at FROM hvpt_trial
		 WHERE user_id = ? AND contrast = ? ORDER BY id`, userID, contrast)
	if err != nil {
		return ContrastState{}, fmt.Errorf("read trials for %s: %w", contrast, err)
	}
	defer rows.Close()

	cs := ContrastState{Contrast: contrast}
	first := true
	for rows.Next() {
		var correct int
		var at string
		if err := rows.Scan(&correct, &at); err != nil {
			return ContrastState{}, err
		}
		now := db.ParseTime(sql.NullString{String: at, Valid: true})
		if first {
			cs.State = srs.NewCardState(now)
			first = false
		}
		rating := srs.Again
		if correct == 1 {
			rating = srs.Good
		}
		next, _, err := sched.Review(cs.State, rating, now)
		if err != nil {
			return ContrastState{}, fmt.Errorf("replay contrast %s: %w", contrast, err)
		}
		cs.State = next
		cs.Accuracy, cs.Total = srs.RollingAccuracy(cs.Accuracy, cs.Total, gateWindow, correct == 1)
	}
	return cs, rows.Err()
}

func contrastStateTx(tx *sql.Tx, userID int64, contrast string) (ContrastState, bool, error) {
	var cs ContrastState
	var due, last sql.NullString
	err := tx.QueryRow(`
		SELECT contrast, due, stability, difficulty, scheduled_days, reps, lapses,
		       state, last_review, recent_accuracy, recent_total, remaining_steps
		  FROM hvpt_contrast_state WHERE user_id = ? AND contrast = ?`, userID, contrast,
	).Scan(&cs.Contrast, &due, &cs.State.Stability, &cs.State.Difficulty,
		&cs.State.ScheduledDays, &cs.State.Reps, &cs.State.Lapses, &cs.State.State,
		&last, &cs.Accuracy, &cs.Total, &cs.State.RemainingSteps)
	if err == sql.ErrNoRows {
		return ContrastState{}, false, nil
	}
	if err != nil {
		return ContrastState{}, false, fmt.Errorf("load contrast %s: %w", contrast, err)
	}
	cs.State.Due = db.ParseTime(due)
	cs.State.LastReview = db.ParseTime(last)
	return cs, true, nil
}

func sameContrastState(a, b ContrastState) bool {
	return db.FmtTime(a.State.Due) == db.FmtTime(b.State.Due) &&
		a.State.Stability == b.State.Stability &&
		a.State.Difficulty == b.State.Difficulty &&
		a.State.ScheduledDays == b.State.ScheduledDays &&
		a.State.Reps == b.State.Reps &&
		a.State.Lapses == b.State.Lapses &&
		a.State.State == b.State.State &&
		db.FmtTime(a.State.LastReview) == db.FmtTime(b.State.LastReview) &&
		a.State.RemainingSteps == b.State.RemainingSteps &&
		a.Accuracy == b.Accuracy && a.Total == b.Total
}
