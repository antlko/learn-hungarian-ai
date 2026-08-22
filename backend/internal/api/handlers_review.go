package api

import (
	"errors"
	"strconv"
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
}

func (s *Server) handleQueue(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	cfg := s.Config()
	store := srs.NewStore(s.db.DB)

	items, counts, err := store.Queue(u.ID, cfg.Review.NewPerDay,
		cfg.Review.MaxReviewsPerDay, cfg.Review.BurySiblingsEnabled(), time.Now())
	if err != nil {
		return err
	}
	if items == nil {
		items = []srs.QueueItem{}
	}
	return c.JSON(QueueResponse{Items: items, Counts: counts})
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

	outcome := srs.Outcome{
		UsedHint: req.UsedHint,
		Duration: time.Duration(req.DurationMs) * time.Millisecond,
	}

	var diff *morph.Diff
	var wrongFeature string

	if card.Direction.Productive() {
		// The server decides correctness for typed answers, and produces the
		// morpheme-level diff that tells the learner WHICH part was wrong.
		d := morph.DiffForms(req.Typed, note.TargetForm, s.analyzer)
		diff = &d
		outcome.Correct = d.Match
		outcome.PartiallyCorrect = d.NearMatch
		wrongFeature = d.WrongFeature
	} else {
		r := srs.Rating(req.Rating)
		if !r.Valid() {
			return badRequest("rating must be between 1 and 4")
		}
		outcome.Correct = r != srs.Again
		// Honour an explicit self-grade rather than re-deriving it: on a
		// receptive card the learner is the only one who knows.
		outcome.UsedHint = outcome.UsedHint || r == srs.Hard
	}

	res, err := e.Answer(u.ID, card, note, outcome, wrongFeature, time.Now())
	if err != nil {
		return err
	}
	if res.GradedFeatures == nil {
		res.GradedFeatures = []srs.FeatureGrade{}
	}

	out := AnswerResponse{
		Correct: outcome.Correct, Rating: int(res.Rating),
		NextDue: res.NextDue, IntervalDays: res.IntervalDays,
		GradedFeatures: res.GradedFeatures, Diff: diff,
	}
	if card.Direction.Productive() {
		out.Expected = note.TargetForm
	}
	return c.JSON(out)
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
		 WHERE n.user_id = ? AND c.suspended = 0 AND c.state != 0
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

// audioAvailable reports whether TTS is reachable, which decides whether the
// audio-only directions can be presented at all.
func (s *Server) audioAvailable() bool {
	cfg := s.Config()
	return cfg.Services.TTS.Provider != config.ProviderOff && cfg.Services.TTS.Endpoint != ""
}
