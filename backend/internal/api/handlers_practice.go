package api

import (
	"errors"
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
// It is a SEPARATE MODE, not a second scheduler. A practice answer changes no
// card state, no feature state, and writes nothing to review_log. Extra
// retrieval is good for learning [R1], but answering ahead of schedule tells
// FSRS the learner recalled something at an interval they were never tested at,
// and discounting the update instead would mean writing scheduling maths of our
// own — which [ADR-003] forbids. So closing a word stays the review queue's
// job, and the UI says so rather than leaving the learner to infer it.
// See ADR-019.

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
// Deliberately carries no nextDue and no gradedFeatures: there is nothing
// scheduled and nothing graded into the second state space, and returning empty
// fields shaped like the review response would invite the client to render a
// schedule that does not exist.
type PracticeAnswerResponse struct {
	Correct             bool        `json:"correct"`
	Diff                *morph.Diff `json:"diff,omitempty"`
	Expected            string      `json:"expected,omitempty"`
	AcceptedWithAccents bool        `json:"acceptedWithAccents,omitempty"`
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

	// The same verdict the review queue would reach. The modes differ in what
	// they record, never in what counts as correct.
	expected := s.expectedAnswer(card.Direction, note)
	graded, err := s.gradeTyped(card.Direction, expected, req)
	if err != nil {
		return err
	}

	now := time.Now()
	if err := store.LogPractice(u.ID, card.ID, card.Direction,
		graded.outcome.Correct, req.DurationMs, now); err != nil {
		return err
	}
	// Minutes count: this is real work, and the ledger measures effort rather
	// than obedience. The STREAK is untouched by it — that needs the real queue
	// cleared as well, so practice can never stand in for the day's obligation.
	_ = stats.NewStore(s.db.DB).RecordPractice(u.ID, localDay(now), req.DurationMs)

	out := PracticeAnswerResponse{
		Correct:             graded.outcome.Correct,
		AcceptedWithAccents: graded.acceptedWithAccents,
	}
	if card.Direction.Productive() {
		out.Diff = graded.diff
		out.Expected = expected
	}
	return c.JSON(out)
}
