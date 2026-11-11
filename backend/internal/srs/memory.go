package srs

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"tanulom/internal/db"
)

// What a card knows about itself, for the learner to see while answering.
//
// [R18] Judgments of learning drive study choices, and they can be calibrated
// by feedback (Metcalfe 2009); informational feedback — how you did, measured —
// does not undermine intrinsic motivation the way rewards do (Deci, Koestner &
// Ryan 1999). So the card carries its memory figures: the chance of recalling
// it now, its stage, its history, and what each grade would schedule. Every
// number here is MEASURED or computed by the library. There is no score.
// See .claude/research/18-sessions-and-progress.md and ADR-026.

// Stage names where a card is in its life, in the vocabulary Anki users
// already read: young and mature split at a 21-day interval. That is a
// reporting convention, not a memory threshold, and the UI says so.
type Stage string

const (
	StageNew        Stage = "new"
	StageLearning   Stage = "learning"
	StageYoung      Stage = "young"
	StageMature     Stage = "mature"
	StageRelearning Stage = "relearning"
)

// MatureDays is Anki's convention for a mature card: an interval of 21 days or
// more. https://docs.ankiweb.net/stats.html
const MatureDays = 21

// StageOf classifies a stored FSRS state.
func StageOf(st CardState) Stage {
	switch st.State {
	case 1:
		return StageLearning
	case 3:
		return StageRelearning
	case 2:
		if st.ScheduledDays >= MatureDays {
			return StageMature
		}
		return StageYoung
	}
	return StageNew
}

// Preview is what each grade would schedule, as due instants, and At is the
// instant they were computed FROM. Approximate on purpose: interval fuzz is
// seeded at answer time, so the real due date can land a day or two off the
// figure shown. The client labels it as such — and reads each interval as
// `due − at`, never `due − now`: the previews are computed when the queue is
// served, and a learner who reached the card three minutes later saw the
// 10-minute step as «~7 мин» and the 6-minute one as «~3 мин», and asked,
// reasonably, whether the maths was right.
type Preview struct {
	At    time.Time `json:"at"`
	Again time.Time `json:"again"`
	Hard  time.Time `json:"hard"`
	Good  time.Time `json:"good"`
	Easy  time.Time `json:"easy"`
}

// Memory is one card's memory figures.
type Memory struct {
	Stage Stage `json:"stage"`
	// Retrievability is the chance of recalling this card right now, from the
	// library's forgetting curve — the same figure the words browser shows.
	// Zero for a card never reviewed.
	Retrievability float64 `json:"retrievability"`
	// StabilityDays is FSRS's stability: the interval at which retrievability
	// would fall to 90%. IntervalDays is the interval actually scheduled.
	StabilityDays float64 `json:"stabilityDays"`
	IntervalDays  uint64  `json:"intervalDays"`
	Reps          uint64  `json:"reps"`
	Lapses        uint64  `json:"lapses"`
	// Correct/Total count this card's answers in the review log; the Word pair
	// counts every card of the note, so "this word: 5 of 7" is answerable.
	Correct     int `json:"correct"`
	Total       int `json:"total"`
	WordCorrect int `json:"wordCorrect"`
	WordTotal   int `json:"wordTotal"`
	// History is the last outcomes on this card, oldest first, at most
	// historyLength of them.
	History []bool `json:"history"`
	// Preview is absent where the caller asked for none (free practice).
	Preview *Preview `json:"preview,omitempty"`

	// The learning ladder, so «вернётся через ~10 мин» can say what it means.
	//
	// A card in Learning or Relearning is walking FSRS's minute-scale steps
	// ({1, 10} and {10} by default). StepsTotal is how many that state has and
	// StepsLeft is how many remain — which is `CardState.RemainingSteps`, the
	// scheduler's own position, not a count we keep. Both are 0 once the card
	// is scheduled in days, and the UI switches to interval language there.
	//
	// GraduatesOnGood is the answer to the question the learner actually
	// asked — "when does this start coming back in days" — and it is asked of
	// the SCHEDULER (Scheduler.PreviewNext), never derived from the numbers
	// here. There is no "N correct in a row" threshold in FSRS and the UI must
	// not invent one; this is the real, and much smaller, thing that is true.
	StepsLeft  int `json:"stepsLeft"`
	StepsTotal int `json:"stepsTotal"`
	// GraduatesOnGood is meaningful only alongside Preview: it is computed
	// from the same dry run, so it is absent whenever Preview is.
	GraduatesOnGood bool `json:"graduatesOnGood"`

	// Progress is the word-progress number, 0-100, derived from the stage and
	// stability above (progress.go, ADR-070). [R20]
	Progress int `json:"progress"`
}

// stepsFor returns how many minute-scale steps the card's state has, and how
// many are left. Zero for anything already scheduled in days.
func stepsFor(st CardState, sched *Scheduler) (left, total int) {
	switch st.State {
	case 1:
		total = len(sched.LearningSteps())
	case 3:
		total = len(sched.RelearningSteps())
	default:
		return 0, 0
	}
	left = st.RemainingSteps
	if left < 0 {
		left = 0
	}
	if left > total {
		left = total
	}
	return left, total
}

// historyLength bounds the row of marks the card draws.
const historyLength = 8

// MemoryFor computes the pure part of a card's memory: everything that follows
// from its stored state alone.
func MemoryFor(st CardState, sched *Scheduler, now time.Time, withPreview bool) Memory {
	m := Memory{
		Stage:          StageOf(st),
		Retrievability: sched.Retrievability(st, now),
		StabilityDays:  st.Stability,
		IntervalDays:   st.ScheduledDays,
		Reps:           st.Reps,
		Lapses:         st.Lapses,
		History:        []bool{},
	}
	m.StepsLeft, m.StepsTotal = stepsFor(st, sched)
	m.Progress = progressOf(m)
	if withPreview {
		if next, err := sched.PreviewNext(st, now); err == nil {
			m.Preview = &Preview{
				At:    now,
				Again: next[Again].Due, Hard: next[Hard].Due,
				Good: next[Good].Due, Easy: next[Easy].Due,
			}
			// State 2 is Review: off the minute-scale steps and onto days.
			// Asked of the scheduler rather than worked out from the interval,
			// because "more than a day" is a guess and this is a fact.
			m.GraduatesOnGood = st.State != 2 && next[Good].State == 2
		}
	}
	return m
}

// AttachMemory fills Memory for every queue item, in two queries for the whole
// queue rather than two per card.
//
// withPreview is false for free practice: the learner picks the order there,
// and a «→ 8 дн» under a practice card would promise a schedule the answer
// may not make (new cards and «Знаю» words stay unscheduled; ADR-064).
func (s *Store) AttachMemory(items []QueueItem, sched *Scheduler, now time.Time, withPreview bool) error {
	if len(items) == 0 {
		return nil
	}
	cardIDs := make([]int64, 0, len(items))
	noteIDs := make([]int64, 0, len(items))
	seenNote := map[int64]bool{}
	for _, it := range items {
		cardIDs = append(cardIDs, it.CardID)
		if !seenNote[it.NoteID] {
			seenNote[it.NoteID] = true
			noteIDs = append(noteIDs, it.NoteID)
		}
	}
	stats, err := s.memoryStats(cardIDs, noteIDs)
	if err != nil {
		return err
	}
	for i := range items {
		it := &items[i]
		m := MemoryFor(it.fsrs, sched, now, withPreview)
		stats.fill(&m, it.CardID, it.NoteID)
		it.Memory = &m
	}
	return nil
}

// CardMemory computes one card's memory after an answer, from the state the
// answer left it in.
func (s *Store) CardMemory(cardID, noteID int64, st CardState, sched *Scheduler, now time.Time) (Memory, error) {
	stats, err := s.memoryStats([]int64{cardID}, []int64{noteID})
	if err != nil {
		return Memory{}, err
	}
	m := MemoryFor(st, sched, now, false)
	stats.fill(&m, cardID, noteID)
	return m, nil
}

// memoryTallies is what the review log says about a set of cards and notes.
type memoryTallies struct {
	cardHistory map[int64][]bool
	cardCorrect map[int64]int
	cardTotal   map[int64]int
	noteCorrect map[int64]int
	noteTotal   map[int64]int
}

func (t memoryTallies) fill(m *Memory, cardID, noteID int64) {
	m.Correct, m.Total = t.cardCorrect[cardID], t.cardTotal[cardID]
	m.WordCorrect, m.WordTotal = t.noteCorrect[noteID], t.noteTotal[noteID]
	if h := t.cardHistory[cardID]; len(h) > 0 {
		if len(h) > historyLength {
			h = h[len(h)-historyLength:]
		}
		m.History = h
	}
}

// memoryStats reads the tallies in chunks, so a long queue cannot exceed the
// bound on SQL variables.
func (s *Store) memoryStats(cardIDs, noteIDs []int64) (memoryTallies, error) {
	t := memoryTallies{
		cardHistory: map[int64][]bool{}, cardCorrect: map[int64]int{}, cardTotal: map[int64]int{},
		noteCorrect: map[int64]int{}, noteTotal: map[int64]int{},
	}
	const chunk = 500
	for start := 0; start < len(cardIDs); start += chunk {
		ids := cardIDs[start:min(start+chunk, len(cardIDs))]
		marks, args := idList(ids)
		rows, err := s.db.Query(`
			SELECT card_id, correct FROM review_log
			 WHERE card_id IN (`+marks+`) ORDER BY card_id, id`, args...)
		if err != nil {
			return t, fmt.Errorf("load card history: %w", err)
		}
		for rows.Next() {
			var id int64
			var correct int
			if err := rows.Scan(&id, &correct); err != nil {
				rows.Close()
				return t, err
			}
			t.cardHistory[id] = append(t.cardHistory[id], correct == 1)
			t.cardTotal[id]++
			if correct == 1 {
				t.cardCorrect[id]++
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return t, err
		}
	}
	for start := 0; start < len(noteIDs); start += chunk {
		ids := noteIDs[start:min(start+chunk, len(noteIDs))]
		marks, args := idList(ids)
		rows, err := s.db.Query(`
			SELECT c.note_id, COUNT(*), SUM(r.correct)
			  FROM review_log r JOIN card c ON c.id = r.card_id
			 WHERE c.note_id IN (`+marks+`) GROUP BY c.note_id`, args...)
		if err != nil {
			return t, fmt.Errorf("load word tallies: %w", err)
		}
		for rows.Next() {
			var id int64
			var total, correct int
			if err := rows.Scan(&id, &total, &nullInt{&correct}); err != nil {
				rows.Close()
				return t, err
			}
			t.noteTotal[id], t.noteCorrect[id] = total, correct
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return t, err
		}
	}
	return t, nil
}

func idList(ids []int64) (string, []any) {
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	return strings.TrimSuffix(strings.Repeat("?,", len(ids)), ","), args
}

// scanCardState reads the FSRS columns selectQueue now carries, so the memory
// figures can be computed from what is stored rather than re-read per card.
func scanCardState(st *CardState, due string, last sql.NullString) {
	st.Due = db.ParseTime(sql.NullString{String: due, Valid: true})
	st.LastReview = db.ParseTime(last)
}
