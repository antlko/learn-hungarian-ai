package api

import (
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/config"
	"tanulom/internal/morph"
	"tanulom/internal/srs"
	"tanulom/internal/stats"
)

// Free practice.
//
// The review queue answers "what do I owe today". This answers "let me train",
// which has to be answerable at any moment — with the queue cleared, or with a
// deck of nothing but new cards. It draws at random from a chosen pile and
// ignores due dates entirely.
//
// A practice answer is a REAL REVIEW [R20] (ADR-064, superseding ADR-019): the
// same Engine.Answer the queue calls, so FSRS itself discounts an early answer
// from the real elapsed time — no scheduling maths of ours (ADR-003). Two
// exceptions stay practice_log-only: words in «Знаю» (nothing about them
// moves) and NEW cards (a first review would bypass the daily new budget).
// See .claude/research/20-practice-and-progress.md.

// PracticeQueueResponse is a practice session.
type PracticeQueueResponse struct {
	Items []srs.QueueItem `json:"items"`
	Scope srs.Scope       `json:"scope"`
	// Mode echoes what was actually served — including the fallback from weak
	// to random on a deck with nothing reviewed yet. The screen prints the
	// selection criterion, so the echo is what keeps it honest. ADR-024.
	Mode string `json:"mode"`
	// Pool is how many words the scope could offer at all, so the screen can
	// say "nothing here yet" instead of opening an empty session.
	Pool int `json:"pool"`
}

func (s *Server) handlePracticeQueue(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	scope, ok := srs.ParseScope(c.Query("scope"))
	if !ok {
		return badRequest("scope must be all, course, reader, trash or text:<id>")
	}

	// Round length is a per-session choice, clamped rather than refused: the
	// point of the bounds is a session with a shape, not input discipline.
	size := config.DefaultPracticeSession
	if raw := c.Query("size"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return badRequest("size must be a number")
		}
		size = min(max(n, config.MinPracticeSession), config.MaxPracticeSession)
	}
	mode := c.Query("mode", "random")
	if mode != "random" && mode != "weak" {
		return badRequest("mode must be random or weak")
	}

	store := srs.NewStore(s.db.DB)
	var items []srs.QueueItem
	served := mode
	if mode == "weak" {
		items, err = store.PracticeQueueWeak(u.ID, scope, size,
			s.engine().Scheduler(), time.Now())
		if err != nil {
			return err
		}
		if len(items) == 0 {
			// Nothing reviewed yet: there is no weakness to rank. Fall back to
			// random and say so — serving an empty round would read as broken.
			served = "random"
		}
	}
	if served == "random" {
		items, err = store.PracticeQueue(u.ID, scope, size)
		if err != nil {
			return err
		}
	}
	if items == nil {
		items = []srs.QueueItem{}
	}
	if err := s.enrichFromPack(items); err != nil {
		return err
	}
	if err := s.attachCribs(u.ID, items); err != nil {
		return err
	}
	// The card's memory figures, WITHOUT the grade preview: nothing here is
	// scheduled, and an interval hint under a practice card would be the
	// screen inventing a schedule. ADR-019, ADR-026.
	if err := store.AttachMemory(items, s.engine().Scheduler(), time.Now(), false); err != nil {
		return err
	}
	pool, err := store.PracticeCount(u.ID, scope)
	if err != nil {
		return err
	}
	s.attachOptions(items, time.Now())
	s.warmCribs(u.ID, items)
	return c.JSON(PracticeQueueResponse{Items: items, Scope: scope, Mode: served, Pool: pool})
}

// PracticeAnswerResponse is the verdict on one practice answer.
//
// Counted says whether the answer was a real review. When it was, the
// scheduling fields are filled with the same shapes as AnswerResponse; when it
// was not, Reason says why ("known": the word is in «Знаю»; "new": the card has
// never been reviewed) and the scheduling fields are absent, so the client
// cannot render a schedule that does not exist.
type PracticeAnswerResponse struct {
	Correct             bool        `json:"correct"`
	Diff                *morph.Diff `json:"diff,omitempty"`
	Expected            string      `json:"expected,omitempty"`
	AcceptedWithAccents bool        `json:"acceptedWithAccents,omitempty"`

	Counted        bool               `json:"counted"`
	Reason         string             `json:"reason,omitempty"`
	Rating         int                `json:"rating,omitempty"`
	NextDue        *time.Time         `json:"nextDue,omitempty"`
	IntervalDays   *uint64            `json:"intervalDays,omitempty"`
	GradedFeatures []srs.FeatureGrade `json:"gradedFeatures,omitempty"`
	Memory         *srs.Memory        `json:"memory,omitempty"`
}

func (s *Server) handlePracticeAnswer(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	var req AnswerRequest
	if err := decode(c, &req); err != nil {
		return err
	}

	store := srs.NewStore(s.db.DB)
	card, err := store.Card(u.ID, req.CardID)
	if err != nil {
		if errors.Is(err, srs.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "no such card")
		}
		return err
	}
	note, err := store.Note(u.ID, card.NoteID)
	if err != nil {
		return err
	}

	if err := checkVia(req, card.Direction); err != nil {
		return err
	}
	if req.Via == viaChoice {
		req.UsedHint = false
	}

	// The same verdict the review queue would reach. The modes differ in what
	// they record, never in what counts as correct.
	expected := s.expectedAnswer(card.Direction, note)
	graded, err := s.gradeTyped(card.Direction, expected, req)
	if err != nil {
		return err
	}

	// One instant for the review_log row and the practice_log row: undo finds
	// the pair by (card, instant). db.FmtTime formats both.
	now := time.Now()
	out := PracticeAnswerResponse{
		Correct:             graded.outcome.Correct,
		AcceptedWithAccents: graded.acceptedWithAccents,
	}
	if card.Direction.Productive() {
		out.Diff = graded.diff
		out.Expected = expected
	}

	switch {
	case note.Trashed:
		out.Reason = "known"
	case card.State.IsNew():
		out.Reason = "new"
	}

	if out.Reason != "" {
		if err := store.LogPractice(u.ID, card.ID, card.Direction,
			graded.outcome.Correct, req.DurationMs, now); err != nil {
			return err
		}
		// Minutes count: this is real work, and the ledger measures effort
		// rather than obedience. The STREAK is untouched by it — that needs the
		// real queue cleared as well.
		_ = stats.NewStore(s.db.DB).RecordPractice(u.ID, localDay(now), req.DurationMs)
		return c.JSON(out)
	}

	// A real review: the engine path the review queue uses (1 card review +
	// feature reviews only for the features the item required).
	e := s.engine()
	outcome := answerOutcome(card.Direction, req, graded)
	res, err := e.Answer(u.ID, card, note, outcome, graded.wrongFeature, now)
	if err != nil {
		return err
	}
	// Best-effort: the review is committed, and the practice tally is a stat.
	if err := store.LogPractice(u.ID, card.ID, card.Direction,
		graded.outcome.Correct, req.DurationMs, now); err != nil {
		slog.WarnContext(c.Context(), "log practice after review", "card", card.ID, "err", err)
	}
	s.recordInteraction(u.ID, "review", req.DurationMs)

	if res.GradedFeatures == nil {
		res.GradedFeatures = []srs.FeatureGrade{}
	}
	out.Counted = true
	out.Rating = int(res.Rating)
	out.NextDue = &res.NextDue
	out.IntervalDays = &res.IntervalDays
	out.GradedFeatures = res.GradedFeatures
	if m, err := e.Store().CardMemory(card.ID, card.NoteID, res.Card.State, e.Scheduler(), now); err == nil {
		out.Memory = &m
	}
	return c.JSON(out)
}
