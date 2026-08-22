package hvpt

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"tanulom/internal/db"
	"tanulom/internal/srs"
)

// Persistence for contrast state and the trial log.

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// States loads every contrast the learner has met.
func (s *Store) States(userID int64) (map[string]ContrastState, error) {
	rows, err := s.db.Query(`
		SELECT contrast, due, stability, difficulty, scheduled_days, reps, lapses,
		       state, last_review, recent_accuracy, recent_total
		  FROM hvpt_contrast_state WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("load contrast states: %w", err)
	}
	defer rows.Close()
	out := map[string]ContrastState{}
	for rows.Next() {
		var cs ContrastState
		var due, last sql.NullString
		if err := rows.Scan(&cs.Contrast, &due, &cs.State.Stability, &cs.State.Difficulty,
			&cs.State.ScheduledDays, &cs.State.Reps, &cs.State.Lapses, &cs.State.State,
			&last, &cs.Accuracy, &cs.Total); err != nil {
			return nil, err
		}
		cs.State.Due = db.ParseTime(due)
		cs.State.LastReview = db.ParseTime(last)
		out[cs.Contrast] = cs
	}
	return out, rows.Err()
}

// Answer applies one trial result: FSRS + accuracy on the contrast, plus the
// trial log row, in one transaction.
func (s *Store) Answer(userID int64, t Trial, chosen string, rtMs int64,
	sched *srs.Scheduler, gateWindow int, now time.Time) (ContrastState, bool, error) {

	correct := chosen == t.Played()

	states, err := s.States(userID)
	if err != nil {
		return ContrastState{}, false, err
	}
	cs, seen := states[t.Pair.Contrast]
	if !seen {
		cs = ContrastState{Contrast: t.Pair.Contrast, State: srs.NewCardState(now)}
	}

	// 2AFC has a 50% guessing floor, so a single miss is weak evidence and the
	// FSRS rating reflects the OUTCOME only coarsely: wrong → Again, right →
	// Good. Latency-derived Easy/Hard would double-count what the rolling
	// accuracy already captures at the contrast level.
	rating := srs.Again
	if correct {
		rating = srs.Good
	}
	next, _, err := sched.Review(cs.State, rating, now)
	if err != nil {
		return ContrastState{}, false, err
	}
	cs.State = next
	cs.Accuracy, cs.Total = srs.RollingAccuracy(cs.Accuracy, cs.Total, gateWindow, correct)

	tx, err := s.db.Begin()
	if err != nil {
		return ContrastState{}, false, err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`
		INSERT INTO hvpt_contrast_state (user_id, contrast, due, stability, difficulty,
		    scheduled_days, reps, lapses, state, last_review, recent_accuracy, recent_total)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(user_id, contrast) DO UPDATE SET
		  due=excluded.due, stability=excluded.stability, difficulty=excluded.difficulty,
		  scheduled_days=excluded.scheduled_days, reps=excluded.reps,
		  lapses=excluded.lapses, state=excluded.state, last_review=excluded.last_review,
		  recent_accuracy=excluded.recent_accuracy, recent_total=excluded.recent_total`,
		userID, cs.Contrast, db.FmtTime(cs.State.Due), cs.State.Stability, cs.State.Difficulty,
		cs.State.ScheduledDays, cs.State.Reps, cs.State.Lapses, cs.State.State,
		db.FmtTime(cs.State.LastReview), cs.Accuracy, cs.Total); err != nil {
		return ContrastState{}, false, fmt.Errorf("save contrast state: %w", err)
	}
	c := 0
	if correct {
		c = 1
	}
	if _, err := tx.Exec(`
		INSERT INTO hvpt_trial (user_id, pair_id, contrast, played, chosen, correct,
		                        talker_id, rate, rt_ms)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		userID, t.Pair.ID, t.Pair.Contrast, t.Played(), chosen, c,
		t.TalkerID, t.Rate, rtMs); err != nil {
		return ContrastState{}, false, fmt.Errorf("log trial: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return ContrastState{}, false, err
	}
	return cs, correct, nil
}

// SessionStats summarises today's training, for the running header.
type SessionStats struct {
	Trials  int `json:"trials"`
	Correct int `json:"correct"`
	Talkers int `json:"talkers"`
}

// TodayStats counts trials since a cutoff.
func (s *Store) TodayStats(userID int64, since time.Time) (SessionStats, error) {
	var st SessionStats
	err := s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(correct),0), COUNT(DISTINCT talker_id)
		  FROM hvpt_trial WHERE user_id = ? AND created_at >= ?`,
		userID, db.FmtTime(since)).Scan(&st.Trials, &st.Correct, &st.Talkers)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return st, err
	}
	return st, nil
}
