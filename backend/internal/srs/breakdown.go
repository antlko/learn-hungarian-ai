package srs

import (
	"database/sql"
	"fmt"
	"time"

	"tanulom/internal/db"
)

// The learner-facing review numbers, in ONE vocabulary. ADR-021.
//
// Three different counts used to describe "the queue" and none of them was the
// session: the tab strip counted raw due CARDS before burial, Today counted an
// unscoped unburied card total, and the served session was one card per word.
// A deck of 26 due cards over 10 words showed «26», served 10, said "done", and
// five learning steps later a reload produced more — every number true in its
// own coordinate system and the learner wrong to trust any of them.
//
// Breakdown is the single coordinate system. Everything in it is POST-burial
// and day-granular: DueNow is what the queue will actually serve, and the rest
// says where every hidden card went, instead of leaving them inside an
// unexplained bigger number.

// Breakdown is the state of one pile at an instant.
type Breakdown struct {
	Scope Scope `json:"scope"`
	// DueNow is the number of items a queue built now would serve, before any
	// maxReviewsPerDay trim: one card per word, siblings buried, graduated
	// (state 2) cards counted for the whole local day, learning steps only once
	// their minutes have elapsed. This is the number every screen calls
	// «к повторению».
	DueNow int `json:"dueNow"`
	// DueLaterToday counts learning/relearning steps that mature before local
	// midnight — the cards behind "5 more appeared after a refresh". NextDueAt
	// is the earliest of them, so the done screen can show a countdown.
	DueLaterToday int        `json:"dueLaterToday"`
	NextDueAt     *time.Time `json:"nextDueAt,omitempty"`
	// BuriedToday counts due cards hidden until tomorrow: other directions of a
	// word already answered today, plus extra directions of a word the session
	// will serve once. They are due, they will not be shown, and a count that
	// included them silently is how «26» happened.
	BuriedToday int `json:"buriedToday"`
	// NewCards / NewNotes are the unseen pool in this pile — cards and the words
	// they belong to. The daily budget is global (ADR-016) and lives on
	// QueueCounts, not here.
	NewCards int `json:"newCards"`
	NewNotes int `json:"newNotes"`
}

// dayWindow bounds "today": now, and the next local midnight.
func dayWindow(now time.Time) (nowStr, endStr string) {
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)
	return db.FmtTime(now), db.FmtTime(end)
}

// Breakdowns reports every fixed scope (all/course/reader, in Scopes order) in
// one pass, plus any extra scopes (a text scope) appended after them.
// `all` equals course + reader field by field: a note belongs to exactly one
// side and burial is a per-note fact, so the arithmetic the segmented control
// shows cannot stop holding.
func (s *Store) Breakdowns(userID int64, burySiblings bool, now time.Time, extra ...Scope) ([]Breakdown, error) {
	var answered map[int64]map[int64]bool
	var err error
	if burySiblings {
		if answered, err = s.answeredToday(userID, now); err != nil {
			return nil, err
		}
	}

	course, reader, err := s.fixedScopePair(userID, burySiblings, answered, now)
	if err != nil {
		return nil, err
	}
	all := sumBreakdowns(ScopeAll, course, reader)

	out := []Breakdown{all, course, reader}
	for _, sc := range extra {
		b, err := s.scopeBreakdown(userID, sc, burySiblings, answered, now)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

// ActionableDue is the streak's "queue cleared" number: what a queue built now
// would serve, across the whole deck. Written into daily_log.due_left after
// every answer, which is what makes the streak WINNABLE: the raw card count it
// replaced included siblings buried until tomorrow, so a learner whose words
// have more than one due card could clear everything the app would ever show
// and still be told the day's obligation stood. Learning steps maturing later
// today deliberately do not block the day — a 10-minute step at 23:55 is not a
// debt. ADR-021.
func (s *Store) ActionableDue(userID int64, burySiblings bool, now time.Time) (int, error) {
	bds, err := s.Breakdowns(userID, burySiblings, now)
	if err != nil {
		return 0, err
	}
	for _, b := range bds {
		if b.Scope == ScopeAll {
			return b.DueNow, nil
		}
	}
	return 0, nil
}

// dueRow is one non-new card inside today's window.
type dueRow struct {
	cardID, noteID int64
	state          int
	due            time.Time
}

// classify folds one scope's due rows into the Breakdown counts.
func classify(b *Breakdown, rows []dueRow, burySiblings bool, answered map[int64]map[int64]bool, now time.Time) {
	dueNotes := map[int64]int{}
	for _, r := range rows {
		if burySiblings {
			if cards, ok := answered[r.noteID]; ok && !cards[r.cardID] {
				b.BuriedToday++
				continue
			}
		}
		// Graduated cards count for the whole local day; learning steps only
		// once their minutes have elapsed.
		if r.state == 2 || !r.due.After(now) {
			dueNotes[r.noteID]++
			continue
		}
		b.DueLaterToday++
		if b.NextDueAt == nil || r.due.Before(*b.NextDueAt) {
			t := r.due
			b.NextDueAt = &t
		}
	}
	for _, n := range dueNotes {
		b.DueNow++
		if burySiblings && n > 1 {
			// Extra directions of a word the session will serve once: due, and
			// hidden until tomorrow by the answer that is about to happen.
			b.BuriedToday += n - 1
		} else if !burySiblings && n > 1 {
			// With burial off every direction is served, so they are all DueNow.
			b.DueNow += n - 1
		}
	}
}

// fixedScopePair computes course and reader in one pass over the deck.
func (s *Store) fixedScopePair(userID int64, burySiblings bool, answered map[int64]map[int64]bool, now time.Time) (Breakdown, Breakdown, error) {
	_, endStr := dayWindow(now)
	rows, err := s.db.Query(`
		SELECT c.id, c.note_id, c.state, c.due,
		       CASE WHEN n.source LIKE 'course:%' THEN 1 ELSE 0 END
		  FROM card c JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND c.suspended = 0 AND c.state != 0 AND c.due < ?`+
		NotTrashed+`
		 ORDER BY c.due ASC LIMIT 10000`, userID, endStr)
	if err != nil {
		return Breakdown{}, Breakdown{}, fmt.Errorf("load due window: %w", err)
	}
	defer rows.Close()

	var courseRows, readerRows []dueRow
	for rows.Next() {
		var r dueRow
		var due string
		var isCourse int
		if err := rows.Scan(&r.cardID, &r.noteID, &r.state, &due, &isCourse); err != nil {
			return Breakdown{}, Breakdown{}, err
		}
		r.due = db.ParseTime(sql.NullString{String: due, Valid: true})
		if isCourse == 1 {
			courseRows = append(courseRows, r)
		} else {
			readerRows = append(readerRows, r)
		}
	}
	if err := rows.Err(); err != nil {
		return Breakdown{}, Breakdown{}, err
	}

	course := Breakdown{Scope: ScopeCourse}
	reader := Breakdown{Scope: ScopeReader}
	classify(&course, courseRows, burySiblings, answered, now)
	classify(&reader, readerRows, burySiblings, answered, now)

	// The unseen pool, split the same way.
	nrows, err := s.db.Query(`
		SELECT CASE WHEN n.source LIKE 'course:%' THEN 1 ELSE 0 END,
		       COUNT(*), COUNT(DISTINCT c.note_id)
		  FROM card c JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND c.suspended = 0 AND c.state = 0`+NotTrashed+`
		 GROUP BY 1`, userID)
	if err != nil {
		return Breakdown{}, Breakdown{}, fmt.Errorf("count new pool: %w", err)
	}
	defer nrows.Close()
	for nrows.Next() {
		var isCourse, cards, notes int
		if err := nrows.Scan(&isCourse, &cards, &notes); err != nil {
			return Breakdown{}, Breakdown{}, err
		}
		if isCourse == 1 {
			course.NewCards, course.NewNotes = cards, notes
		} else {
			reader.NewCards, reader.NewNotes = cards, notes
		}
	}
	return course, reader, nrows.Err()
}

// scopeBreakdown computes one arbitrary scope (used for extra scopes such as a
// text) with the same classification as the fixed pair.
func (s *Store) scopeBreakdown(userID int64, scope Scope, burySiblings bool, answered map[int64]map[int64]bool, now time.Time) (Breakdown, error) {
	_, endStr := dayWindow(now)
	rows, err := s.db.Query(`
		SELECT c.id, c.note_id, c.state, c.due
		  FROM card c JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND c.suspended = 0 AND c.state != 0 AND c.due < ?`+scope.filter()+`
		 ORDER BY c.due ASC LIMIT 10000`, userID, endStr)
	if err != nil {
		return Breakdown{}, fmt.Errorf("load due window (%s): %w", scope, err)
	}
	defer rows.Close()

	var drs []dueRow
	for rows.Next() {
		var r dueRow
		var due string
		if err := rows.Scan(&r.cardID, &r.noteID, &r.state, &due); err != nil {
			return Breakdown{}, err
		}
		r.due = db.ParseTime(sql.NullString{String: due, Valid: true})
		drs = append(drs, r)
	}
	if err := rows.Err(); err != nil {
		return Breakdown{}, err
	}

	b := Breakdown{Scope: scope}
	classify(&b, drs, burySiblings, answered, now)

	err = s.db.QueryRow(`
		SELECT COUNT(*), COUNT(DISTINCT c.note_id)
		  FROM card c JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND c.suspended = 0 AND c.state = 0`+scope.filter(),
		userID).Scan(&b.NewCards, &b.NewNotes)
	if err != nil {
		return Breakdown{}, fmt.Errorf("count new pool (%s): %w", scope, err)
	}
	return b, nil
}

// breakdownFor picks one scope's entry; a scope the list does not carry comes
// back zero-valued rather than as somebody else's numbers.
func breakdownFor(bds []Breakdown, scope Scope) Breakdown {
	for _, b := range bds {
		if b.Scope == scope {
			return b
		}
	}
	return Breakdown{Scope: scope}
}

func sumBreakdowns(scope Scope, parts ...Breakdown) Breakdown {
	out := Breakdown{Scope: scope}
	for _, p := range parts {
		out.DueNow += p.DueNow
		out.DueLaterToday += p.DueLaterToday
		out.BuriedToday += p.BuriedToday
		out.NewCards += p.NewCards
		out.NewNotes += p.NewNotes
		if p.NextDueAt != nil && (out.NextDueAt == nil || p.NextDueAt.Before(*out.NextDueAt)) {
			t := *p.NextDueAt
			out.NextDueAt = &t
		}
	}
	return out
}
