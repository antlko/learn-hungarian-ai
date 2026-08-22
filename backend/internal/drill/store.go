package drill

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"tanulom/internal/curriculum"
	"tanulom/internal/db"
)

// Persistence for the curriculum gate and the drill log.
//
// Mirrors internal/srs/store.go, including the querier indirection: the SQLite
// pool is capped at one connection, so any read issued against the pool while a
// transaction holds it deadlocks instantly. Every method that can run inside a
// transaction takes it.

// Store persists topic progress and drill attempts.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// DB exposes the handle for callers that need to open the shared transaction.
func (s *Store) DB() *sql.DB { return s.db }

type querier interface {
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
	Exec(query string, args ...any) (sql.Result, error)
}

func (s *Store) q(tx *sql.Tx) querier {
	if tx != nil {
		return tx
	}
	return s.db
}

// Progress loads one topic's progress, returning a fresh locked one if absent.
//
// Absent means "never touched", and a never-touched topic is locked rather than
// blocked: whether it may be opened is the DAG's call, not the store's.
func (s *Store) Progress(tx *sql.Tx, userID int64, code string) (Progress, error) {
	p := Progress{TopicCode: code, Phase: Locked}
	var since sql.NullString
	err := s.q(tx).QueryRow(`
		SELECT phase, blocked_accuracy, blocked_total, interleaved_accuracy,
		       interleaved_total, median_ms, attempts, correct, fallbacks, phase_since
		  FROM topic_progress WHERE user_id = ? AND topic_code = ?`, userID, code,
	).Scan(&p.Phase, &p.BlockedAccuracy, &p.BlockedTotal, &p.InterleavedAccuracy,
		&p.InterleavedTotal, &p.MedianMs, &p.Attempts, &p.Correct, &p.Fallbacks, &since)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return p, fmt.Errorf("load topic progress: %w", err)
	}
	p.PhaseSince = db.ParseTime(since)
	return p, nil
}

// AllProgress loads every topic the learner has touched.
func (s *Store) AllProgress(userID int64) (map[string]Progress, error) {
	rows, err := s.db.Query(`
		SELECT topic_code, phase, blocked_accuracy, blocked_total,
		       interleaved_accuracy, interleaved_total, median_ms,
		       attempts, correct, fallbacks, phase_since
		  FROM topic_progress WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("load curriculum progress: %w", err)
	}
	defer rows.Close()

	out := map[string]Progress{}
	for rows.Next() {
		var p Progress
		var since sql.NullString
		if err := rows.Scan(&p.TopicCode, &p.Phase, &p.BlockedAccuracy, &p.BlockedTotal,
			&p.InterleavedAccuracy, &p.InterleavedTotal, &p.MedianMs,
			&p.Attempts, &p.Correct, &p.Fallbacks, &since); err != nil {
			return nil, err
		}
		p.PhaseSince = db.ParseTime(since)
		out[p.TopicCode] = p
	}
	return out, rows.Err()
}

// SaveProgress upserts a topic's progress.
func (s *Store) SaveProgress(tx *sql.Tx, userID int64, p Progress) error {
	since := db.FmtTime(p.PhaseSince)
	if since == "" {
		since = db.FmtTime(time.Now())
	}
	_, err := s.q(tx).Exec(`
		INSERT INTO topic_progress (user_id, topic_code, phase, blocked_accuracy,
		    blocked_total, interleaved_accuracy, interleaved_total, median_ms,
		    attempts, correct, fallbacks, phase_since)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(user_id, topic_code) DO UPDATE SET
		  phase=excluded.phase,
		  blocked_accuracy=excluded.blocked_accuracy,
		  blocked_total=excluded.blocked_total,
		  interleaved_accuracy=excluded.interleaved_accuracy,
		  interleaved_total=excluded.interleaved_total,
		  median_ms=excluded.median_ms,
		  attempts=excluded.attempts,
		  correct=excluded.correct,
		  fallbacks=excluded.fallbacks,
		  phase_since=excluded.phase_since`,
		userID, p.TopicCode, string(p.Phase), p.BlockedAccuracy, p.BlockedTotal,
		p.InterleavedAccuracy, p.InterleavedTotal, p.MedianMs,
		p.Attempts, p.Correct, p.Fallbacks, since)
	if err != nil {
		return fmt.Errorf("save topic progress: %w", err)
	}
	return nil
}

// Attempt is one answered drill item, as stored.
type Attempt struct {
	TopicCode  string
	Kind       Kind
	Phase      Phase
	Prompt     string
	Expected   string
	Answer     string
	Correct    bool
	DurationMs int64
	FeatureIDs []string
	ErrorClass string
	Origin     string
}

// AppendAttempt writes one drill_attempt row.
func (s *Store) AppendAttempt(tx *sql.Tx, userID int64, a Attempt) error {
	correct := 0
	if a.Correct {
		correct = 1
	}
	origin := a.Origin
	if origin == "" {
		origin = "drill"
	}
	_, err := s.q(tx).Exec(`
		INSERT INTO drill_attempt (user_id, topic_code, kind, phase, prompt, expected,
		                           answer, correct, duration_ms, feature_ids,
		                           error_class, origin)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		userID, a.TopicCode, string(a.Kind), string(a.Phase), a.Prompt, a.Expected,
		a.Answer, correct, a.DurationMs, strings.Join(a.FeatureIDs, "|"),
		a.ErrorClass, origin)
	if err != nil {
		return fmt.Errorf("append drill attempt: %w", err)
	}
	return nil
}

// TopicCounts is the honest lifetime tally shown next to a topic.
type TopicCounts struct {
	Attempts int `json:"attempts"`
	Correct  int `json:"correct"`
}

// RecentErrors returns the features the learner has recently got wrong, most
// frequent first.
//
// It reads BOTH state spaces' error records — feature reviews from the review
// loop and drill attempts from here — because an inessive missed while reading
// and an inessive missed in a drill are the same weakness, and treating them
// separately would mean the reader never influences what gets drilled.
//
// `origin` on feature_review is deliberately not filtered: a wrong answer is
// evidence wherever it came from.
func (s *Store) RecentErrors(userID int64, since time.Time, limit int) ([]FeatureError, error) {
	if limit <= 0 {
		limit = 12
	}
	rows, err := s.db.Query(`
		SELECT feature_id, COUNT(*) AS misses, MAX(review_time) AS last_seen
		  FROM feature_review
		 WHERE user_id = ? AND correct = 0 AND review_time >= ?
		 GROUP BY feature_id
		 ORDER BY misses DESC, last_seen DESC
		 LIMIT ?`, userID, db.FmtTime(since), limit)
	if err != nil {
		return nil, fmt.Errorf("load recent feature errors: %w", err)
	}
	defer rows.Close()

	var out []FeatureError
	for rows.Next() {
		var fe FeatureError
		var last sql.NullString
		if err := rows.Scan(&fe.FeatureID, &fe.Misses, &last); err != nil {
			return nil, err
		}
		fe.LastSeen = db.ParseTime(last)
		out = append(out, fe)
	}
	return out, rows.Err()
}

// RecentErrorClasses counts drill error classes in a window, so the UI can say
// "most of your misses last week were harmony, not case".
func (s *Store) RecentErrorClasses(userID int64, since time.Time) (map[string]int, error) {
	rows, err := s.db.Query(`
		SELECT error_class, COUNT(*) FROM drill_attempt
		 WHERE user_id = ? AND correct = 0 AND error_class != '' AND created_at >= ?
		 GROUP BY error_class`, userID, db.FmtTime(since))
	if err != nil {
		return nil, fmt.Errorf("load recent error classes: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var class string
		var n int
		if err := rows.Scan(&class, &n); err != nil {
			return nil, err
		}
		out[class] = n
	}
	return out, rows.Err()
}

// AnsweredToday counts drill items answered since a cutoff, for the Today view.
func (s *Store) AnsweredSince(userID int64, since time.Time) (int, error) {
	var n int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM drill_attempt WHERE user_id = ? AND created_at >= ?`,
		userID, db.FmtTime(since)).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count drill attempts: %w", err)
	}
	return n, nil
}

// ── placement ─────────────────────────────────────────────────────────────

// PlacementResults reads a running or finished placement test out of the
// attempt log.
//
// The log IS the state. A separate placement session table would be another
// thing to keep in step with the attempts it summarises, and the attempts are
// already written for every answer — including placement ones, tagged by origin.
func (s *Store) PlacementResults(userID int64) (map[string]curriculum.TopicResult, error) {
	rows, err := s.db.Query(`
		SELECT topic_code, COUNT(*), SUM(correct)
		  FROM drill_attempt
		 WHERE user_id = ? AND origin = 'placement'
		 GROUP BY topic_code`, userID)
	if err != nil {
		return nil, fmt.Errorf("load placement results: %w", err)
	}
	defer rows.Close()

	out := map[string]curriculum.TopicResult{}
	for rows.Next() {
		var r curriculum.TopicResult
		if err := rows.Scan(&r.Code, &r.Answered, &r.Correct); err != nil {
			return nil, err
		}
		out[r.Code] = r
	}
	return out, rows.Err()
}

// ClearPlacement discards a placement test so it can be retaken.
//
// The drill attempts go; the feature reviews they produced do NOT. Those were
// real retrievals and deleting them would rewrite history in the one state space
// that has to stay auditable.
func (s *Store) ClearPlacement(userID int64) error {
	_, err := s.db.Exec(
		`DELETE FROM drill_attempt WHERE user_id = ? AND origin = 'placement'`, userID)
	if err != nil {
		return fmt.Errorf("clear placement: %w", err)
	}
	return nil
}

// PlacementDone reports whether a learner has ever answered a placement item, so
// the UI can offer the test once rather than nagging.
func (s *Store) PlacementAnswered(userID int64) (int, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM drill_attempt WHERE user_id = ? AND origin = 'placement'`,
		userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count placement answers: %w", err)
	}
	return n, nil
}

// ApplySeeds writes the initial topic state a placement test decided.
//
// Existing progress wins on every counter: a learner who retakes placement after
// two months of real practice must not have that practice overwritten by three
// probe items. Only a topic with no measured history is seeded.
func (s *Store) ApplySeeds(userID int64, seeds []curriculum.Seed, cfg GateConfig, now time.Time) (int, error) {
	cfg = cfg.normalized()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after a successful commit

	applied := 0
	for _, seed := range seeds {
		cur, err := s.Progress(tx, userID, seed.Code)
		if err != nil {
			return 0, err
		}
		if cur.Attempts > 0 && cur.BlockedTotal >= cfg.Window {
			continue // real history already exists; leave it alone
		}
		phase := Phase(seed.Phase)
		if !phase.Valid() {
			phase = Blocked
		}
		cur.TopicCode = seed.Code
		cur.Phase = phase
		cur.BlockedAccuracy = seed.Accuracy
		cur.BlockedTotal = seed.Observations
		cur.PhaseSince = now
		if err := s.SaveProgress(tx, userID, cur); err != nil {
			return 0, err
		}
		applied++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit placement seeds: %w", err)
	}
	return applied, nil
}
