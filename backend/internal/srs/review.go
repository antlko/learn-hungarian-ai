package srs

import (
	"database/sql"
	"fmt"
	"time"

	"tanulom/internal/db"
)

// Engine ties the scheduler, the store and the two state spaces together.
type Engine struct {
	store      *Store
	scheduler  *Scheduler
	gateWindow int
}

// NewEngine builds an engine at a given desired retention.
func NewEngine(store *Store, desiredRetention float64, gateWindow int) *Engine {
	if gateWindow < 2 {
		gateWindow = 20
	}
	return &Engine{store: store, scheduler: NewScheduler(desiredRetention), gateWindow: gateWindow}
}

func (e *Engine) Scheduler() *Scheduler { return e.scheduler }
func (e *Engine) Store() *Store         { return e.store }

// AnswerResult is what one answer produced.
type AnswerResult struct {
	Card           Card           `json:"-"`
	Rating         Rating         `json:"rating"`
	NextDue        time.Time      `json:"nextDue"`
	IntervalDays   uint64         `json:"intervalDays"`
	GradedFeatures []FeatureGrade `json:"gradedFeatures"`
}

// Answer applies one answer, updating BOTH state spaces atomically.
//
// The whole update is one transaction. A card whose schedule advanced but whose
// features did not — or vice versa — would leave the two spaces disagreeing about
// what the learner has practised, and nothing downstream could detect it.
func (e *Engine) Answer(
	userID int64,
	card Card,
	note Note,
	outcome Outcome,
	wrongFeature string,
	now time.Time,
) (AnswerResult, error) {
	rating := DeriveRating(outcome)

	nextState, log, err := e.scheduler.Review(card.State, rating, now)
	if err != nil {
		return AnswerResult{}, err
	}
	log.DurationMs = outcome.Duration.Milliseconds()
	log.Correct = outcome.Correct

	// Only cloze grades features: blanking exactly one span is what makes it
	// sound to say which suffix was wrong. See Direction.GradesFeatures.
	var grades []FeatureGrade
	if card.Direction.GradesFeatures() {
		grades = RequiredFeatures(note.TargetFeats, outcome, wrongFeature)
	}

	tx, err := e.store.db.Begin()
	if err != nil {
		return AnswerResult{}, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after a successful commit

	card.State = nextState
	if err := e.store.SaveCard(tx, card); err != nil {
		return AnswerResult{}, err
	}
	if err := e.store.AppendLog(tx, card.ID, log); err != nil {
		return AnswerResult{}, err
	}
	if err := e.applyFeatureGrades(tx, userID, grades, card.ID, outcome, now); err != nil {
		return AnswerResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return AnswerResult{}, fmt.Errorf("commit answer: %w", err)
	}

	return AnswerResult{
		Card: card, Rating: rating,
		NextDue: nextState.Due, IntervalDays: nextState.ScheduledDays,
		GradedFeatures: grades,
	}, nil
}

// applyFeatureGrades advances the morphological state space from a card answer.
func (e *Engine) applyFeatureGrades(
	tx *sql.Tx, userID int64, grades []FeatureGrade,
	cardID int64, outcome Outcome, now time.Time,
) error {
	return e.GradeFeatures(tx, userID, grades, cardID, outcome, "review", now)
}

// GradeFeatures advances the morphological state space for grades that did not
// come from a card.
//
// A grammar drill has no note and no card, but it is exactly as much a retrieval
// attempt as a cloze review is, and the second state space is where that
// evidence belongs — otherwise the mastery heatmap would silently ignore every
// drill the learner ever answered. `origin` records where the evidence came
// from, so "your inessive is weak" can always be traced back to the items that
// said so. Pass cardID 0 when there is no card.
func (e *Engine) GradeFeatures(
	tx *sql.Tx, userID int64, grades []FeatureGrade,
	cardID int64, outcome Outcome, origin string, now time.Time,
) error {
	durationMs := outcome.Duration.Milliseconds()

	for _, g := range grades {
		fs, err := e.store.FeatureState(tx, userID, g.FeatureID, now)
		if err != nil {
			return err
		}
		next, _, err := e.scheduler.Review(fs.State, g.Rating, now)
		if err != nil {
			return err
		}
		fs.State = next
		fs.Accuracy, fs.Total = RollingAccuracy(fs.Accuracy, fs.Total, e.gateWindow, g.Correct)
		if g.Correct {
			// Latency only means something on a correct answer: a wrong answer
			// given quickly is not evidence of automaticity.
			fs.MedianMs = UpdateMedian(fs.MedianMs, durationMs)
		}

		if err := e.store.SaveFeatureState(tx, userID, fs); err != nil {
			return err
		}
		if err := e.store.AppendFeatureReview(tx, userID, g, cardID, durationMs, now, origin); err != nil {
			return err
		}
	}
	return nil
}

// FeatureMastery summarises one feature for the heatmap.
type FeatureMastery struct {
	FeatureID      string  `json:"featureId"`
	Retrievability float64 `json:"retrievability"`
	Accuracy       float64 `json:"accuracy"`
	Reps           uint64  `json:"reps"`
	Observations   int     `json:"observations"`
	MedianMs       int64   `json:"medianMs"`
	// Automated matters for motor skills such as vowel harmony, where being
	// correct-but-slow means the skill is not yet automatic. See [R15].
	Automated bool `json:"automated"`
}

// Mastery returns every feature the learner has met, for the progress heatmap.
func (e *Engine) Mastery(userID int64, harmonyLatencyMs int, now time.Time) ([]FeatureMastery, error) {
	// `state` is not decoration: FSRS reports zero retrievability for a card it
	// believes is New, so omitting the column made every feature read 0.00 no
	// matter how much stability it had accumulated.
	rows, err := e.store.db.Query(`
		SELECT feature_id, stability, difficulty, state, scheduled_days,
		       last_review, reps, recent_accuracy, recent_total, median_ms
		  FROM morph_state WHERE user_id = ? ORDER BY feature_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("load mastery: %w", err)
	}
	defer rows.Close()

	var out []FeatureMastery
	for rows.Next() {
		var m FeatureMastery
		var st CardState
		var last sql.NullString
		if err := rows.Scan(&m.FeatureID, &st.Stability, &st.Difficulty, &st.State,
			&st.ScheduledDays, &last, &st.Reps,
			&m.Accuracy, &m.Observations, &m.MedianMs); err != nil {
			return nil, err
		}
		st.LastReview = db.ParseTime(last)
		m.Reps = st.Reps
		m.Retrievability = e.scheduler.Retrievability(st, now)
		m.Automated = m.MedianMs > 0 && m.MedianMs <= int64(harmonyLatencyMs) &&
			m.Accuracy >= 0.9 && GateReady(m.Observations, e.gateWindow)
		out = append(out, m)
	}
	return out, rows.Err()
}
