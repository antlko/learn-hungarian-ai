package srs

import (
	"database/sql"
	"fmt"
	"hash/fnv"
	"sort"
	"time"

	"tanulom/internal/db"
)

// QueueItem is one card ready to be shown, with everything the UI needs.
type QueueItem struct {
	CardID    int64     `json:"cardId"`
	NoteID    int64     `json:"noteId"`
	Direction Direction `json:"direction"`
	IsNew     bool      `json:"isNew"`
	// State is the FSRS state, for queue policy (the maxReviewsPerDay trim caps
	// graduated cards only). Not the client's business.
	State int       `json:"-"`
	Due   time.Time `json:"due"`
	// fsrs is the whole stored state, carried so AttachMemory can compute the
	// card's memory figures without a second read per card. Never on the wire;
	// what the client gets is Memory.
	fsrs CardState
	// Memory is what the card knows about itself — chance of recall, stage,
	// history, and what each grade would schedule. Filled by AttachMemory,
	// after the queue is built. ADR-026.
	Memory     *Memory `json:"memory,omitempty"`
	HU         string  `json:"hu"`
	TargetForm string  `json:"targetForm"`
	// TargetStart/TargetEnd are BYTE offsets into HU and never cross the wire.
	// A JavaScript client indexes strings in UTF-16 code units, so slicing HU
	// with them put the blank in the middle of a word on every sentence
	// containing an accented letter before the target — `Jó éjszakát, Timmy!`
	// rendered as `Jó é_____immy!`. Target below is the split, done here.
	TargetStart int          `json:"-"`
	TargetEnd   int          `json:"-"`
	Target      *TargetSplit `json:"target,omitempty"`
	TargetFeats []string     `json:"targetFeats"`
	GlossRU     string       `json:"glossRu"`
	GlossEN     string       `json:"glossEn"`
	// SentenceRU/EN translate the WHOLE sentence, where the pack has one.
	//
	// Distinct from GlossRU/EN, which gloss the TARGET WORD, and confusing them
	// is what made the review card unreadable: `Jó hírt hoztunk.` was shown
	// under the single word «хороший; добрый», the gloss of `jó`, with nothing
	// saying which of the four words it belonged to or what the sentence meant.
	// They are filled by the API layer, not here: the translations live in the
	// language pack and this store only holds the learner's database.
	SentenceRU string `json:"sentenceRu,omitempty"`
	SentenceEN string `json:"sentenceEn,omitempty"`
	// SentenceModel names the MACHINE that wrote SentenceRU/EN, and is empty
	// for a pack translation — those were written by people and there is no
	// model to name. A note mined from the learner's own reading has no pack
	// sentence to borrow from, so its crib comes from the MT sidecar, and
	// ADR-014 makes the label the entire defence: a wrong translation is
	// well-formed, so it has to say who wrote it. The client prints this beside
	// the text, never in a tooltip. See ADR-018.
	SentenceModel string `json:"sentenceModel,omitempty"`
	// SentenceID identifies the pack sentence this note was built from, for
	// that lookup. Never on the wire — it is a pack row id, not the client's
	// business.
	SentenceID *int64 `json:"-"`
	LemmaID    *int64 `json:"lemmaId,omitempty"`
	// Lemma is the dictionary form of the target word, filled by the API layer
	// from the pack. It is the CUE a cloze cannot be answered without: `Jó
	// ____, Timmy!` is reggelt, napot, estét or éjszakát, and a learner given
	// no word to inflect is guessing vocabulary on an exercise that grades
	// morphology. See Direction.GradesFeatures.
	Lemma string `json:"lemma,omitempty"`
	// Expected is the string this card is graded against, filled by the API
	// layer through the same rule the answer path uses.
	//
	// It is on the wire so «Подсказка» can show the answer WITHOUT grading the
	// card: revealing is the whole point of a hint, and on a productive card
	// there was nothing to reveal — the client had no way to know that produce
	// wants the dictionary form while cloze wants the form in the sentence. No
	// secret is spent doing it. TargetForm and Lemma are both already here, so
	// the answer was always derivable client-side; what was missing was WHICH
	// of the two this direction wants, and guessing that in the client is how
	// the hint would come to disagree with the grader.
	Expected string `json:"expected,omitempty"`
}

// TargetSplit is the sentence either side of the target span, so the client can
// blank it or highlight it without ever having to locate it.
type TargetSplit struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

// splitCloze locates the blank. Nil when the note has no usable target span:
// an unlocated target is stored as -1, and a stale offset must degrade to "no
// blank" rather than to a sliced-apart sentence.
func splitTarget(hu string, start, end int) *TargetSplit {
	if start < 0 || end <= start || end > len(hu) {
		return nil
	}
	return &TargetSplit{Before: hu[:start], After: hu[end:]}
}

// Order is how the served queue is arranged. ADR-026.
//
// OrderDue is oldest-first: a card overdue by a month is decaying faster than
// one due today and belongs at the front. OrderRandom is the learner's
// explicit, printed choice — never a hidden agenda — and it is stable for the
// day: cards are ordered by a hash of (user, local day, card id), so a reload
// after a maturing learning step does not reshuffle the remainder under the
// learner's hands. Which cards are served never changes with the order.
type Order string

const (
	OrderDue    Order = "due"
	OrderRandom Order = "random"
)

// ParseOrder maps a query parameter to an Order; blank is OrderDue.
func ParseOrder(s string) (Order, bool) {
	switch Order(s) {
	case "":
		return OrderDue, true
	case OrderDue, OrderRandom:
		return Order(s), true
	}
	return "", false
}

// QueueCounts summarises the session that was just built, in the Breakdown
// vocabulary (ADR-021): every figure is post-burial and day-granular, so the
// number on screen is the number of things that will actually be shown.
//
// **Cards and notes are counted separately, on purpose.** A note is a word to
// learn; a card is one direction of it, and a note spawns three to five. The
// screen that said "59 new cards waiting" to a learner who had taken up 27
// words was accurate and useless — they had no way to turn one number into the
// other, and 59 sounds like a backlog where 27 sounds like a unit of the
// course. The new-material figures below therefore lead with words.
type QueueCounts struct {
	// Due is the served scope's Breakdown.DueNow: what this queue serves before
	// any maxReviewsPerDay trim. New is the scope's raw unseen CARD pool.
	Due   int `json:"due"`
	New   int `json:"new"`
	Total int `json:"total"` // items actually served
	// The fate of every new word this build considered, on one basis:
	// shown now · deferred because a sibling was already due or answered today
	// · held back by the daily budget alone. The old pair (`newCapped` /
	// `newNotesCapped`) mixed a scoped count with the global budget and blamed
	// the limit for sibling deferrals; these three cannot.
	NewShownNotes    int `json:"newShownNotes"`
	NewDeferredNotes int `json:"newDeferredNotes"`
	NewBacklogNotes  int `json:"newBacklogNotes"`
	// The global daily budget (ADR-016): started today, the configured cap
	// (0 = unlimited), and what was left of it when this build ran.
	IntroducedToday int `json:"introducedToday"`
	NewPerDay       int `json:"newPerDay"`
	NewBudgetLeft   int `json:"newBudgetLeft"`
	// Learning steps maturing before local midnight, and the earliest of them —
	// what the done screen counts down to instead of claiming the day is over.
	DueLaterToday int        `json:"dueLaterToday"`
	NextDueAt     *time.Time `json:"nextDueAt,omitempty"`
	// ReviewsCapped is how many due items maxReviewsPerDay trimmed from THIS
	// build, after the day's earlier answers were subtracted.
	ReviewsCapped int `json:"reviewsCapped,omitempty"`
}

// Queue assembles today's review queue.
//
// Two rules that matter more than they look:
//
//   - **New cards are capped and reviews are not.** Reviews are a debt already
//     incurred; refusing to show them just defers the problem. New cards are the
//     tap, and the cap is the only thing standing between a learner and the
//     review avalanche that is the most common reason people quit.
//   - **Siblings are buried.** Showing `recognize` and `produce` of the same note
//     in one session turns the second into a copying exercise; the answer is
//     still on screen in the learner's memory, so it tests nothing.
func (s *Store) Queue(userID int64, scope Scope, newLimit, maxReviews int, burySiblings bool, now time.Time) ([]QueueItem, QueueCounts, error) {
	return s.QueueOrdered(userID, scope, newLimit, maxReviews, burySiblings, OrderDue, now)
}

// QueueOrdered is Queue with the learner's chosen Order applied. The order
// changes only the sequence: selection, burial, the day cap and the new-card
// budget all run exactly as for OrderDue, and spreadSentences still runs last.
func (s *Store) QueueOrdered(userID int64, scope Scope, newLimit, maxReviews int, burySiblings bool, order Order, now time.Time) ([]QueueItem, QueueCounts, error) {
	nowStr, endStr := dayWindow(now)
	var counts QueueCounts

	// The serving predicate, in the Breakdown vocabulary: a graduated card
	// (state 2) is due for the whole local day — «к повторению» must not creep
	// upward through the afternoon as graduation hours pass — while learning
	// and relearning steps (states 1, 3) are minutes by design and appear only
	// once elapsed. Oldest first: a card overdue by a month is decaying faster
	// than one due today.
	items, err := s.selectQueue(userID, scope, `
		AND ((c.state = 2 AND c.due < ?) OR (c.state IN (1,3) AND c.due <= ?))
		ORDER BY c.due ASC LIMIT 10000`, endStr, nowStr)
	if err != nil {
		return nil, QueueCounts{}, err
	}

	introducedToday, err := s.newCardsIntroducedToday(userID, now)
	if err != nil {
		return nil, QueueCounts{}, err
	}
	counts.IntroducedToday, counts.NewPerDay = introducedToday, newLimit

	// Burial spans the DAY, not the page load. Without this, answering a word's
	// cloze and reloading served the same word's produce card a second later:
	// the session never shrank, and the app looked like it had forgotten the
	// answer. See buryAnsweredSiblings.
	var answered map[int64]map[int64]bool
	if burySiblings {
		if answered, err = s.answeredToday(userID, now); err != nil {
			return nil, QueueCounts{}, err
		}
		items = buryAnsweredSiblings(items, answered)
		items = burySiblingCards(items)
	}

	// The reported numbers come from Breakdowns — the one coordinate system
	// every screen shares (ADR-021). queue_day_test.go pins the served list to
	// Breakdown.DueNow, so the tab strip, Today and the session cannot disagree
	// the way the old pre-burial counts did.
	var extras []Scope
	if scope != ScopeAll && scope != ScopeCourse && scope != ScopeReader {
		extras = append(extras, scope)
	}
	bds, err := s.Breakdowns(userID, burySiblings, now, extras...)
	if err != nil {
		return nil, QueueCounts{}, err
	}
	bd := breakdownFor(bds, scope)
	counts.Due = bd.DueNow
	counts.New = bd.NewCards
	counts.DueLaterToday = bd.DueLaterToday
	counts.NextDueAt = bd.NextDueAt

	// maxReviewsPerDay is a DAY cap, not a fetch cap: the day's earlier
	// review-state answers are subtracted, and the trim runs after burial so it
	// caps what actually reaches the screen. Learning and relearning steps are
	// never trimmed — a step already in flight must be allowed to finish, or
	// the cap turns minutes-scale scheduling into tomorrow.
	if maxReviews > 0 {
		reviewsToday, err := s.reviewStateAnswersToday(userID, now)
		if err != nil {
			return nil, QueueCounts{}, err
		}
		items, counts.ReviewsCapped = capReviewItems(items, max(0, maxReviews-reviewsToday))
	}

	// Order last, once the list is final: the trim above drops from the
	// latest-due end and needs the oldest-first order to do it. A random
	// order is applied here, after the cap, for the same reason.
	if order == OrderRandom {
		shuffleDayStable(items, userID, now)
	}
	items = spreadSentences(items)

	// New cards, after the due ones and under today's limit.
	//
	// **Buried first, then trimmed to the limit.** It used to be the other way
	// round: take `remaining` cards, then collapse siblings — which threw away
	// most of them, because `ORDER BY note_id` puts a note's three directions
	// next to each other. A limit of 20 selected 20 cards, buried them down to
	// seven notes, and showed seven. The learner met a fraction of the new
	// material their setting promised and nothing said why. The limit is a
	// budget for what actually reaches the screen.
	remaining := counts.New
	if newLimit > 0 {
		remaining = max(0, newLimit-introducedToday)
	}
	counts.NewBudgetLeft = remaining
	if remaining > 0 {
		// Bounded: enough rows to survive sibling collapse (≤5 cards per word),
		// the answered-note deferral and the due-list dedup, instead of
		// materialising every unseen card on every fetch.
		selLimit := 10_000
		if newLimit > 0 {
			if need := (remaining + len(items) + len(answered)) * 5; need < selLimit {
				selLimit = need
			}
		}
		fresh, err := s.selectQueue(userID, scope, `
			AND c.state = 0
			ORDER BY c.note_id ASC, c.id ASC LIMIT ?`, selLimit)
		if err != nil {
			return nil, QueueCounts{}, err
		}
		deferred := map[int64]bool{}
		if burySiblings {
			// A word answered today, or already in the due list, does not get a
			// second direction in the same session — that second card is a
			// copying exercise. These are DEFERRALS, not budget: the old screen
			// folded them into "the daily limit is reached" and sent people to
			// Settings over a limit that was never touched.
			seen := map[int64]bool{}
			for _, it := range items {
				seen[it.NoteID] = true
			}
			kept := fresh[:0]
			for _, it := range fresh {
				if _, wasAnswered := answered[it.NoteID]; wasAnswered || seen[it.NoteID] {
					deferred[it.NoteID] = true
					continue
				}
				kept = append(kept, it)
			}
			fresh = kept
			fresh = burySiblingCards(fresh)
		}
		if len(fresh) > remaining {
			fresh = fresh[:remaining]
		}
		// After the trim, never before it: `ORDER BY c.note_id` is the order the
		// words were taken up, and for a course unit that is curriculum order.
		// Which words get introduced today is not this function's business to
		// change — only the order they are asked in.
		if order == OrderRandom {
			shuffleDayStable(fresh, userID, now)
		}
		fresh = spreadSentences(fresh)
		items = append(items, fresh...)

		shownNotes := map[int64]bool{}
		for _, it := range fresh {
			shownNotes[it.NoteID] = true
		}
		counts.NewShownNotes = len(shownNotes)
		counts.NewDeferredNotes = len(deferred)
	}
	counts.NewBacklogNotes = max(0, bd.NewNotes-counts.NewShownNotes-counts.NewDeferredNotes)
	counts.Total = len(items)
	return items, counts, nil
}

// reviewStateAnswersToday counts today's answers on graduated (state 2) cards —
// the quantity maxReviewsPerDay budgets. review_log.state is the state BEFORE
// the answer, so this is one indexed count, and it is deliberately unscoped:
// the cap is a workload ceiling for the day, like the new-card budget.
func (s *Store) reviewStateAnswersToday(userID int64, now time.Time) (int, error) {
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var n int
	err := s.db.QueryRow(`
		SELECT COUNT(*)
		  FROM review_log r
		  JOIN card c ON c.id = r.card_id
		  JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND r.state = 2 AND r.review_time >= ?`,
		userID, db.FmtTime(midnight)).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count today's reviews: %w", err)
	}
	return n, nil
}

// capReviewItems trims graduated (state 2) items beyond the day's remaining
// allowance, dropping from the latest-due end; learning and relearning steps
// pass through untouched.
func capReviewItems(items []QueueItem, allowed int) ([]QueueItem, int) {
	reviews := 0
	for _, it := range items {
		if it.State == 2 {
			reviews++
		}
	}
	if reviews <= allowed {
		return items, 0
	}
	drop := reviews - allowed
	dropped := 0
	out := make([]QueueItem, 0, len(items)-drop)
	for i := len(items) - 1; i >= 0; i-- {
		if dropped < drop && items[i].State == 2 {
			dropped++
			continue
		}
		out = append(out, items[i])
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, drop
}

func (s *Store) selectQueue(userID int64, scope Scope, where string, args ...any) ([]QueueItem, error) {
	q := `
		SELECT c.id, c.note_id, c.direction, c.state, c.due,
		       n.hu, n.target_form, n.target_start, n.target_end, n.target_feats,
		       n.gloss_ru, n.gloss_en, n.lemma_id, n.sentence_id,
		       c.stability, c.difficulty, c.scheduled_days, c.reps, c.lapses,
		       c.last_review, c.remaining_steps
		  FROM card c JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND c.suspended = 0 ` + scope.filter() + where

	rows, err := s.db.Query(q, append([]any{userID}, args...)...)
	if err != nil {
		return nil, fmt.Errorf("build queue: %w", err)
	}
	defer rows.Close()

	var out []QueueItem
	for rows.Next() {
		var it QueueItem
		var due, feats string
		var last sql.NullString
		if err := rows.Scan(&it.CardID, &it.NoteID, &it.Direction, &it.State, &due,
			&it.HU, &it.TargetForm, &it.TargetStart, &it.TargetEnd, &feats,
			&it.GlossRU, &it.GlossEN, &it.LemmaID, &it.SentenceID,
			&it.fsrs.Stability, &it.fsrs.Difficulty, &it.fsrs.ScheduledDays,
			&it.fsrs.Reps, &it.fsrs.Lapses, &last, &it.fsrs.RemainingSteps); err != nil {
			return nil, fmt.Errorf("scan queue item: %w", err)
		}
		it.IsNew = it.State == 0
		it.Due = db.ParseTime(sql.NullString{String: due, Valid: true})
		it.fsrs.State = it.State
		scanCardState(&it.fsrs, due, last)
		it.TargetFeats = splitFeats(feats)
		it.Target = splitTarget(it.HU, it.TargetStart, it.TargetEnd)
		// A list, empty or not: `null` on the wire is what a client's `.length` trips on.
		if it.TargetFeats == nil {
			it.TargetFeats = []string{}
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// newCardsIntroducedToday counts first reviews since local midnight, so the
// daily cap means a day rather than a rolling 24 hours.
func (s *Store) newCardsIntroducedToday(userID int64, now time.Time) (int, error) {
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var n int
	err := s.db.QueryRow(`
		SELECT COUNT(DISTINCT r.card_id)
		  FROM review_log r
		  JOIN card c ON c.id = r.card_id
		  JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND r.state = 0 AND r.review_time >= ?`,
		userID, db.FmtTime(midnight)).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count new cards today: %w", err)
	}
	return n, nil
}

// spreadSentences deals the queue out so that two cards blanking the SAME
// sentence never land back to back.
//
// Ten words mined from one paragraph become ten notes over three or four
// sentences, created in one batch — so `ORDER BY c.note_id` hands them out in
// source order, and `ORDER BY c.due` does the same thing a day later, because a
// batch introduced together comes back together. The session then reads as the
// same sentence eight times with the blank sliding along it, and by the third
// card the learner is not recalling anything: the sentence is on screen, the
// answer is a word they read in it a moment ago, and the only thing being
// tested is whether they can still see. It also makes a whole session feel like
// one item, which is the reverse of the interleaving the app argues for
// everywhere else [R5].
//
// The rule, which is the whole contract:
//
//   - **A queue in which no sentence repeats comes back UNCHANGED.** That is
//     the ordinary case — a course session is one sentence per card — and it is
//     what lets this run over the due list without touching its oldest-first
//     order. Nothing moves unless something repeats.
//   - When a sentence does repeat, the repeats are what dictate the order: take
//     from whichever sentence has the most cards left, never the one just
//     served. That is the classic arrangement, and it separates them completely
//     whenever separating them is possible at all.
//   - Ties break on first appearance, which is what makes the first rule fall
//     out of the second and keeps the result deterministic. A random shuffle
//     would reorder the queue on every reload, and a session topped up by a
//     maturing learning step would look as though it had reshuffled itself
//     under the learner's hands.
//
// Best effort, and it says so rather than pretending: a queue that is nothing
// but one sentence cannot be separated, and it comes back whole and unspread.
func spreadSentences(items []QueueItem) []QueueItem {
	if len(items) < 3 {
		return items
	}
	// Group by sentence, keeping first-appearance order — the tie-break that
	// makes a queue without repeats come back untouched.
	index := map[string]int{}
	var groups [][]QueueItem
	for _, it := range items {
		i, ok := index[it.HU]
		if !ok {
			i = len(groups)
			index[it.HU] = i
			groups = append(groups, nil)
		}
		groups[i] = append(groups[i], it)
	}
	if len(groups) == len(items) {
		return items
	}

	out := make([]QueueItem, 0, len(items))
	prev := -1
	for len(out) < len(items) {
		pick := -1
		for i := range groups {
			if len(groups[i]) == 0 || i == prev {
				continue
			}
			if pick == -1 || len(groups[i]) > len(groups[pick]) {
				pick = i
			}
		}
		if pick == -1 {
			// Only the sentence just served has cards left: they cannot be
			// separated, so they go out as they are rather than being dropped.
			out = append(out, groups[prev]...)
			break
		}
		out = append(out, groups[pick][0])
		groups[pick] = groups[pick][1:]
		prev = pick
	}
	return out
}

// shuffleDayStable orders items by a hash of (user, local day, card id).
//
// A seeded Fisher–Yates over the CURRENT list would not do: the list shrinks
// as the learner answers, and a reload after a maturing learning step would
// permute the remainder differently — the queue would visibly reshuffle
// itself. A per-card key gives every card a fixed place for the day, whatever
// else is in the list.
func shuffleDayStable(items []QueueItem, userID int64, now time.Time) {
	day := now.Year()*10000 + int(now.Month())*100 + now.Day()
	key := func(cardID int64) uint64 {
		h := fnv.New64a()
		fmt.Fprintf(h, "%d|%d|%d", userID, day, cardID)
		return h.Sum64()
	}
	sort.SliceStable(items, func(i, j int) bool {
		return key(items[i].CardID) < key(items[j].CardID)
	})
}

// burySiblingCards keeps at most one card per note in a session.
func burySiblingCards(items []QueueItem) []QueueItem {
	seen := map[int64]bool{}
	out := make([]QueueItem, 0, len(items))
	for _, it := range items {
		if seen[it.NoteID] {
			continue
		}
		seen[it.NoteID] = true
		out = append(out, it)
	}
	return out
}

func splitFeats(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '|' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}

// nullInt scans a possibly-NULL SUM into an int.
type nullInt struct{ dst *int }

func (n *nullInt) Scan(v any) error {
	switch t := v.(type) {
	case nil:
		*n.dst = 0
	case int64:
		*n.dst = int(t)
	case float64:
		*n.dst = int(t)
	default:
		return fmt.Errorf("cannot scan %T into int", v)
	}
	return nil
}

// answeredToday maps each note the learner has reviewed today to the cards of
// that note they actually answered.
//
// Burial needs both halves. A note that was answered today should not offer a
// DIFFERENT direction of the same word again — but the card that was answered
// may legitimately come back on its own learning step, ten minutes later, and
// that is the scheduler doing its job rather than a sibling sneaking in.
func (s *Store) answeredToday(userID int64, now time.Time) (map[int64]map[int64]bool, error) {
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	rows, err := s.db.Query(`
		SELECT DISTINCT c.note_id, c.id
		  FROM review_log r
		  JOIN card c ON c.id = r.card_id
		  JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND r.review_time >= ?`,
		userID, db.FmtTime(midnight))
	if err != nil {
		return nil, fmt.Errorf("read today's answers: %w", err)
	}
	defer rows.Close()

	out := map[int64]map[int64]bool{}
	for rows.Next() {
		var noteID, cardID int64
		if err := rows.Scan(&noteID, &cardID); err != nil {
			return nil, err
		}
		if out[noteID] == nil {
			out[noteID] = map[int64]bool{}
		}
		out[noteID][cardID] = true
	}
	return out, rows.Err()
}

// buryAnsweredSiblings drops cards whose word was already answered today by a
// DIFFERENT direction.
//
// In-session burial was only ever half the rule. It keeps one card per note
// within a single queue, and a page reload builds a new one — so answering the
// cloze for `házban` and reloading served the produce card for the same note a
// second later, with the answer still on screen in the learner's memory. It
// tests nothing, and it reads as the app having forgotten the answer: the same
// word comes back and the session never shrinks.
//
// The card that WAS answered is deliberately kept. When it reappears it is due
// on its own learning step, which is short-term scheduling working, not a
// sibling.
func buryAnsweredSiblings(items []QueueItem, answered map[int64]map[int64]bool) []QueueItem {
	if len(answered) == 0 {
		return items
	}
	out := make([]QueueItem, 0, len(items))
	for _, it := range items {
		if cards, ok := answered[it.NoteID]; ok && !cards[it.CardID] {
			continue
		}
		out = append(out, it)
	}
	return out
}
