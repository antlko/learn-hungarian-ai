package srs

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"tanulom/internal/db"
)

// Free practice: retrieval without scheduling.
//
// The review queue answers "what do I owe today". This answers a different
// question — "let me train" — and it has to be answerable at any moment, with
// an empty queue and a deck of nothing but new cards. So it ignores due dates,
// ignores the daily new-card budget, and draws at random.
//
// A practice answer IS a review [R20] (ADR-064, superseding ADR-019): the API
// handler feeds it to Engine.Answer, and FSRS discounts an early answer by the
// real elapsed time, so there is no scheduling maths of ours. Two cases stay
// practice_log-only — words in «Знаю» and NEW cards (which would bypass the
// daily new budget). LogPractice below is the tally for all of them.

// PracticeQueue draws a random sample of cards from one scope.
//
// Random and unweighted, on purpose: this is the mode a learner opens to train
// rather than to be managed, and a hidden agenda about which words come up
// would make it something else. Siblings are buried WITHIN the sample for the
// usual reason — two directions of one word together turns the second into a
// copying exercise — but never across the day: practice is always available,
// and burying it against review answers would be the app rationing the one
// thing it does not schedule.
func (s *Store) PracticeQueue(userID int64, scope Scope, limit int) ([]QueueItem, error) {
	if limit <= 0 {
		limit = 20
	}
	// Over-select so that burying siblings does not leave the session short of
	// what was asked for.
	// Seen cards first, still in random order among themselves; new cards only
	// fill what is left. A practice answer on a new card is not counted
	// (ADR-064: it would bypass the daily new budget), so a round dealt
	// blindly from a deck that is mostly new would be a round that moves
	// nothing — the opposite of what practice is for. This is which cards are
	// dealt, not when a card is due: no scheduling.
	items, err := s.selectQueue(userID, scope,
		` ORDER BY (c.state = 0), RANDOM() LIMIT ?`, limit*4)
	if err != nil {
		return nil, err
	}
	items = burySiblingCards(items)
	if len(items) > limit {
		items = items[:limit]
	}
	// RANDOM() can still deal three blanks of one sentence in a row, and a
	// round drawn from a single text is exactly where that happens.
	return spreadSentences(items), nil
}

// PracticeQueueWeak serves the learner's shakiest SEEN cards: lowest current
// retrievability first, weakest card per word, computed by the library
// (ADR-003 — no scheduling maths of ours).
//
// This does not contradict the "random and unweighted" rule above — that rule
// rejects a HIDDEN agenda. Weak mode is the learner's explicit choice with the
// criterion printed on screen, which is the opposite of being managed. It
// is only about which cards are dealt. ADR-024; answers count since ADR-064.
//
// Cards never reviewed have no retrievability to rank; a deck with nothing
// reviewed yet returns empty, and the handler falls back to random and SAYS so
// in the echoed mode.
func (s *Store) PracticeQueueWeak(userID int64, scope Scope, limit int, sched *Scheduler, now time.Time) ([]QueueItem, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT c.id, c.note_id, c.due, c.stability, c.difficulty, c.scheduled_days,
		       c.reps, c.lapses, c.state, c.last_review, c.remaining_steps
		  FROM card c JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND c.suspended = 0 AND c.state != 0`+scope.filter()+`
		 LIMIT 5000`, userID)
	if err != nil {
		return nil, fmt.Errorf("load seen cards: %w", err)
	}
	defer rows.Close()

	type weak struct {
		cardID, noteID int64
		r              float64
	}
	var seen []weak
	for rows.Next() {
		var w weak
		var st CardState
		var due string
		var last sql.NullString
		if err := rows.Scan(&w.cardID, &w.noteID, &due, &st.Stability, &st.Difficulty,
			&st.ScheduledDays, &st.Reps, &st.Lapses, &st.State, &last,
			&st.RemainingSteps); err != nil {
			return nil, err
		}
		st.Due = db.ParseTime(sql.NullString{String: due, Valid: true})
		st.LastReview = db.ParseTime(last)
		w.r = sched.Retrievability(st, now)
		seen = append(seen, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(seen) == 0 {
		return nil, nil
	}

	sort.Slice(seen, func(i, j int) bool { return seen[i].r < seen[j].r })
	order := map[int64]int{}
	usedNote := map[int64]bool{}
	ids := make([]string, 0, limit)
	args := make([]any, 0, limit+1)
	args = append(args, userID)
	for _, w := range seen {
		if usedNote[w.noteID] {
			continue
		}
		usedNote[w.noteID] = true
		order[w.cardID] = len(order)
		ids = append(ids, "?")
		args = append(args, w.cardID)
		if len(order) == limit {
			break
		}
	}

	// `scope`, never ScopeAll: the ids came from this scope, and ScopeAll
	// carries the not-trashed clause — a weak round over the trash would rank
	// its cards and then re-select none of them.
	items, err := s.selectQueue(userID, scope,
		` AND c.id IN (`+strings.Join(ids, ",")+`)`, args[1:]...)
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return order[items[i].CardID] < order[items[j].CardID] })
	return items, nil
}

// PracticeCount is how many cards a scope could offer at all, so the screen can
// say "there is nothing here yet" rather than opening an empty session.
func (s *Store) PracticeCount(userID int64, scope Scope) (int, error) {
	var n int
	err := s.db.QueryRow(`
		SELECT COUNT(DISTINCT c.note_id)
		  FROM card c JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND c.suspended = 0`+scope.filter(), userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count practice pool: %w", err)
	}
	return n, nil
}

// LogPractice records one practice answer.
//
// This is the practice tally only. Whether the answer also became a review is
// the caller's decision (ADR-064); for a counted one the review_log row carries
// the SAME instant, which is how UndoLast finds this row to delete.
func (s *Store) LogPractice(userID, cardID int64, direction Direction, correct bool, durationMs int64, now time.Time) error {
	c := 0
	if correct {
		c = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO practice_log (user_id, card_id, direction, correct, duration_ms, practised_at)
		VALUES (?,?,?,?,?,?)`,
		userID, cardID, string(direction), c, durationMs, db.FmtTime(now))
	if err != nil {
		return fmt.Errorf("log practice: %w", err)
	}
	return nil
}

// PracticeStats is how much practice happened and how it went.
type PracticeStats struct {
	Today      int `json:"today"`
	TodayRight int `json:"todayRight"`
	Total      int `json:"total"`
	TotalRight int `json:"totalRight"`
}

// PracticeDay is one local day's practice volume and accuracy.
type PracticeDay struct {
	Day   string `json:"day"`
	Total int    `json:"total"`
	Right int    `json:"right"`
}

// PracticeDays buckets the last `days` days of practice by LOCAL day.
//
// Bucketed in Go rather than by substr(practised_at, 1, 10): the stored
// strings are UTC (db.FmtTime), so the SQL shortcut files every evening
// session under the wrong date for any learner east of Greenwich. Row counts
// are small; correctness wins.
func (s *Store) PracticeDays(userID int64, days int, now time.Time) ([]PracticeDay, error) {
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	since := midnight.AddDate(0, 0, -(days - 1))
	rows, err := s.db.Query(`
		SELECT practised_at, correct FROM practice_log
		 WHERE user_id = ? AND practised_at >= ? ORDER BY practised_at`,
		userID, db.FmtTime(since))
	if err != nil {
		return nil, fmt.Errorf("load practice days: %w", err)
	}
	defer rows.Close()

	byDay := map[string]*PracticeDay{}
	var order []string
	for rows.Next() {
		var at string
		var correct int
		if err := rows.Scan(&at, &correct); err != nil {
			return nil, err
		}
		day := db.ParseTime(sql.NullString{String: at, Valid: true}).
			In(now.Location()).Format("2006-01-02")
		d, ok := byDay[day]
		if !ok {
			d = &PracticeDay{Day: day}
			byDay[day] = d
			order = append(order, day)
		}
		d.Total++
		if correct == 1 {
			d.Right++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]PracticeDay, 0, len(order))
	for _, day := range order {
		out = append(out, *byDay[day])
	}
	return out, nil
}

// PracticeDirection is one direction's all-time practice tally.
type PracticeDirection struct {
	Direction Direction `json:"direction"`
	Total     int       `json:"total"`
	Right     int       `json:"right"`
}

// PracticeByDirection splits practice accuracy by card direction — where the
// Progress screen can show that recognising works and producing does not.
func (s *Store) PracticeByDirection(userID int64) ([]PracticeDirection, error) {
	rows, err := s.db.Query(`
		SELECT direction, COUNT(*), SUM(correct)
		  FROM practice_log WHERE user_id = ?
		 GROUP BY direction ORDER BY COUNT(*) DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("split practice by direction: %w", err)
	}
	defer rows.Close()
	var out []PracticeDirection
	for rows.Next() {
		var d PracticeDirection
		if err := rows.Scan(&d.Direction, &d.Total, &nullInt{&d.Right}); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Practice reports the learner's practice volume and accuracy.
//
// Volume AND accuracy, never volume alone: a count of items answered is a
// number that only ever goes up, which is the shape of a point economy rather
// than of a competence signal [R14]. How often it was right is the part with
// something measured behind it.
func (s *Store) Practice(userID int64, now time.Time) (PracticeStats, error) {
	var p PracticeStats
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	err := s.db.QueryRow(`
		SELECT
		  SUM(CASE WHEN practised_at >= ? THEN 1 ELSE 0 END),
		  SUM(CASE WHEN practised_at >= ? AND correct = 1 THEN 1 ELSE 0 END),
		  COUNT(*),
		  SUM(correct)
		  FROM practice_log WHERE user_id = ?`,
		db.FmtTime(midnight), db.FmtTime(midnight), userID,
	).Scan(&nullInt{&p.Today}, &nullInt{&p.TodayRight}, &nullInt{&p.Total}, &nullInt{&p.TotalRight})
	if err != nil {
		return p, fmt.Errorf("read practice stats: %w", err)
	}
	return p, nil
}
