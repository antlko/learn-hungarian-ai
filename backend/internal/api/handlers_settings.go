package api

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/config"
)

// secretKeys are the settings whose values never leave the server. Only presence
// is exposed, as SettingsDTO.Secrets.
var secretKeys = []string{"ttsKey", "asrKey", "nlpKey", "llmKey"}

// secretPresence reports which secrets are stored, without their values.
func (s *Server) secretPresence(userID int64) (map[string]bool, error) {
	stored, err := s.db.Settings(userID)
	if err != nil {
		return nil, err
	}
	present := make(map[string]bool, len(secretKeys))
	for _, k := range secretKeys {
		present[k] = false
	}
	for _, st := range stored {
		if st.Secret && st.Value != "" {
			present[st.Key] = true
		}
	}
	return present, nil
}

func (s *Server) handleGetSettings(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	secrets, err := s.secretPresence(u.ID)
	if err != nil {
		return err
	}
	return c.JSON(settingsDTO(s.Config(), secrets))
}

// handleUpdateSettings applies a partial update. Omitted sections are unchanged.
//
// Secrets: a blank or omitted value preserves what is stored. This is what lets
// the UI round-trip settings without ever having seen the secret.
func (s *Server) handleUpdateSettings(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}

	var req UpdateSettingsRequest
	if err := decode(c, &req); err != nil {
		return err
	}

	// Validate every name up front: updateConfig saves atomically, so rejecting
	// an unknown service afterwards would leave the rest of the edit persisted.
	for name := range req.Services {
		if serviceField(&config.Config{}, name) == nil {
			return badRequest("unknown service %q", name)
		}
	}

	// Secrets live in SQLite, not in config.yaml — apply them separately.
	for key, val := range req.Secrets {
		if !isSecretKey(key) {
			return badRequest("unknown secret %q", key)
		}
		if strings.TrimSpace(val) == "" {
			continue // blank means "keep what is stored"
		}
		if err := s.db.SetSetting(u.ID, key, val, true); err != nil {
			return err
		}
	}

	clamped, err := s.updateConfig(func(cfg *config.Config) {
		if v := req.Locale; v != nil {
			cfg.Locale.UI = v.UI
			cfg.Locale.Gloss = v.Gloss
			cfg.Locale.GlossFallback = v.GlossFallback
		}
		if v := req.Review; v != nil {
			cfg.Review.DesiredRetention = v.DesiredRetention
			cfg.Review.NewPerDay = v.NewPerDay
			cfg.Review.MaxReviewsPerDay = v.MaxReviewsPerDay
			cfg.Review.DailyMinutes = v.DailyMinutes
			cfg.Review.Goal = v.Goal
			b := v.BurySiblings
			cfg.Review.BurySiblings = &b
			if v.Directions != nil {
				dirs := make(map[string]config.Direction, len(v.Directions))
				for k, d := range v.Directions {
					dirs[k] = config.Direction{Enabled: d.Enabled, Weight: d.Weight}
				}
				cfg.Review.Directions = dirs
			}
		}
		if v := req.Mining; v != nil {
			cfg.Mining.KnownThreshold = v.KnownThreshold
			cfg.Mining.FeatureWeight = v.FeatureWeight
			cfg.Mining.TargetUnknown = v.TargetUnknown
			cfg.Mining.UnknownSigma = v.UnknownSigma
			cfg.Mining.MinCoverage = v.MinCoverage
			cfg.Mining.TargetLength = v.TargetLength
		}
		if v := req.Drill; v != nil {
			cfg.Drill.GateThreshold = v.GateThreshold
			cfg.Drill.GateWindow = v.GateWindow
			cfg.Drill.MasteryThreshold = v.MasteryThreshold
			cfg.Drill.MasteryWindow = v.MasteryWindow
			cfg.Drill.Hysteresis = v.Hysteresis
			cfg.Drill.Definiteness.DefiniteObjectPct = v.DefiniteObjectPct
			cfg.Drill.Definiteness.DemonstrativePct = v.DemonstrativePct
			cfg.Drill.HarmonyLatencyMs = v.HarmonyLatencyMs
		}
		if v := req.HVPT; v != nil {
			cfg.HVPT.MinTalkers = v.MinTalkers
			cfg.HVPT.TrialsPerSession = v.TrialsPerSession
			cfg.HVPT.RateJitter = v.RateJitter
		}
		if v := req.Curriculum; v != nil {
			cfg.Curriculum.FocusUnlockListeningHours = v.FocusUnlockListeningHours
		}
		for name, svc := range req.Services {
			target := serviceField(cfg, name)
			if target == nil {
				continue // unreachable: names were validated above
			}
			target.Provider = svc.Provider
			target.Endpoint = svc.Endpoint
			target.Model = svc.Model
			target.Voices = svc.Voices
		}
	})
	if err != nil {
		return err
	}

	// Service endpoints may have changed; do not show stale probe results.
	InvalidateCapabilityCache()

	secrets, err := s.secretPresence(u.ID)
	if err != nil {
		return err
	}
	out := UpdateSettingsResponse{Settings: settingsDTO(s.Config(), secrets)}
	for _, cl := range clamped {
		out.Clamped = append(out.Clamped, ClampDTO{Field: cl.Field, From: cl.From, To: cl.To})
	}
	return c.JSON(out)
}

// serviceField maps a service name to its field in a Config.
func serviceField(cfg *config.Config, name string) *config.Service {
	switch name {
	case "tts":
		return &cfg.Services.TTS
	case "asr":
		return &cfg.Services.ASR
	case "nlp":
		return &cfg.Services.NLP
	case "llm":
		return &cfg.Services.LLM
	default:
		return nil
	}
}

func isSecretKey(k string) bool {
	for _, s := range secretKeys {
		if s == k {
			return true
		}
	}
	return false
}
