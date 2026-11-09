package api

import (
	"database/sql"
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/config"
	"tanulom/internal/morph"
	"tanulom/internal/srs"
)

// Review endpoints.
//
// The shape to notice: one answer returns not only the card's new schedule but
// the list of morphological features it graded. The UI renders those as chips,
// which is what makes the second state space visible rather than mysterious —
// a learner can see that answering this cloze credited their inessive.

// engine builds a review engine at the learner's current settings.
func (s *Server) engine() *srs.Engine {
	cfg := s.Config()
	return srs.NewEngine(srs.NewStore(s.db.DB), cfg.Review.DesiredRetention, cfg.Drill.GateWindow)
}

// QueueResponse is today's queue.
type QueueResponse struct {
	Items  []srs.QueueItem `json:"items"`
	Counts srs.QueueCounts `json:"counts"`
	// Scope is what was actually served. Echoed rather than assumed: a client
	// that guessed its own request won would mislabel the whole session after
	// a rejected or defaulted parameter.
	Scope srs.Scope `json:"scope"`
	// Scopes carries EVERY pile's Breakdown, whichever one was asked for,
	// because the segmented control has to label the tab the learner is not
	// standing on. Post-burial, day-granular numbers — ADR-021. A text scope,
	// when one was served, is appended after the three fixed piles.
	Scopes []srs.Breakdown `json:"scopes"`
	// Order echoes the sequence actually applied, for the same reason Scope is
	// echoed: the screen prints the criterion it is using (ADR-026).
	Order srs.Order `json:"order"`
}

func (s *Server) handleQueue(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	scope, ok := srs.ParseScope(c.Query("scope"))
	if !ok {
		return badRequest("scope must be all, course, reader or text:<id>")
	}
	// The trash is practiseable, never schedulable (ADR-025). Refused here
	// rather than in ParseScope, which practice shares: a scope that built a
	// review queue would make the trash a pile cards come back FROM, and
	// answering out of it would move FSRS state on words the learner asked to
	// stop seeing. `/api/practice/queue?scope=trash` is the way in.
	if scope == srs.ScopeTrash {
		return badRequest("the trash cannot be reviewed; practise it or restore it")
	}
	// The order is the learner's explicit, printed choice: oldest-first by
	// default, or a day-stable random. It never changes which cards are
	// served — see srs.Order.
	order, ok := srs.ParseOrder(c.Query("order"))
	if !ok {
		return badRequest("order must be due or random")
	}
	cfg := s.Config()
	store := srs.NewStore(s.db.DB)
	now := time.Now()

	// Repair notes that predate the current cloze rule before the queue is
	// built, so a card the learner should already have had appears in the very
	// session that noticed it was missing.
	if err := s.backfillClozeCards(u.ID); err != nil {
		return err
	}

	// The daily new-card cap is GLOBAL, not per scope: `newLimit` and
	// `introducedToday` count the whole deck however the queue is filtered.
	// Scoping the cap would let a learner take 20 course cards and 20 reader
	// cards while the setting says 20, which is exactly the review avalanche
	// the cap exists to prevent. Scope chooses which new cards are ELIGIBLE;
	// it never enlarges the budget. See ADR-016.
	items, counts, err := store.QueueOrdered(u.ID, scope, cfg.Review.NewPerDay,
		cfg.Review.MaxReviewsPerDay, cfg.Review.BurySiblingsEnabled(), order, now)
	if err != nil {
		return err
	}
	if items == nil {
		items = []srs.QueueItem{}
	}
	if err := s.enrichFromPack(items); err != nil {
		return err
	}
	// What each card knows about itself — chance of recall, stage, history,
	// and what each grade would schedule. With the preview: this IS the
	// scheduler's queue, and the interval hints under the grades are its own
	// figures. ADR-026.
	if err := store.AttachMemory(items, s.engine().Scheduler(), now, true); err != nil {
		return err
	}
	// A sibling of enrichFromPack, never nested inside it: cribs live in the
	// LEARNER's database, and enrichFromPack returns early when no pack is
	// installed.
	if err := s.attachCribs(u.ID, items); err != nil {
		return err
	}
	s.attachOptions(items, now)
	// A text scope's numbers ride along after the three fixed piles, so the
	// segmented control keeps its arithmetic and the text chip gets its own.
	var extras []srs.Scope
	if scope.IsText() {
		extras = append(extras, scope)
	}
	scopes, err := store.Breakdowns(u.ID, cfg.Review.BurySiblingsEnabled(), now, extras...)
	if err != nil {
		return err
	}
	s.warmCribs(u.ID, items)
	return c.JSON(QueueResponse{Items: items, Counts: counts, Scope: scope, Scopes: scopes, Order: order})
}

// enrichFromPack fills the fields that live in the language pack rather than in
// the learner's database: what the whole sentence means, and the dictionary
// form of the target word.
func (s *Server) enrichFromPack(items []srs.QueueItem) error {
	if s.pack != nil {
		if err := s.attachSentenceTranslations(items); err != nil {
			return err
		}
		if err := s.attachLemmas(items); err != nil {
			return err
		}
	}
	// Last, and OUTSIDE the pack guard: Expected depends on the lemma the pack
	// supplies, but with no pack it still has to say the surface form rather
	// than nothing. A hint that reveals an empty string is the bug it exists to
	// fix.
	for i := range items {
		items[i].Expected = expectedFor(items[i].Direction, items[i].Lemma, items[i].TargetForm)
	}
	return nil
}

// packLemma is what the language pack knows about one lemma.
type packLemma struct{ Lemma, GlossRU, GlossEN string }

// lemmaData resolves pack lemma ids in ONE query — fifty cards must not be
// fifty round trips.
//
// An absent pack, an id of zero and a lemma the pack does not carry all yield
// no entry: never an error, and never an invented string. Callers fall back.
func (s *Server) lemmaData(ids []int64) (map[int64]packLemma, error) {
	out := map[int64]packLemma{}
	if s.pack == nil || len(ids) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(args)), ",")
	// sense_idx = 0 is the first sense, which is present for every lemma the
	// pack glosses at all.
	rows, err := s.pack.DB().Query(`
		SELECT l.id, l.lemma,
		       MAX(CASE WHEN g.lang = 'ru' THEN g.gloss END),
		       MAX(CASE WHEN g.lang = 'en' THEN g.gloss END)
		  FROM lemma l
		  LEFT JOIN lemma_gloss g ON g.lemma_id = l.id AND g.sense_idx = 0
		 WHERE l.id IN (`+marks+`)
		 GROUP BY l.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var lemma string
		var ru, en sql.NullString
		if err := rows.Scan(&id, &lemma, &ru, &en); err != nil {
			return nil, err
		}
		out[id] = packLemma{Lemma: lemma, GlossRU: ru.String, GlossEN: en.String}
	}
	return out, rows.Err()
}

// attachLemmas resolves each note's target lemma to its dictionary form, and
// supplies a meaning for any note that has none.
//
// The dictionary form is the cue a cloze is unanswerable without. `Jó ____,
// Timmy!` accepts reggelt, napot, estét and éjszakát — all real Hungarian
// greetings — so a card that names no word is asking the learner to read our
// mind, and then grading Case=Acc on the result. Cloze is the ONLY direction
// that writes to the morphological state space (Direction.GradesFeatures),
// which makes a guessed vocabulary failure land in the heatmap as a grammar
// failure. The lemma is given; the FORM is the exercise.
//
// The gloss fill repairs a different hole. A word added from the reader carries
// only a meaning the learner typed by hand, and almost nobody types one — so
// every one of those notes reached the `produce` direction, whose entire prompt
// IS the gloss, with an EMPTY question. The pack has a dictionary entry for
// nearly every word the reader can resolve. This is pack data compiled by
// people, not machine translation, so ADR-014's labelling rule does not reach
// it.
//
// Filled PER FIELD and never over a meaning the learner wrote: the reader sends
// only the learner's own locale, so somebody who typed a Russian gloss should
// still receive the pack's English one, which `pickGloss` renders behind the
// `review.glossFallback` badge. Nothing is written back — filling the DTO is
// enough, and a GET that quietly rewrote the learner's notes would be a
// surprise nobody asked for.
func (s *Server) attachLemmas(items []srs.QueueItem) error {
	ids := make([]int64, 0, len(items))
	seen := map[int64]bool{}
	for _, it := range items {
		if it.LemmaID != nil && *it.LemmaID != 0 && !seen[*it.LemmaID] {
			seen[*it.LemmaID] = true
			ids = append(ids, *it.LemmaID)
		}
	}
	byID, err := s.lemmaData(ids)
	if err != nil {
		return err
	}
	for i := range items {
		if items[i].LemmaID == nil {
			continue
		}
		d, ok := byID[*items[i].LemmaID]
		if !ok {
			continue
		}
		items[i].Lemma = d.Lemma
		if items[i].GlossRU == "" {
			items[i].GlossRU = d.GlossRU
		}
		if items[i].GlossEN == "" {
			items[i].GlossEN = d.GlossEN
		}
	}
	return nil
}

// attachSentenceTranslations fills in what the WHOLE SENTENCE means.
//
// The card used to carry only the target word's gloss, printed under the
// Hungarian with nothing to connect the two: `Jó hírt hoztunk.` above
// «хороший; добрый», which is the dictionary entry for `jó` and says nothing
// about the sentence or about which of its four words it belongs to. The pack
// has translated 19,641 of its sentences into English and 3,746 into Russian
// and the review screen was reading none of them.
//
// One query for the whole queue: fifty cards must not be fifty round trips.
func (s *Server) attachSentenceTranslations(items []srs.QueueItem) error {
	if s.pack == nil {
		return nil
	}
	ids := make([]any, 0, len(items))
	seen := map[int64]bool{}
	for _, it := range items {
		if it.SentenceID != nil && !seen[*it.SentenceID] {
			seen[*it.SentenceID] = true
			ids = append(ids, *it.SentenceID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	rows, err := s.pack.DB().Query(
		`SELECT sentence_id, lang, text FROM sentence_trans
		  WHERE sentence_id IN (`+marks+`) AND lang IN ('ru','en')`, ids...)
	if err != nil {
		return err
	}
	defer rows.Close()

	type pair struct{ ru, en string }
	byID := map[int64]*pair{}
	for rows.Next() {
		var id int64
		var lang, text string
		if err := rows.Scan(&id, &lang, &text); err != nil {
			return err
		}
		p := byID[id]
		if p == nil {
			p = &pair{}
			byID[id] = p
		}
		if lang == "ru" {
			p.ru = text
		} else {
			p.en = text
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range items {
		if items[i].SentenceID == nil {
			continue
		}
		if p := byID[*items[i].SentenceID]; p != nil {
			items[i].SentenceRU, items[i].SentenceEN = p.ru, p.en
		}
	}
	return nil
}

// AnswerRequest is one graded answer.
type AnswerRequest struct {
	CardID int64 `json:"cardId"`
	// Rating is the learner's self-grade for receptive directions. Productive
	// directions send `typed` instead and the server decides.
	Rating     int    `json:"rating"`
	Typed      string `json:"typed"`
	UsedHint   bool   `json:"usedHint"`
	DurationMs int64  `json:"durationMs"`
	// Via is how the answer was entered: "typed" (default) or "choice", a tap
	// on one of QueueItem.Options. The chosen string arrives in Typed and is
	// graded exactly like typing; Via changes only that a hint flag is ignored
	// (a choice card has no hint) and is never stored. ADR-060.
	Via string `json:"via,omitempty"`
}

// AnswerResponse reports the outcome, including which features were graded.
type AnswerResponse struct {
	Correct        bool               `json:"correct"`
	Rating         int                `json:"rating"`
	NextDue        time.Time          `json:"nextDue"`
	IntervalDays   uint64             `json:"intervalDays"`
	GradedFeatures []srs.FeatureGrade `json:"gradedFeatures"`
	Diff           *morph.Diff        `json:"diff,omitempty"`
	Expected       string             `json:"expected,omitempty"`
	// AcceptedWithAccents marks a correct-by-tolerance answer: the right word,
	// minus diacritics a plain keyboard cannot type. Shown, never silent.
	AcceptedWithAccents bool `json:"acceptedWithAccents,omitempty"`
	// Memory is the card's memory AFTER this answer, so the screen can show
	// the interval it had against the one it has now. ADR-026.
	Memory *srs.Memory `json:"memory,omitempty"`
}

// gradedAnswer is the verdict on one answer, before anything is scheduled.
type gradedAnswer struct {
	outcome             srs.Outcome
	diff                *morph.Diff
	wrongFeature        string
	acceptedWithAccents bool
}

// gradeTyped decides whether an answer was right.
//
// Shared by review and by free practice, and shared on purpose: the two modes
// differ in what they RECORD, never in what counts as correct. A second copy of
// this would drift, and the first symptom would be a word the trainer marks
// right and the review queue marks wrong.
func (s *Server) gradeTyped(d srs.Direction, expected string, req AnswerRequest) (gradedAnswer, error) {
	var g gradedAnswer
	if d.Productive() {
		// The server decides correctness for typed answers, and produces the
		// morpheme-level diff that tells the learner WHICH part was wrong.
		diff := morph.DiffForms(req.Typed, expected, s.analyzer)
		g.diff = &diff
		g.outcome.Correct = diff.Match
		g.outcome.PartiallyCorrect = diff.NearMatch
		g.wrongFeature = diff.WrongFeature

		// Accent tolerance: `viszlat` for `viszlát` is accepted — the learner
		// produced the right word on a keyboard that cannot type the mark. It
		// rates Hard (PartiallyCorrect), the diff still underlines the missing
		// accent, and the response says out loud that this was a tolerated
		// pass, never a silent one. `spell` is exempt: that direction exists
		// to train exactly the diacritics being forgiven.
		if diff.AccentOnly() && d != srs.Spell &&
			s.Config().Review.AccentToleranceEnabled() {
			g.outcome.Correct = true
			g.outcome.PartiallyCorrect = true
			g.acceptedWithAccents = true
		}
		return g, nil
	}
	r := srs.Rating(req.Rating)
	if !r.Valid() {
		return g, badRequest("rating must be between 1 and 4")
	}
	g.outcome.Correct = r != srs.Again
	// Honour an explicit self-grade rather than re-deriving it: on a receptive
	// card the learner is the only one who knows.
	g.outcome.UsedHint = r == srs.Hard
	return g, nil
}

// expectedAnswer is the string a typed answer is graded against.
//
// [R12] defines `produce` as "RU/EN -> HU (typed), form recall": the prompt is
// the target word's GLOSS, so the answer has to be the word that gloss belongs
// to — the dictionary form. `note.TargetForm` is whatever inflected token the
// note's sentence happened to contain, and grading against it prompted «день;
// солнце» while demanding `napokra`. The sublative plural is a fact the card
// never mentioned, so the learner could only fail; the placeholder even
// promised «одно слово по-венгерски» while the server wanted a case form.
//
// `cloze` keeps the surface form because it blanks that exact span, and it is
// the direction that grades morphology. `spell` keeps it because that is the
// word being dictated. The note still holds its sentence and its cloze card, so
// nothing loses the suffix — it simply lives on the direction whose job it is.
//
// Falls back to note.TargetForm whenever the lemma cannot be resolved: no
// lemma_id, no pack, or a lemma the pack does not carry. A card that grades the
// inflected form is wrong; a card that grades nothing is broken.
// See .claude/research/12-retrieval-direction.md and ADR-017.
func (s *Server) expectedAnswer(d srs.Direction, note srs.Note) string {
	if d != srs.Produce || note.LemmaID == nil || *note.LemmaID == 0 {
		return note.TargetForm
	}
	byID, err := s.lemmaData([]int64{*note.LemmaID})
	if err != nil {
		return note.TargetForm
	}
	return expectedFor(d, byID[*note.LemmaID].Lemma, note.TargetForm)
}

// expectedFor is the RULE, with the lemma already resolved.
//
// Split out so the queue and the answer path cannot disagree. The queue ships
// the expected answer with each item (QueueItem.Expected) because «Подсказка»
// has to be able to show it without grading the card, and a second copy of this
// three-line rule in the client is exactly how the hint would come to show one
// string while the grader wanted another.
func expectedFor(d srs.Direction, lemma, targetForm string) string {
	if d != srs.Produce || lemma == "" {
		return targetForm
	}
	return lemma
}

func (s *Server) handleAnswer(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	var req AnswerRequest
	if err := decode(c, &req); err != nil {
		return err
	}

	e := s.engine()
	card, err := e.Store().Card(u.ID, req.CardID)
	if err != nil {
		if errors.Is(err, srs.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "no such card")
		}
		return err
	}
	note, err := e.Store().Note(u.ID, card.NoteID)
	if err != nil {
		return err
	}
	// A word set aside between the queue being built and this answer landing.
	// The client drops the card the moment it trashes one, so this is the stale
	// tab — and the whole promise of the button is that nothing about the word
	// moves afterwards. 409, because the request was well-formed and the state
	// changed under it.
	if note.Trashed {
		return fiber.NewError(fiber.StatusConflict, "this word is in the trash")
	}

	if err := checkVia(req, card.Direction); err != nil {
		return err
	}
	if req.Via == viaChoice {
		// A choice card has no «Подсказка»: hiding options is the client's
		// affair and a hint flag here would only cap the rating. Ignored.
		req.UsedHint = false
		slog.DebugContext(c.Context(), "review answer by choice", "card", card.ID)
	}

	outcome := srs.Outcome{
		UsedHint: req.UsedHint,
		Duration: time.Duration(req.DurationMs) * time.Millisecond,
	}
	if card.Direction.Productive() {
		outcome.Typed = strings.TrimSpace(req.Typed)
	}

	var diff *morph.Diff
	var wrongFeature string
	var acceptedWithAccents bool

	expected := s.expectedAnswer(card.Direction, note)

	graded, err := s.gradeTyped(card.Direction, expected, req)
	if err != nil {
		return err
	}
	outcome.Correct = graded.outcome.Correct
	outcome.PartiallyCorrect = graded.outcome.PartiallyCorrect
	outcome.UsedHint = outcome.UsedHint || graded.outcome.UsedHint
	diff, wrongFeature, acceptedWithAccents = graded.diff, graded.wrongFeature, graded.acceptedWithAccents

	res, err := e.Answer(u.ID, card, note, outcome, wrongFeature, time.Now())
	if err != nil {
		return err
	}
	if res.GradedFeatures == nil {
		res.GradedFeatures = []srs.FeatureGrade{}
	}
	// The daily ledger, for the streak. Best-effort by design: the answer is
	// already committed, and losing it over a ledger hiccup would be worse.
	s.recordInteraction(u.ID, "review", req.DurationMs)

	out := AnswerResponse{
		Correct: outcome.Correct, Rating: int(res.Rating),
		NextDue: res.NextDue, IntervalDays: res.IntervalDays,
		GradedFeatures: res.GradedFeatures, Diff: diff,
		AcceptedWithAccents: acceptedWithAccents,
	}
	// Best-effort like the ledger: the answer is committed, and a figure the
	// widget can do without must not fail the response that carries the verdict.
	if m, err := e.Store().CardMemory(card.ID, card.NoteID, res.Card.State, e.Scheduler(), time.Now()); err == nil {
		out.Memory = &m
	}
	if card.Direction.Productive() {
		out.Expected = expected
	}
	return c.JSON(out)
}

// handleUndo reverts the learner's last answer in both state spaces.
//
// Delete-and-replay, per internal/srs/undo.go: the log is the truth, state is a
// cache of it, and the property test that proves replay fidelity proves undo.
func (s *Server) handleUndo(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	res, err := s.engine().UndoLast(u.ID, s.Config().Drill.GateWindow, time.Now())
	if errors.Is(err, srs.ErrNothingToUndo) {
		return fiber.NewError(fiber.StatusConflict, "nothing to undo")
	}
	if err != nil {
		return err
	}
	return c.JSON(res)
}

// ForecastResponse is the workload projection behind the new-card governor.
type ForecastResponse struct {
	Forecast srs.Forecast         `json:"forecast"`
	Verdict  *srs.GovernorVerdict `json:"verdict,omitempty"`
}

// handleForecast projects review load, optionally checking a proposed new-card
// limit. The curve is always returned, so the UI can show the consequence rather
// than merely reporting a refusal.
func (s *Server) handleForecast(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	cfg := s.Config()

	in, err := s.forecastInput(u.ID, cfg)
	if err != nil {
		return err
	}

	if raw := c.Query("newPerDay"); raw != "" {
		requested, err := strconv.Atoi(raw)
		if err != nil || requested < 0 {
			return badRequest("newPerDay must be a non-negative integer")
		}
		v := srs.CheckNewCardLimit(in, requested)
		return c.JSON(ForecastResponse{Forecast: v.Forecast, Verdict: &v})
	}
	return c.JSON(ForecastResponse{Forecast: srs.ProjectForecast(in)})
}

// forecastInput measures the learner's actual position rather than assuming one.
func (s *Server) forecastInput(userID int64, cfg config.Config) (srs.ForecastInput, error) {
	const window = 90

	due := make([]int, window)
	rows, err := s.db.Query(`
		SELECT CAST(julianday(c.due) - julianday('now') AS INTEGER) AS day, COUNT(*)
		  FROM card c JOIN note n ON n.id = c.note_id
		 WHERE n.user_id = ? AND c.suspended = 0 AND c.state != 0`+srs.NotTrashed+`
		 GROUP BY day`, userID)
	if err != nil {
		return srs.ForecastInput{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var day, n int
		if err := rows.Scan(&day, &n); err != nil {
			return srs.ForecastInput{}, err
		}
		if day < 0 {
			day = 0 // already overdue: it lands today
		}
		if day < window {
			due[day] += n
		}
	}

	// Cards per note comes from the actual direction plan, because "20 new a
	// day" means something very different at one card per note than at five.
	plan := srs.PlanFor(cfg.Review.Goal, directionWeights(cfg), s.audioAvailable())
	cardsPerNote := float64(len(plan.Directions))
	if cardsPerNote < 1 {
		cardsPerNote = 1
	}

	seconds := s.medianSecondsPerReview(userID)

	return srs.ForecastInput{
		ExistingDue:      due,
		NewPerDay:        cfg.Review.NewPerDay,
		CardsPerNote:     cardsPerNote,
		SecondsPerReview: seconds,
		DailyMinutes:     cfg.Review.DailyMinutes,
		DesiredRetention: cfg.Review.DesiredRetention,
		Days:             window,
	}, nil
}

// medianSecondsPerReview uses the learner's own pace once there is enough data.
// Before that, 8 seconds is a reasonable published average — and using a
// measured value later means the governor's advice improves as it learns them.
func (s *Server) medianSecondsPerReview(userID int64) float64 {
	const fallback = 8.0
	var median float64
	err := s.db.QueryRow(`
		SELECT AVG(duration_ms) / 1000.0 FROM (
			SELECT r.duration_ms FROM review_log r
			  JOIN card c ON c.id = r.card_id
			  JOIN note n ON n.id = c.note_id
			 WHERE n.user_id = ? AND r.duration_ms BETWEEN 500 AND 120000
			 ORDER BY r.review_time DESC LIMIT 200)`, userID).Scan(&median)
	if err != nil || median <= 0 {
		return fallback
	}
	return median
}

func directionWeights(cfg config.Config) map[srs.Direction]float64 {
	out := map[srs.Direction]float64{}
	for name, d := range cfg.Review.Directions {
		if d.Enabled {
			out[srs.Direction(name)] = d.Weight
		}
	}
	return out
}

// audioAvailable reports whether speech can be produced at all, which decides
// whether the audio-only directions are worth creating.
//
// `device` counts and needs no endpoint: the browser speaks it. Whether THIS
// browser has a Hungarian voice is a question only the client can answer, and
// the review screen answers it per card — an audio direction on a device with
// no voice says so instead of presenting a silent card.
func (s *Server) audioAvailable() bool {
	tts := s.Config().Services.TTS
	switch tts.Provider {
	case config.ProviderOff:
		return false
	case config.ProviderDevice:
		return true
	default:
		return tts.Endpoint != ""
	}
}

// backfillClozeCards repairs notes that predate the current cloze rule.
//
// A cloze once required features as well as a span, which left every word the
// analyser could not decorate — 23 of 37 mined from a text, and 21 course words
// besides — with a sentence nothing ever asked about. The review screen is
// where that gap shows, and it is the only place holding both the notes and the
// learner's configured direction plan, which is exactly what a SQL migration
// cannot see. See ADR-017.
//
// Skipped outright when the learner has cloze switched off: repairing a card
// somebody disabled is not a repair.
func (s *Server) backfillClozeCards(userID int64) error {
	cfg := s.Config()
	plan := srs.PlanFor(cfg.Review.Goal, directionWeights(cfg), s.audioAvailable())
	if !slices.Contains(plan.Directions, srs.Cloze) {
		return nil
	}
	n, err := srs.NewStore(s.db.DB).BackfillClozeCards(userID, time.Now())
	if err != nil {
		return err
	}
	if n > 0 {
		slog.Info("backfilled cloze cards", "user", userID, "cards", n)
	}
	return nil
}
