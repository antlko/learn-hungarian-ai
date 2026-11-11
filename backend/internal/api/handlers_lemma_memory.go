package api

import (
	"database/sql"
	"time"

	"tanulom/internal/db"
	"tanulom/internal/srs"
)

// lemmaMemory builds the word inspector's memory block: the learner's cards of
// one lemma, the best one by the Words list's rule (highest retrievability,
// then reps) giving the headline figures. Nil, not an error, when the word has
// no card in the deck. [R20] ADR-070.
func (s *Server) lemmaMemory(userID, lemmaID int64) (*LemmaMemory, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.note_id, c.direction, c.due, c.stability, c.difficulty,
		       c.scheduled_days, c.reps, c.lapses, c.state, c.last_review, c.remaining_steps
		  FROM card c JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND n.lemma_id = ? AND c.suspended = 0`+srs.NotTrashed+`
		 ORDER BY c.id`, userID, lemmaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sched := s.engine().Scheduler()
	now := time.Now()
	type card struct {
		id, noteID int64
		st         srs.CardState
		mem        srs.Memory
	}
	var cards []card
	out := &LemmaMemory{History: []bool{}, Cards: []LemmaCardMemory{}}
	bestIdx := -1
	var nextDue time.Time
	for rows.Next() {
		var c card
		var dir, due string
		var last sql.NullString
		if err := rows.Scan(&c.id, &c.noteID, &dir, &due, &c.st.Stability, &c.st.Difficulty,
			&c.st.ScheduledDays, &c.st.Reps, &c.st.Lapses, &c.st.State, &last,
			&c.st.RemainingSteps); err != nil {
			return nil, err
		}
		c.st.Due = db.ParseTime(sql.NullString{String: due, Valid: true})
		c.st.LastReview = db.ParseTime(last)
		c.mem = srs.MemoryFor(c.st, sched, now, false)
		cards = append(cards, c)
		out.Cards = append(out.Cards, LemmaCardMemory{
			Direction: dir, Progress: c.mem.Progress, Stage: string(c.mem.Stage)})
		out.Lapses += int(c.st.Lapses)
		if c.st.State != 0 && !c.st.Due.IsZero() && (nextDue.IsZero() || c.st.Due.Before(nextDue)) {
			nextDue = c.st.Due
		}
		if bestIdx < 0 || c.mem.Retrievability > cards[bestIdx].mem.Retrievability ||
			(c.mem.Retrievability == cards[bestIdx].mem.Retrievability && c.mem.Reps > cards[bestIdx].mem.Reps) {
			bestIdx = len(cards) - 1
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if bestIdx < 0 {
		return nil, nil
	}
	best := cards[bestIdx]
	// CardMemory adds the review-log history to the pure figures.
	m, err := srs.NewStore(s.db.DB).CardMemory(best.id, best.noteID, best.st, sched, now)
	if err != nil {
		return nil, err
	}
	out.Progress, out.Stage = m.Progress, string(m.Stage)
	out.IntervalDays, out.Reps = int(m.IntervalDays), int(m.Reps)
	out.Retrievability = m.Retrievability
	if m.History != nil {
		out.History = m.History
	}
	if !nextDue.IsZero() {
		out.NextDue = db.FmtTime(nextDue)
	}
	return out, nil
}
