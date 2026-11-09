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
// So the pack ships a syllabus in three finite tiers (ADR-057): 24 basic units,
// 20 intermediate and 20 advanced, each naming the errand it buys ("I can
// register at the doctor"), the grammar it leans on, and the words to own. The
// learner picks their own pace through them. The app prescribes until the last
// tier runs out, then hands over to the resource catalogue — [R11]'s reading
// volume, with a floor under it.
//
// The hand-off is the design. A course that never ends produces a learner who
// can only read material made for them.
package course

import (
	"database/sql"
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
	Code string `json:"code"`
	// Month is an internal ordering/checkpoint-placement field (1-18). It is no
	// longer shown to learners: they pick their own pace (ADR-057).
	Month       int      `json:"month"`
	Ord         int      `json:"ord"`
	TitleRU     string   `json:"titleRu"`
	TitleEN     string   `json:"titleEn"`
	GoalRU      string   `json:"goalRu"`
	GoalEN      string   `json:"goalEn"`
	SituationRU string   `json:"situationRu"`
	SituationEN string   `json:"situationEn"`
	TopicCodes  []string `json:"topicCodes"`
	// Track is "core" for the guided path (all three tiers) and "theme" for an optional
	// side unit (colours, the body, the weather). Only core units decide
	// Plan.Current and Plan.HandedOver (ADR-050). A pack built before the
	// column existed reads as core.
	Track string `json:"track"`
	// Tier is "basic", "intermediate" or "advanced" (ADR-057). A pack built
	// before the column existed reads as basic; theme units are basic.
	Tier  string `json:"tier"`
	Words []Word `json:"words"`
}

// Tier values.
const (
	TierBasic        = "basic"
	TierIntermediate = "intermediate"
	TierAdvanced     = "advanced"
)

// Tiers lists the tiers in course order.
var Tiers = []string{TierBasic, TierIntermediate, TierAdvanced}

// Track values.
const (
	TrackCore  = "core"
	TrackTheme = "theme"
)

// IsTheme reports an optional side unit. Anything but "theme" is core, so an
// unknown or empty value can never silently drop a unit off the course path.
func (u Unit) IsTheme() bool { return u.Track == TrackTheme }

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
	// `track` and `tier` arrived after the first packs; read only the columns
	// the pack has, so an older pack still loads (every unit core and basic).
	cols := map[string]bool{}
	crows, err := p.DB().Query(`PRAGMA table_info(course_unit)`)
	if err != nil {
		return s, nil
	}
	for crows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := crows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			crows.Close()
			return nil, fmt.Errorf("course_unit columns: %w", err)
		}
		cols[name] = true
	}
	crows.Close()
	if len(cols) == 0 {
		// Missing table — an older pack. Not fatal; see the doc comment.
		return s, nil
	}
	query := `SELECT code, month, ord, title_ru, title_en, goal_ru, goal_en,
		       situation_ru, situation_en, topic_codes`
	hasTrack, hasTier := cols["track"], cols["tier"]
	if hasTrack {
		query += `, track`
	}
	if hasTier {
		query += `, tier`
	}
	rows, err := p.DB().Query(query + ` FROM course_unit ORDER BY ord`)
	if err != nil {
		return s, nil
	}
	defer rows.Close()
	for rows.Next() {
		var u Unit
		var topics string
		dest := []any{&u.Code, &u.Month, &u.Ord, &u.TitleRU, &u.TitleEN,
			&u.GoalRU, &u.GoalEN, &u.SituationRU, &u.SituationEN, &topics}
		if hasTrack {
			dest = append(dest, &u.Track)
		}
		if hasTier {
			dest = append(dest, &u.Tier)
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("scan course unit: %w", err)
		}
		if u.Track == "" {
			u.Track = TrackCore
		}
		if u.Tier == "" {
			u.Tier = TierBasic
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

// UnlockShare is the soft gate (ADR-058): the unit after one with fewer than
// this share of its words SETTLED is drawn as closed. Advice, not a lock
// (ADR-013) — opening it anyway is always one tap and no endpoint refuses.
const UnlockShare = 0.70

// Unlock is the gate's rule as sent on GET /api/course, so the client never
// hard-codes it. SettledDays is srs.MatureDays (the interval at which a card
// counts as settled); it is set by the API layer, which owns the srs import.
type Unlock struct {
	Share       float64 `json:"share"`
	SettledDays int     `json:"settledDays"`
}

// Status is where a unit stands for this learner.
type Status string

const (
	// StatusDone: every word is in play in the review deck.
	StatusDone Status = "done"
	// StatusCurrent: the unit to take next (ADR-048: follows enrolment).
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
	// Learned is how many of the unit's words are COUNTED lemmas — the same
	// rule /api/stats/level uses (known, or a graduated card whose
	// retrievability is still at the mining threshold). WordsInPlay says a word
	// is in the deck; Learned says it has been learned. A unit enrolled a minute
	// ago has all its words in play and none learned (ADR-057). Derived, never
	// stored; 0 when the caller supplies no counted function.
	Learned int `json:"learned"`
	// Settled is how many of the unit's words have been remembered long enough to
	// go to long-interval repetition: lemma `known`, or the strongest
	// non-suspended, non-trashed card is mature (interval >= srs.MatureDays).
	// A subset of WordsInPlay, derived, never stored (ADR-058). 0 when the
	// caller supplies no settled function.
	Settled int `json:"settled"`
	// NotInPlay names the unit's words that are not yet in the review deck.
	//
	// A count alone forces the UI to guess WHICH words those are, and a plan
	// screen that greys out the wrong word is worse than one that greys out
	// nothing. A unit is forty words; naming them is cheap and lets the word
	// list show per-word state as fact rather than inference.
	NotInPlay []string `json:"notInPlay"`
	// FromHere is the subset of WordsInPlay whose notes this unit created.
	//
	// WordsInPlay counts a lemma wherever it came from — and that is correct,
	// because a word in the deck is in the deck. But it made the plan screen
	// LIE: a word added from the reader raised month 2's fraction, and the
	// fraction was the only progress a row showed, so the screen reported a
	// month the learner had never started. The two numbers apart let the row
	// say the true thing instead: "12 of 40, and 9 of those you already met
	// while reading".
	//
	// Derived, never stored. Unit completion stays a fact about the deck.
	FromHere int `json:"fromHere"`
}

// Plan is the whole syllabus resolved for one learner.
type Plan struct {
	Units []UnitState `json:"units"`
	// Current is the unit to take NEXT; empty once the course is finished. The
	// JSON name stays `current` for compatibility; the learner-facing word is
	// «следующий» (ADR-048). It follows ENROLMENT, an act of the learner's:
	// the first unit enrolled and not done, else the first neither enrolled
	// nor done. Words added from the reader can complete a later unit (done is
	// derived from the deck) but can never move this away from a unit the
	// learner started.
	Current string `json:"current"`
	// HandedOver is true once every unit is done — the point where the app
	// stops prescribing vocabulary and the resource catalogue takes over.
	HandedOver bool `json:"handedOver"`
	// Completed/Total drive the headline "unit 7 of 64". CORE units only, across
	// all three tiers: the themed side units are optional and finite, and
	// counting them would move the total and make a finished course look
	// unfinished (ADR-050, ADR-057).
	Completed int `json:"completed"`
	Total     int `json:"total"`
}

// Resolve joins the syllabus to what the learner actually has in play.
//
// `inPlay` reports whether a lemma is in the learner's lemma_state as learning
// or known. That — not a button — is what makes a unit done: the criterion is
// a fact about the review deck, so the plan cannot drift from the deck.
func (s *Syllabus) Resolve(progress map[string]Progress, inPlay func(lemma string) bool,
	fromUnit func(unitCode, lemma string) bool) Plan {
	return s.ResolveCounted(progress, inPlay, fromUnit, nil, nil)
}

// ResolveCounted is Resolve plus the per-unit `learned` figure: `counted`
// reports whether a lemma is a counted (learned) one. A counted word that is
// not in play does not count — Learned is a subset of WordsInPlay.
func (s *Syllabus) ResolveCounted(progress map[string]Progress, inPlay func(lemma string) bool,
	fromUnit func(unitCode, lemma string) bool, counted, settled func(lemma string) bool) Plan {
	plan := Plan{Units: make([]UnitState, 0, len(s.Units))}
	for _, u := range s.Units {
		if !u.IsTheme() {
			plan.Total++
		}
	}

	for _, u := range s.Units {
		st := UnitState{Unit: u, WordsTotal: len(u.Words)}
		if p, ok := progress[u.Code]; ok {
			st.Enrolled = p.Enrolled
		}
		st.NotInPlay = []string{}
		for _, w := range u.Words {
			if inPlay(w.Lemma) {
				st.WordsInPlay++
				if counted != nil && counted(w.Lemma) {
					st.Learned++
				}
				if settled != nil && settled(w.Lemma) {
					st.Settled++
				}
				if fromUnit != nil && fromUnit(u.Code, w.Lemma) {
					st.FromHere++
				}
				continue
			}
			st.NotInPlay = append(st.NotInPlay, w.Lemma)
		}
		if st.WordsTotal > 0 && st.WordsInPlay >= st.WordsTotal {
			st.Status = StatusDone
			if !u.IsTheme() {
				plan.Completed++
			}
		} else {
			st.Status = StatusAhead
		}
		plan.Units = append(plan.Units, st)
	}

	// Next: an enrolled, unfinished unit first (the learner started it), and
	// only then the first untouched one. See Plan.Current and ADR-048.
	next := -1
	// Theme units are skipped in both passes: a side branch is displayed with
	// its own done/ahead state but never becomes "next" and never gates the
	// hand-over (ADR-050). Derived completion is unchanged for them.
	for i, st := range plan.Units {
		if st.Status != StatusDone && st.Enrolled && !st.Unit.IsTheme() {
			next = i
			break
		}
	}
	if next < 0 {
		for i, st := range plan.Units {
			if st.Status != StatusDone && !st.Enrolled && !st.Unit.IsTheme() {
				next = i
				break
			}
		}
	}
	if next >= 0 {
		plan.Units[next].Status = StatusCurrent
		plan.Current = plan.Units[next].Unit.Code
	}
	plan.HandedOver = plan.Total > 0 && plan.Completed == plan.Total
	return plan
}

// TierTotals is the word tally for one tier (or the whole course).
type TierTotals struct {
	Words   int `json:"words"`
	InDeck  int `json:"inDeck"`
	Learned int `json:"learned"`
	Settled int `json:"settled"`
}

// Totals is the course-wide tally of CORE words, in total and per tier. Theme
// units are excluded for the same reason they are excluded from Plan.Total.
// A word taught by one unit counts once (the pack guarantees a word is in one
// unit). Everything here is derived from the deck at request time.
type Totals struct {
	TierTotals
	PerTier map[string]TierTotals `json:"perTier"`
}

// Totals sums the plan's core units. PerTier always carries all three tiers.
func (p Plan) Totals() Totals {
	t := Totals{PerTier: map[string]TierTotals{}}
	for _, tier := range Tiers {
		t.PerTier[tier] = TierTotals{}
	}
	for _, st := range p.Units {
		if st.Unit.IsTheme() {
			continue
		}
		tier := st.Unit.Tier
		if _, ok := t.PerTier[tier]; !ok {
			tier = TierBasic
		}
		pt := t.PerTier[tier]
		pt.Words += st.WordsTotal
		pt.InDeck += st.WordsInPlay
		pt.Learned += st.Learned
		pt.Settled += st.Settled
		t.PerTier[tier] = pt
		t.Words += st.WordsTotal
		t.InDeck += st.WordsInPlay
		t.Learned += st.Learned
		t.Settled += st.Settled
	}
	return t
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
