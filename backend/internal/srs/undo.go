package srs

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"tanulom/internal/db"
)

// Undo: revert the learner's last answer, in BOTH state spaces.
//
// The mechanism is delete-and-replay, not restore-from-snapshot: the review log
// is the source of truth and state is a cache of it (see replay.go). Deleting
// the last log row and replaying the remainder cannot leave the two out of
// step, which a stored-previous-state scheme can — and the property test that
// guarantees replay fidelity is the same one that guarantees undo fidelity.
//
// Scope: the last CARD answer. Its feature reviews — written in the same
// transaction, identified by (card_id, review_time) — are reverted with it.
// Drill answers are not undoable: their gate movement is announced on screen at
// answer time, and unwinding topic phase transitions would mean un-ringing a
// bell the learner already saw.

// ErrNothingToUndo is returned when the user has no reviews at all.
var ErrNothingToUndo = errors.New("nothing to undo")

// UndoResult reports what was reverted.
type UndoResult struct {
	CardID     int64     `json:"cardId"`
	NoteID     int64     `json:"noteId"`
	ReviewTime time.Time `json:"reviewTime"`
	// Features are the ids whose state was recomputed.
	Features []string `json:"features"`
}

// UndoLast reverts the most recent card answer for a user.
func (e *Engine) UndoLast(userID int64, gateWindow int, now time.Time) (UndoResult, error) {
	var (
		logID, cardID, noteID int64
		reviewTime            string
	)
	err := e.store.db.QueryRow(`
		SELECT r.id, r.card_id, n.id, r.review_time
		  FROM review_log r
		  JOIN card c ON c.id = r.card_id
		  JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ?
		 ORDER BY r.id DESC LIMIT 1`, userID).Scan(&logID, &cardID, &noteID, &reviewTime)
	if errors.Is(err, sql.ErrNoRows) {
		return UndoResult{}, ErrNothingToUndo
	}
	if err != nil {
		return UndoResult{}, fmt.Errorf("find last review: %w", err)
	}

	tx, err := e.store.db.Begin()
	if err != nil {
		return UndoResult{}, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after a successful commit

	// Which features did that same answer grade? Same card, same timestamp —
	// they were written in one transaction with the log row.
	frows, err := tx.Query(`
		SELECT DISTINCT feature_id FROM feature_review
		 WHERE user_id = ? AND card_id = ? AND review_time = ?`,
		userID, cardID, reviewTime)
	if err != nil {
		return UndoResult{}, err
	}
	var features []string
	for frows.Next() {
		var id string
		if err := frows.Scan(&id); err != nil {
			frows.Close()
			return UndoResult{}, err
		}
		features = append(features, id)
	}
	frows.Close()

	if _, err := tx.Exec(`DELETE FROM review_log WHERE id = ?`, logID); err != nil {
		return UndoResult{}, err
	}
	if _, err := tx.Exec(`
		DELETE FROM feature_review
		 WHERE user_id = ? AND card_id = ? AND review_time = ?`,
		userID, cardID, reviewTime); err != nil {
		return UndoResult{}, err
	}

	// Replay the card from what remains.
	steps, err := cardSteps(tx, cardID)
	if err != nil {
		return UndoResult{}, err
	}
	state, err := e.scheduler.ReplayCard(steps)
	if err != nil {
		return UndoResult{}, err
	}
	if len(steps) == 0 {
		// Back to genuinely new: due now rather than at the zero time, so the
		// card re-enters today's queue instead of the epoch's.
		state = NewCardState(now)
	}
	if err := e.store.SaveCard(tx, Card{ID: cardID, State: state}); err != nil {
		return UndoResult{}, err
	}

	// Replay each touched feature from its full remaining history — including
	// drill answers, which also live in feature_review and must keep counting.
	for _, id := range features {
		fsteps, err := featureSteps(tx, userID, id)
		if err != nil {
			return UndoResult{}, err
		}
		fs, err := e.scheduler.ReplayFeature(id, fsteps, gateWindow)
		if err != nil {
			return UndoResult{}, err
		}
		if len(fsteps) == 0 {
			// No history left: remove the row rather than storing a zero state
			// that would render as "seen" in the mastery map.
			if _, err := tx.Exec(`
				DELETE FROM morph_state WHERE user_id = ? AND feature_id = ?`,
				userID, id); err != nil {
				return UndoResult{}, err
			}
			continue
		}
		if err := e.store.SaveFeatureState(tx, userID, fs); err != nil {
			return UndoResult{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return UndoResult{}, fmt.Errorf("commit undo: %w", err)
	}
	t := db.ParseTime(sql.NullString{String: reviewTime, Valid: true})
	return UndoResult{CardID: cardID, NoteID: noteID, ReviewTime: t, Features: features}, nil
}

func cardSteps(tx *sql.Tx, cardID int64) ([]ReplayStep, error) {
	rows, err := tx.Query(`
		SELECT rating, review_time, correct, duration_ms
		  FROM review_log WHERE card_id = ? ORDER BY id`, cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSteps(rows)
}

func featureSteps(tx *sql.Tx, userID int64, featureID string) ([]ReplayStep, error) {
	rows, err := tx.Query(`
		SELECT rating, review_time, correct, duration_ms
		  FROM feature_review WHERE user_id = ? AND feature_id = ? ORDER BY id`,
		userID, featureID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSteps(rows)
}

func scanSteps(rows *sql.Rows) ([]ReplayStep, error) {
	var out []ReplayStep
	for rows.Next() {
		var st ReplayStep
		var rating, correct int
		var when string
		if err := rows.Scan(&rating, &when, &correct, &st.DurationMs); err != nil {
			return nil, err
		}
		st.Rating = Rating(rating)
		st.Correct = correct == 1
		st.ReviewTime = db.ParseTime(sql.NullString{String: when, Valid: true})
		out = append(out, st)
	}
	return out, rows.Err()
}
