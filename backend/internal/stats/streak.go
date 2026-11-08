// Package stats aggregates learner progress: the daily ledger, the streak, the
// mastery heatmap and the checkpoints.
package stats

import (
	"database/sql"
	"fmt"
	"time"

	"tanulom/internal/db"
)

// The streak. [R14] — .claude/research/14-motivation.md:
//
//	"A day counts when the learner clears their due reviews and meets their
//	 stated minutes — not when they tap something. A streak you can keep by
//	 opening the app for 30 seconds is a streak that measures nothing."
//
// Implementation follows from that sentence. daily_log rows are written as the
// learner works (see Record); DayMet applies the two obligations; the streak
// walks backwards until a day fails. A day with no row breaks the streak.

// Day is one day's ledger.
type Day struct {
	Day     string  `json:"day"`
	Reviews int     `json:"reviews"`
	Drills  int     `json:"drills"`
	Trials  int     `json:"trials"`
	ListenS float64 `json:"listenS"`
	// ReadS is time spent in the reader, reported the same clamped way
	// listening is. Carried separately from ActiveMs so the statistics screen
	// can say where an hour went rather than only how long it was.
	ReadS    float64 `json:"readS"`
	ActiveMs int64   `json:"activeMs"`
	DueLeft  int     `json:"dueLeft"`
	Human    bool    `json:"human"`
}

// Met applies the obligation rule at a given daily-minutes target.
func (d Day) Met(dailyMinutes int) bool {
	return d.DueLeft == 0 && d.ActiveMs >= int64(dailyMinutes)*60*1000
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// Record folds one graded interaction into today's row. `kind` selects the
// counter; durations accumulate into active_ms; dueLeft overwrites (it is the
// state at this moment, not a sum).
func (s *Store) Record(userID int64, day string, kind string, durationMs int64, dueLeft int) error {
	col := map[string]string{
		"review": "reviews", "drill": "drills", "trial": "trials",
	}[kind]
	if col == "" {
		return fmt.Errorf("unknown interaction kind %q", kind)
	}
	_, err := s.db.Exec(`
		INSERT INTO daily_log (user_id, day, `+col+`, active_ms, due_left, updated_at)
		VALUES (?,?,1,?,?,?)
		ON CONFLICT(user_id, day) DO UPDATE SET
		  `+col+` = `+col+` + 1,
		  active_ms = active_ms + excluded.active_ms,
		  due_left = excluded.due_left,
		  updated_at = excluded.updated_at`,
		userID, day, durationMs, dueLeft, db.FmtTime(time.Now()))
	if err != nil {
		return fmt.Errorf("record %s: %w", kind, err)
	}
	return nil
}

// RecordListening folds heard seconds into today's row. Listening time counts
// toward the minutes obligation: it is real work.
func (s *Store) RecordListening(userID int64, day string, seconds float64) error {
	_, err := s.db.Exec(`
		INSERT INTO daily_log (user_id, day, listen_s, active_ms, updated_at)
		VALUES (?,?,?,?,?)
		ON CONFLICT(user_id, day) DO UPDATE SET
		  listen_s = listen_s + excluded.listen_s,
		  active_ms = active_ms + excluded.active_ms,
		  updated_at = excluded.updated_at`,
		userID, day, seconds, int64(seconds*1000), db.FmtTime(time.Now()))
	return err
}

// RecordReading folds seconds spent with a text into today's row.
//
// Reading counts toward the minutes obligation for the same reason listening
// does: it is real work, and the app that told a learner otherwise was simply
// wrong about their day. Clamped by the caller, exactly like listening — see
// handlers_texts.go — because the only thing standing between "read for a
// minute" and "left the tab open" is that clamp.
func (s *Store) RecordReading(userID int64, day string, seconds float64) error {
	_, err := s.db.Exec(`
		INSERT INTO daily_log (user_id, day, read_s, active_ms, updated_at)
		VALUES (?,?,?,?,?)
		ON CONFLICT(user_id, day) DO UPDATE SET
		  read_s = read_s + excluded.read_s,
		  active_ms = active_ms + excluded.active_ms,
		  updated_at = excluded.updated_at`,
		userID, day, seconds, int64(seconds*1000), db.FmtTime(time.Now()))
	return err
}

// AttestHuman records the learner's claim that live practice happened today.
// The app cannot verify this and the streak ignores it — it exists for the
// checkpoint diagnostic. See [R14] and the Loop 4 honesty rule.
func (s *Store) AttestHuman(userID int64, day string, happened bool) error {
	v := 0
	if happened {
		v = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO daily_log (user_id, day, human, updated_at) VALUES (?,?,?,?)
		ON CONFLICT(user_id, day) DO UPDATE SET
		  human = excluded.human, updated_at = excluded.updated_at`,
		userID, day, v, db.FmtTime(time.Now()))
	return err
}

// Days loads the ledger newest-first, up to limit.
func (s *Store) Days(userID int64, limit int) ([]Day, error) {
	if limit <= 0 {
		limit = 400
	}
	rows, err := s.db.Query(`
		SELECT day, reviews, drills, trials, listen_s, read_s, active_ms, due_left, human
		  FROM daily_log WHERE user_id = ? ORDER BY day DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("load daily log: %w", err)
	}
	defer rows.Close()
	var out []Day
	for rows.Next() {
		var d Day
		var human int
		if err := rows.Scan(&d.Day, &d.Reviews, &d.Drills, &d.Trials,
			&d.ListenS, &d.ReadS, &d.ActiveMs, &d.DueLeft, &human); err != nil {
			return nil, err
		}
		d.Human = human == 1
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListeningHours is the learner's accumulated listening volume — the figure
// that unlocks the word-order topics. [R15]
//
// Read from the daily ledger rather than from listen_log, although listen_log
// is the append-only record of what was actually played. The two hold the same
// clamped seconds, but listen_log grows about 240 rows per listening hour while
// this sum is wanted on every curriculum resolve; daily_log answers it from one
// row per day. Hours still only accumulate from reported playback — nothing
// here lets them be claimed.
func (s *Store) ListeningHours(userID int64) (float64, error) {
	var seconds float64
	err := s.db.QueryRow(
		`SELECT COALESCE(SUM(listen_s),0) FROM daily_log WHERE user_id = ?`,
		userID).Scan(&seconds)
	return seconds / 3600, err
}

// Streak is the computed state.
type Streak struct {
	// Current counts consecutive met days ending today or yesterday — today is
	// allowed to be in progress without breaking the run.
	Current int `json:"current"`
	Best    int `json:"best"`
	// TodayMet reports whether today's obligations are already met.
	TodayMet bool `json:"todayMet"`
}

// ComputeStreak walks the ledger. `today` is the user-local YYYY-MM-DD.
func ComputeStreak(days []Day, today string, dailyMinutes int) Streak {
	met := map[string]bool{}
	for _, d := range days {
		met[d.Day] = d.Met(dailyMinutes)
	}
	st := Streak{TodayMet: met[today]}

	// Current: walk back from today (or yesterday, if today is not yet met —
	// an in-progress day must not read as a broken streak at 9 in the morning).
	cursor, _ := time.Parse("2006-01-02", today)
	if !met[today] {
		cursor = cursor.AddDate(0, 0, -1)
	}
	for {
		key := cursor.Format("2006-01-02")
		if !met[key] {
			break
		}
		st.Current++
		cursor = cursor.AddDate(0, 0, -1)
	}

	// Best: longest run anywhere in the ledger.
	run := 0
	var prev time.Time
	for i := len(days) - 1; i >= 0; i-- { // oldest first
		d := days[i]
		if !d.Met(dailyMinutes) {
			run = 0
			continue
		}
		t, err := time.Parse("2006-01-02", d.Day)
		if err != nil {
			continue
		}
		if !prev.IsZero() && t.Sub(prev) == 24*time.Hour {
			run++
		} else {
			run = 1
		}
		prev = t
		if run > st.Best {
			st.Best = run
		}
	}
	if st.Current > st.Best {
		st.Best = st.Current
	}
	return st
}

// RecordPractice folds free practice into today's row.
//
// Minutes only, and deliberately no counter of its own in daily_log: the volume
// and accuracy live in practice_log, where they can be reported honestly as
// practice rather than blurred into the review count.
//
// It does NOT write due_left. Practice clears no obligation, so the day's
// standing must be whatever the review queue last left it at — otherwise a
// practice session could refresh a stale zero and hand out a streak nobody
// earned. See [R14] and ADR-019.
func (s *Store) RecordPractice(userID int64, day string, durationMs int64) error {
	_, err := s.db.Exec(`
		INSERT INTO daily_log (user_id, day, active_ms, updated_at)
		VALUES (?,?,?,?)
		ON CONFLICT(user_id, day) DO UPDATE SET
		  active_ms = active_ms + excluded.active_ms,
		  updated_at = excluded.updated_at`,
		userID, day, durationMs, db.FmtTime(time.Now()))
	if err != nil {
		return fmt.Errorf("record practice: %w", err)
	}
	return nil
}
