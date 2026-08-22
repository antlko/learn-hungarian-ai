package api

import (
	"errors"
	"math/rand"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/hvpt"
)

// Ear-trainer endpoints. See internal/hvpt and [R7].

func (s *Server) requireHVPT() error {
	if err := s.requirePack(); err != nil {
		return err
	}
	if s.hvptInv == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable,
			"the language pack has no minimal-pair inventory")
	}
	return nil
}

// HVPTSessionResponse is one session, or the honest refusal.
type HVPTSessionResponse struct {
	Trials []hvpt.Trial `json:"trials"`
	// Talkers is the distinct-voice count behind this session — auditable
	// variety, not asserted variety.
	Talkers int `json:"talkers"`
	// Today's running numbers, so the header survives a reload.
	Today hvpt.SessionStats `json:"today"`
	// Refused explains an empty session: the talker floor with its numbers.
	Refused *TalkerFloorDTO `json:"refused,omitempty"`
}

type TalkerFloorDTO struct {
	Have int `json:"have"`
	Need int `json:"need"`
}

func (s *Server) handleHVPTSession(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if err := s.requireHVPT(); err != nil {
		return err
	}
	cfg := s.Config()
	store := hvpt.NewStore(s.db.DB)

	today, err := store.TodayStats(u.ID, startOfDay(time.Now()))
	if err != nil {
		return err
	}
	states, err := store.States(u.ID)
	if err != nil {
		return err
	}

	trials, err := s.hvptInv.SessionPlan(states, cfg.HVPT.TrialsPerSession,
		cfg.HVPT.MinTalkers, rand.New(rand.NewSource(time.Now().UnixNano())),
		time.Now(), s.engine().Scheduler())
	var floor *hvpt.TalkerFloorError
	if errors.As(err, &floor) {
		// Refusal is a 200 with the reason, not an error status: the screen it
		// produces is a designed state of the product, not a failure.
		return c.JSON(HVPTSessionResponse{
			Trials: []hvpt.Trial{}, Talkers: floor.Have, Today: today,
			Refused: &TalkerFloorDTO{Have: floor.Have, Need: floor.Need},
		})
	}
	if err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, err.Error())
	}

	served, err := s.hvptPending.Put(u.ID, trials)
	if err != nil {
		return err
	}
	return c.JSON(HVPTSessionResponse{
		Trials: served, Talkers: len(s.hvptInv.Talkers), Today: today,
	})
}

// HVPTAnswerRequest is one 2AFC choice.
type HVPTAnswerRequest struct {
	TrialID string `json:"trialId"`
	Chosen  string `json:"chosen"` // "a" | "b"
	RtMs    int64  `json:"rtMs"`
}

// HVPTAnswerResponse reveals the played member AFTER the choice.
type HVPTAnswerResponse struct {
	Correct  bool    `json:"correct"`
	Played   string  `json:"played"`
	Accuracy float64 `json:"accuracy"`
	Total    int     `json:"total"`
}

func (s *Server) handleHVPTAnswer(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if err := s.requireHVPT(); err != nil {
		return err
	}
	var req HVPTAnswerRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	if req.Chosen != "a" && req.Chosen != "b" {
		return badRequest("chosen must be a or b")
	}
	trial, ok := s.hvptPending.Take(u.ID, req.TrialID)
	if !ok {
		return fiber.NewError(fiber.StatusGone, "this trial has expired — fetch a new session")
	}
	cs, correct, err := hvpt.NewStore(s.db.DB).Answer(u.ID, trial, req.Chosen, req.RtMs,
		s.engine().Scheduler(), s.Config().Drill.GateWindow, time.Now())
	if err != nil {
		return err
	}
	s.recordInteraction(u.ID, "trial", req.RtMs)
	return c.JSON(HVPTAnswerResponse{
		Correct: correct, Played: trial.Played(),
		Accuracy: cs.Accuracy, Total: cs.Total,
	})
}

// HVPTContrastDTO is one contrast for the overview list.
type HVPTContrastDTO struct {
	Contrast string  `json:"contrast"`
	Seen     bool    `json:"seen"`
	Accuracy float64 `json:"accuracy"`
	Total    int     `json:"total"`
	Due      bool    `json:"due"`
}

func (s *Server) handleHVPTContrasts(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if err := s.requireHVPT(); err != nil {
		return err
	}
	states, err := hvpt.NewStore(s.db.DB).States(u.ID)
	if err != nil {
		return err
	}
	now := time.Now()
	out := []HVPTContrastDTO{}
	for _, contrast := range s.hvptInv.AudibleContrasts() {
		dto := HVPTContrastDTO{Contrast: contrast}
		if st, ok := states[contrast]; ok {
			dto.Seen = true
			dto.Accuracy, dto.Total = st.Accuracy, st.Total
			dto.Due = !st.State.Due.After(now)
		}
		out = append(out, dto)
	}
	return c.JSON(fiber.Map{"contrasts": out, "talkers": len(s.hvptInv.Talkers)})
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
