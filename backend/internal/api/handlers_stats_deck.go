package api

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/db"
	"tanulom/internal/srs"
)

// DeckStageCounts is the learner's lemmas by the Words list's stage rule.
type DeckStageCounts struct {
	Learning   int `json:"learning"`
	Relearning int `json:"relearning"`
	Young      int `json:"young"`
	Mature     int `json:"mature"`
	Known      int `json:"known"`
}

// DeckStages is GET /api/stats/deck: WORDS (lemmas) by life stage. It is not a
// review-queue figure and must never be shown as a due count — those come from
// srs.Breakdowns. `learning` here means "answered at least once and still in
// its learning steps"; a word never reviewed is counted under `new` and NOT
// under `learning` (the Words list shows it as learning, «ещё не
// повторялось»). total == learning+relearning+young+mature+known+new, which
// equals the Words list total for status=all.
type DeckStages struct {
	Total  int             `json:"total"`
	Stages DeckStageCounts `json:"stages"`
	New    int             `json:"new"`
}

func (s *Server) handleStatsDeck(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	now := time.Now()
	// Make sure a fresh cache entry exists, then hang the deck figures on it so
	// recordInteraction / the TTL drop them with the card scan.
	if _, _, err := s.countedAndSettled(u.ID, now); err != nil {
		return err
	}
	s.countedMu.Lock()
	if e, ok := s.countedCache[u.ID]; ok && e.deck != nil {
		d := *e.deck
		s.countedMu.Unlock()
		return c.JSON(d)
	}
	s.countedMu.Unlock()

	d, err := s.scanDeckStages(u.ID, now)
	if err != nil {
		return err
	}
	s.countedMu.Lock()
	if e, ok := s.countedCache[u.ID]; ok {
		e.deck = &d
		s.countedCache[u.ID] = e
	}
	s.countedMu.Unlock()
	return c.JSON(d)
}

// scanDeckStages mirrors handleWords: lemma_state rows other than `new`, the
// strongest non-suspended, non-trashed card per lemma (retrievability, then
// reps), and wordStage. One query for states, one for cards.
func (s *Server) scanDeckStages(userID int64, now time.Time) (DeckStages, error) {
	var d DeckStages
	rows, err := s.db.Query(`
		SELECT lemma_id, status FROM lemma_state WHERE user_id = ? AND status != 'new'`, userID)
	if err != nil {
		return d, err
	}
	status := map[int64]string{}
	for rows.Next() {
		var id int64
		var st string
		if err := rows.Scan(&id, &st); err != nil {
			rows.Close()
			return d, err
		}
		status[id] = st
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return d, err
	}

	sched := s.engine().Scheduler()
	crows, err := s.db.Query(`
		SELECT n.lemma_id, c.due, c.stability, c.difficulty, c.scheduled_days,
		       c.reps, c.lapses, c.state, c.last_review, c.remaining_steps
		  FROM card c JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND n.lemma_id IS NOT NULL AND c.suspended = 0`+
		srs.NotTrashed, userID)
	if err != nil {
		return d, err
	}
	best := map[int64]srs.Memory{}
	for crows.Next() {
		var lemmaID int64
		var st srs.CardState
		var due string
		var last sql.NullString
		if err := crows.Scan(&lemmaID, &due, &st.Stability, &st.Difficulty,
			&st.ScheduledDays, &st.Reps, &st.Lapses, &st.State, &last,
			&st.RemainingSteps); err != nil {
			crows.Close()
			return d, err
		}
		if _, ok := status[lemmaID]; !ok {
			continue
		}
		st.Due = db.ParseTime(sql.NullString{String: due, Valid: true})
		st.LastReview = db.ParseTime(last)
		mem := srs.MemoryFor(st, sched, now, false)
		if cur, seen := best[lemmaID]; !seen || mem.Retrievability > cur.Retrievability ||
			(mem.Retrievability == cur.Retrievability && mem.Reps > cur.Reps) {
			best[lemmaID] = mem
		}
	}
	crows.Close()
	if err := crows.Err(); err != nil {
		return d, err
	}

	for id, stt := range status {
		m, ok := best[id]
		switch wordStage(stt, m, ok) {
		case "known":
			d.Stages.Known++
		case "relearning":
			d.Stages.Relearning++
		case "young":
			d.Stages.Young++
		case "mature":
			d.Stages.Mature++
		case "learning":
			if ok && m.Stage == srs.StageLearning {
				d.Stages.Learning++
			} else {
				d.New++ // in the deck, no reviewed card yet
			}
		default:
			continue // e.g. ignored with no card: the list gives it no stage
		}
		d.Total++
	}
	return d, nil
}
