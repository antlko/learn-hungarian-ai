package api

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/config"
	"tanulom/internal/db"
)

// errorHandler renders every failure as {"error": "..."} — the single shape the
// frontend parses. Handlers return fiber.NewError(code, msg); they never format
// an error body themselves.
func errorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	msg := "internal server error"

	var fe *fiber.Error
	switch {
	case errors.As(err, &fe):
		code, msg = fe.Code, fe.Message
	case errors.Is(err, db.ErrNotFound):
		code, msg = fiber.StatusNotFound, "not found"
	}
	// Any other error keeps the generic message: internal detail stays in the log,
	// not in the response.
	return c.Status(code).JSON(fiber.Map{"error": msg})
}

// decode parses a JSON body, turning malformed input into a 400 rather than a 500.
func decode(c fiber.Ctx, dst any) error {
	if err := c.Bind().Body(dst); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	return nil
}

// badRequest is shorthand for a 400 with a specific message.
func badRequest(format string, args ...any) error {
	return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf(format, args...))
}

// userDTO renders a user without its password hash.
func userDTO(u db.User) UserDTO {
	return UserDTO{
		ID:        u.ID,
		Email:     u.Email,
		Role:      u.Role,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// settingsDTO renders config for the wire. Secret values are never included —
// only their presence, supplied by the caller.
func settingsDTO(c config.Config, secrets map[string]bool) SettingsDTO {
	dirs := make(map[string]DirectionDTO, len(c.Review.Directions))
	for k, v := range c.Review.Directions {
		dirs[k] = DirectionDTO{Enabled: v.Enabled, Weight: v.Weight}
	}
	svc := func(s config.Service) ServiceDTO {
		return ServiceDTO{Provider: s.Provider, Endpoint: s.Endpoint, Model: s.Model, Voices: s.Voices}
	}
	if secrets == nil {
		secrets = map[string]bool{}
	}
	return SettingsDTO{
		Locale: LocaleDTO{UI: c.Locale.UI, Gloss: c.Locale.Gloss, GlossFallback: c.Locale.GlossFallback},
		Review: ReviewDTO{
			DesiredRetention: c.Review.DesiredRetention,
			NewPerDay:        c.Review.NewPerDay,
			MaxReviewsPerDay: c.Review.MaxReviewsPerDay,
			DailyMinutes:     c.Review.DailyMinutes,
			Goal:             c.Review.Goal,
			Directions:       dirs,
			BurySiblings:     c.Review.BurySiblingsEnabled(),
		},
		Mining: MiningDTO{
			KnownThreshold: c.Mining.KnownThreshold,
			FeatureWeight:  c.Mining.FeatureWeight,
			TargetUnknown:  c.Mining.TargetUnknown,
			UnknownSigma:   c.Mining.UnknownSigma,
			MinCoverage:    c.Mining.MinCoverage,
			TargetLength:   c.Mining.TargetLength,
		},
		Drill: DrillDTO{
			GateThreshold:     c.Drill.GateThreshold,
			GateWindow:        c.Drill.GateWindow,
			MasteryThreshold:  c.Drill.MasteryThreshold,
			MasteryWindow:     c.Drill.MasteryWindow,
			Hysteresis:        c.Drill.Hysteresis,
			DefiniteObjectPct: c.Drill.Definiteness.DefiniteObjectPct,
			DemonstrativePct:  c.Drill.Definiteness.DemonstrativePct,
			HarmonyLatencyMs:  c.Drill.HarmonyLatencyMs,
		},
		HVPT: HVPTDTO{
			MinTalkers:       c.HVPT.MinTalkers,
			TrialsPerSession: c.HVPT.TrialsPerSession,
			RateJitter:       c.HVPT.RateJitter,
		},
		Curriculum: CurriculumDTO{
			FocusUnlockListeningHours: c.Curriculum.FocusUnlockListeningHours,
			ExamDate:                  c.Curriculum.ExamDate,
		},
		Services: map[string]ServiceDTO{
			"tts": svc(c.Services.TTS),
			"asr": svc(c.Services.ASR),
			"nlp": svc(c.Services.NLP),
			"llm": svc(c.Services.LLM),
		},
		Secrets: secrets,
	}
}

// jsonUnmarshalString decodes a JSON string column, tolerating an empty value.
func jsonUnmarshalString(s string, dst any) error {
	if s == "" {
		return nil
	}
	return json.Unmarshal([]byte(s), dst)
}
