package api

import (
	"database/sql"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/db"
	"tanulom/internal/morph"
	"tanulom/internal/srs"
)

// The words browser: «что выучилось, что нет» as a list a learner can read,
// search and sort — not a percentage.
//
// Everything here is derived, never stored: status from lemma_state (the
// learner's declarations plus MarkLemmaLearning), and the memory figures from
// the lemma's CARDS — lemma_state's own FSRS columns are dormant by design,
// only its status is ever written. Retrievability is the strongest card's,
// computed by the library (ADR-003); next due is the earliest scheduled card.

// WordRow is one word of the learner's deck.
type WordRow struct {
	LemmaID int64  `json:"lemmaId"`
	Lemma   string `json:"lemma"`
	GlossRU string `json:"glossRu"`
	GlossEN string `json:"glossEn"`
	Status  string `json:"status"`
	// FirstSeen is when the word entered play; "" for a bare declaration made
	// before first_seen existed.
	FirstSeen string `json:"firstSeen,omitempty"`
	// Retrievability is the chance of recalling it right now — the strongest
	// card's figure, 0 when nothing was ever reviewed.
	Retrievability float64 `json:"retrievability"`
	NextDue        string  `json:"nextDue,omitempty"`
	Lapses         int     `json:"lapses"`
	Cards          int     `json:"cards"`
	// Stage is where the word is in its life, in the review card's own
	// vocabulary: "learning" | "relearning" | "young" | "mature" | "known".
	// "known" is the learner's declaration («Знаю»), which outranks the
	// cards; otherwise it is srs.StageOf the STRONGEST card (young/mature
	// split at srs.MatureDays, Anki's convention). A word with no reviewed
	// card yet is "learning". Empty only for an ignored word with no cards.
	Stage string `json:"stage"`
	// IntervalDays, Reps, StepsLeft and StepsTotal are the strongest card's
	// srs.Memory figures — computed by the library, never re-derived here.
	IntervalDays int `json:"intervalDays"`
	Reps         int `json:"reps"`
	StepsLeft    int `json:"stepsLeft"`
	StepsTotal   int `json:"stepsTotal"`
	// Progress is the strongest card's 0-100 progress number (ADR-070, [R20]);
	// a learning word with no reviewed card is 0. Absent for a «Знаю» word.
	Progress *int `json:"progress,omitempty"`
}

// wordStages are the values of WordRow.Stage, and of the ?stage= filter.
var wordStages = map[string]bool{
	"learning": true, "relearning": true, "young": true, "mature": true, "known": true,
}

// WordsResponse is one page of the browser.
type WordsResponse struct {
	Words    []WordRow `json:"words"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
}

func (s *Server) handleWords(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}

	status := c.Query("status")
	switch status {
	case "", "all", "learning", "known", "ignored":
	default:
		return badRequest("status must be all, learning, known or ignored")
	}
	stage := c.Query("stage")
	if stage != "" && stage != "all" && !wordStages[stage] {
		return badRequest("stage must be learning, relearning, young, mature or known")
	}
	sortBy := c.Query("sort", "added")
	switch sortBy {
	case "added", "word", "due", "retrievability", "progress":
	default:
		return badRequest("sort must be added, word, due, retrievability or progress")
	}
	page, _ := strconv.Atoi(c.Query("page", "1"))
	page = max(1, page)
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "50"))
	pageSize = min(max(pageSize, 10), 200)

	// The learner's words: everything ever put in play or declared.
	rows, err := s.db.Query(`
		SELECT lemma_id, status, COALESCE(first_seen, '')
		  FROM lemma_state WHERE user_id = ? AND status != 'new'`, u.ID)
	if err != nil {
		return err
	}
	byID := map[int64]*WordRow{}
	best := map[int64]srs.Memory{} // the strongest card's figures, per lemma
	for rows.Next() {
		var w WordRow
		if err := rows.Scan(&w.LemmaID, &w.Status, &w.FirstSeen); err != nil {
			rows.Close()
			return err
		}
		byID[w.LemmaID] = &w
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// The memory figures come from the cards. One pass over the deck; the
	// strongest card's retrievability, the earliest scheduled due, every lapse.
	sched := s.engine().Scheduler()
	now := time.Now()
	crows, err := s.db.Query(`
		SELECT n.lemma_id, c.due, c.stability, c.difficulty, c.scheduled_days,
		       c.reps, c.lapses, c.state, c.last_review, c.remaining_steps
		  FROM card c JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND n.lemma_id IS NOT NULL AND c.suspended = 0`+
		srs.NotTrashed, u.ID)
	if err != nil {
		return err
	}
	for crows.Next() {
		var lemmaID int64
		var st srs.CardState
		var due string
		var last sql.NullString
		if err := crows.Scan(&lemmaID, &due, &st.Stability, &st.Difficulty,
			&st.ScheduledDays, &st.Reps, &st.Lapses, &st.State, &last,
			&st.RemainingSteps); err != nil {
			crows.Close()
			return err
		}
		w, ok := byID[lemmaID]
		if !ok {
			continue
		}
		st.Due = db.ParseTime(sql.NullString{String: due, Valid: true})
		st.LastReview = db.ParseTime(last)
		w.Cards++
		w.Lapses += int(st.Lapses)
		mem := srs.MemoryFor(st, sched, now, false)
		if r := mem.Retrievability; r > w.Retrievability {
			w.Retrievability = r
		}
		if cur, seen := best[lemmaID]; !seen || mem.Retrievability > cur.Retrievability ||
			(mem.Retrievability == cur.Retrievability && mem.Reps > cur.Reps) {
			best[lemmaID] = mem
		}
		if st.State != 0 && !st.Due.IsZero() {
			if w.NextDue == "" || st.Due.Before(db.ParseTime(sql.NullString{String: w.NextDue, Valid: true})) {
				w.NextDue = db.FmtTime(st.Due)
			}
		}
	}
	crows.Close()
	if err := crows.Err(); err != nil {
		return err
	}

	for id, w := range byID {
		m, ok := best[id]
		w.Stage = wordStage(w.Status, m, ok)
		if w.Stage != "" && w.Stage != "known" {
			p := 0
			if ok {
				p = m.Progress
			}
			w.Progress = &p
		}
		if ok {
			w.IntervalDays, w.Reps = int(m.IntervalDays), int(m.Reps)
			w.StepsLeft, w.StepsTotal = m.StepsLeft, m.StepsTotal
		}
	}

	// Names and glosses from the pack, in chunks — the pack is a separate
	// database handle, so this is the two-query pattern, never a SQL join.
	ids := make([]int64, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	for start := 0; start < len(ids); start += 500 {
		chunk := ids[start:min(start+500, len(ids))]
		data, err := s.lemmaData(chunk)
		if err != nil {
			return err
		}
		for id, d := range data {
			byID[id].Lemma, byID[id].GlossRU, byID[id].GlossEN = d.Lemma, d.GlossRU, d.GlossEN
		}
	}

	// Search: resolve the query to candidate lemma ids in the PACK first, via
	// case variants — `WHERE form = ? COLLATE NOCASE` scans two million rows
	// (see internal/morph/casefold.go) — then intersect with the deck. A match
	// on the dictionary form or on the gloss text both count: a learner
	// searches in whichever language the word surfaces in their memory.
	q := strings.TrimSpace(c.Query("q"))
	all := make([]WordRow, 0, len(byID))
	if q != "" {
		match := map[int64]bool{}
		if s.pack != nil {
			variants := morph.CaseVariants(q)
			args := make([]any, 0, len(variants))
			for _, v := range variants {
				args = append(args, escapeLike(v)+"%")
			}
			prows, err := s.pack.DB().Query(`
				SELECT id FROM lemma WHERE `+likeAny("lemma", len(variants)), args...)
			if err == nil {
				for prows.Next() {
					var id int64
					if err := prows.Scan(&id); err == nil {
						match[id] = true
					}
				}
				prows.Close()
			}
		}
		lq := strings.ToLower(q)
		for _, w := range byID {
			if match[w.LemmaID] ||
				strings.Contains(strings.ToLower(w.GlossRU), lq) ||
				strings.Contains(strings.ToLower(w.GlossEN), lq) {
				all = append(all, *w)
			}
		}
	} else {
		for _, w := range byID {
			all = append(all, *w)
		}
	}

	if status != "" && status != "all" {
		kept := all[:0]
		for _, w := range all {
			if w.Status == status {
				kept = append(kept, w)
			}
		}
		all = kept
	}

	if stage != "" && stage != "all" {
		kept := all[:0]
		for _, w := range all {
			if w.Stage == stage {
				kept = append(kept, w)
			}
		}
		all = kept
	}

	sort.Slice(all, func(i, j int) bool {
		a, b := all[i], all[j]
		switch sortBy {
		case "word":
			if a.Lemma != b.Lemma {
				return a.Lemma < b.Lemma
			}
		case "due":
			// Words with a schedule first, earliest first.
			if (a.NextDue == "") != (b.NextDue == "") {
				return a.NextDue != ""
			}
			if a.NextDue != b.NextDue {
				return a.NextDue < b.NextDue
			}
		case "retrievability":
			if a.Retrievability != b.Retrievability {
				return a.Retrievability < b.Retrievability
			}
		case "progress":
			// Lowest first, like retrievability; a word without one (known) last.
			if (a.Progress == nil) != (b.Progress == nil) {
				return a.Progress != nil
			}
			if a.Progress != nil && *a.Progress != *b.Progress {
				return *a.Progress < *b.Progress
			}
		default: // added: newest first
			if a.FirstSeen != b.FirstSeen {
				return a.FirstSeen > b.FirstSeen
			}
		}
		return a.LemmaID < b.LemmaID
	})

	total := len(all)
	from := min((page-1)*pageSize, total)
	to := min(from+pageSize, total)
	pageRows := all[from:to]
	if pageRows == nil {
		pageRows = []WordRow{}
	}
	return c.JSON(WordsResponse{Words: pageRows, Total: total, Page: page, PageSize: pageSize})
}

// likeAny builds `(col LIKE ? ESCAPE '\' OR ...)` for prefix search over the
// pack's BINARY-collated index — one arm per case variant, never NOCASE.
func likeAny(col string, n int) string {
	arms := make([]string, n)
	for i := range arms {
		arms[i] = col + ` LIKE ? ESCAPE '\'`
	}
	return "(" + strings.Join(arms, " OR ") + ")"
}

// escapeLike neutralises LIKE metacharacters in learner input, so searching
// for «100%» is a search and not a wildcard.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	return strings.ReplaceAll(s, `_`, `\_`)
}

// wordStage is the ONE stage rule of a word, shared by the Words list and
// GET /api/stats/deck so their numbers cannot disagree: `known` status
// outranks; else the strongest card's stage; a learning word with no reviewed
// card yet is "learning" (the list's «ещё не повторялось»).
func wordStage(status string, best srs.Memory, hasCard bool) string {
	switch {
	case status == "known":
		return "known"
	case hasCard && best.Stage != srs.StageNew:
		return string(best.Stage)
	case status == "learning":
		return "learning"
	}
	return ""
}
