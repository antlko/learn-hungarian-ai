// Package course owns the guided syllabus: the ordered vocabulary plan the
// learner follows before they are ready to choose their own material.
//
// Why this exists at all. The grammar DAG (internal/curriculum) fixes the order
// RULES arrive in and says nothing about words; the miner (internal/mining)
// finds good sentences but only once there is a vocabulary to score against.
// Between them sits the question every beginner actually asks — "what do I do
// today?" — and "mine your own sentences" is not an answer to it, it is a
// research project handed to someone who cannot yet read a shop sign.
//
// So the pack ships twenty-four units across six months, each naming the errand
// it buys ("I can register at the doctor"), the grammar it leans on, and the
// words to own. The app prescribes until they run out, then hands over to the
// resource catalogue — [R11]'s reading volume, with a floor under it.
//
// The hand-off is the design. A course that never ends produces a learner who
// can only read material made for them.
package course

import (
	"fmt"
	"strings"

	"tanulom/internal/lang"
)

// Word is one item of a unit's vocabulary.
type Word struct {
	Lemma   string `json:"lemma"`
	UPOS    string `json:"upos"`
	GlossRU string `json:"glossRu"`
	GlossEN string `json:"glossEn"`
	// Note flags what will trip the learner — an irregular stem, a register
	// trap, a false friend. Empty for the ordinary majority.
	Note string `json:"note"`
}

// Unit is one step of the syllabus.
type Unit struct {
	Code        string   `json:"code"`
	Month       int      `json:"month"`
	Ord         int      `json:"ord"`
	TitleRU     string   `json:"titleRu"`
	TitleEN     string   `json:"titleEn"`
	GoalRU      string   `json:"goalRu"`
	GoalEN      string   `json:"goalEn"`
	SituationRU string   `json:"situationRu"`
	SituationEN string   `json:"situationEn"`
	TopicCodes  []string `json:"topicCodes"`
	Words       []Word   `json:"words"`
}

// Syllabus is the loaded course.
type Syllabus struct {
	Units  []Unit
	byCode map[string]Unit
}

// Load reads the syllabus out of a language pack.
//
// A pack without a course is not an error: the starter and full packs both
// carry one, but an older pack predates the table and the app must still run.
// Callers get an empty syllabus and the plan screen says the pack has no course.
func Load(p *lang.Pack) (*Syllabus, error) {
	s := &Syllabus{byCode: map[string]Unit{}}
	if p == nil {
		return s, nil
	}
	rows, err := p.DB().Query(`
		SELECT code, month, ord, title_ru, title_en, goal_ru, goal_en,
		       situation_ru, situation_en, topic_codes
		  FROM course_unit ORDER BY ord`)
	if err != nil {
		// Missing table — an older pack. Not fatal; see the doc comment.
		return s, nil
	}
	defer rows.Close()
	for rows.Next() {
		var u Unit
		var topics string
		if err := rows.Scan(&u.Code, &u.Month, &u.Ord, &u.TitleRU, &u.TitleEN,
			&u.GoalRU, &u.GoalEN, &u.SituationRU, &u.SituationEN, &topics); err != nil {
			return nil, fmt.Errorf("scan course unit: %w", err)
		}
		u.TopicCodes = splitList(topics)
		s.Units = append(s.Units, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	wrows, err := p.DB().Query(`
		SELECT unit_code, lemma, upos, gloss_ru, gloss_en, note
		  FROM course_word ORDER BY unit_code, ord`)
	if err != nil {
		return s, nil
	}
	defer wrows.Close()
	byUnit := map[string][]Word{}
	for wrows.Next() {
		var code string
		var w Word
		if err := wrows.Scan(&code, &w.Lemma, &w.UPOS, &w.GlossRU, &w.GlossEN, &w.Note); err != nil {
			return nil, fmt.Errorf("scan course word: %w", err)
		}
		byUnit[code] = append(byUnit[code], w)
	}
	for i := range s.Units {
		s.Units[i].Words = byUnit[s.Units[i].Code]
		s.byCode[s.Units[i].Code] = s.Units[i]
	}
	return s, wrows.Err()
}

// Unit looks one up.
func (s *Syllabus) Unit(code string) (Unit, bool) {
	u, ok := s.byCode[code]
	return u, ok
}

// Empty reports a pack with no syllabus.
func (s *Syllabus) Empty() bool { return len(s.Units) == 0 }

// Progress is the learner's stored position on one unit.
type Progress struct {
	UnitCode  string `json:"unitCode"`
	Enrolled  bool   `json:"enrolled"`
	NotesMade int    `json:"notesMade"`
	Skipped   int    `json:"skipped"`
}

// Status is where a unit stands for this learner.
type Status string

const (
	// StatusDone: every word is in play in the review deck.
	StatusDone Status = "done"
	// StatusCurrent: the unit the learner should be working on.
	StatusCurrent Status = "current"
	// StatusAhead: later in the syllabus. Not "locked" — nothing stops a
	// learner jumping ahead, and pretending otherwise would be a lie about a
	// restriction the app does not actually enforce. The order is advice.
	StatusAhead Status = "ahead"
)

// UnitState is a unit plus where the learner stands in it.
type UnitState struct {
	Unit        Unit   `json:"unit"`
	Status      Status `json:"status"`
	Enrolled    bool   `json:"enrolled"`
	WordsTotal  int    `json:"wordsTotal"`
	WordsInPlay int    `json:"wordsInPlay"`
	// NotInPlay names the unit's words that are not yet in the review deck.
	//
	// A count alone forces the UI to guess WHICH words those are, and a plan
	// screen that greys out the wrong word is worse than one that greys out
	// nothing. A unit is forty words; naming them is cheap and lets the word
	// list show per-word state as fact rather than inference.
	NotInPlay []string `json:"notInPlay"`
}

// Plan is the whole syllabus resolved for one learner.
type Plan struct {
	Units []UnitState `json:"units"`
	// Current is the unit to work on now; empty once the course is finished.
	Current string `json:"current"`
	// HandedOver is true once every unit is done — the point where the app
	// stops prescribing vocabulary and the resource catalogue takes over.
	HandedOver bool `json:"handedOver"`
	// Completed/Total drive the headline "unit 7 of 24".
	Completed int `json:"completed"`
	Total     int `json:"total"`
}

// Resolve joins the syllabus to what the learner actually has in play.
//
// `inPlay` reports whether a lemma is in the learner's lemma_state as learning
// or known. That — not a button — is what makes a unit done: the criterion is
// a fact about the review deck, so the plan cannot drift from the deck.
func (s *Syllabus) Resolve(progress map[string]Progress, inPlay func(lemma string) bool) Plan {
	plan := Plan{Total: len(s.Units), Units: make([]UnitState, 0, len(s.Units))}
	currentFound := false

	for _, u := range s.Units {
		st := UnitState{Unit: u, WordsTotal: len(u.Words)}
		if p, ok := progress[u.Code]; ok {
			st.Enrolled = p.Enrolled
		}
		st.NotInPlay = []string{}
		for _, w := range u.Words {
			if inPlay(w.Lemma) {
				st.WordsInPlay++
				continue
			}
			st.NotInPlay = append(st.NotInPlay, w.Lemma)
		}
		switch {
		case st.WordsTotal > 0 && st.WordsInPlay >= st.WordsTotal:
			st.Status = StatusDone
			plan.Completed++
		case !currentFound:
			st.Status = StatusCurrent
			plan.Current = u.Code
			currentFound = true
		default:
			st.Status = StatusAhead
		}
		plan.Units = append(plan.Units, st)
	}
	plan.HandedOver = plan.Total > 0 && plan.Completed == plan.Total
	return plan
}

func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
