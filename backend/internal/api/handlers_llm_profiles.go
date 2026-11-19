package api

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/config"
)

// Saved tutor profiles (ADR-082). `services.llm` stays the one ACTIVE config
// every call site reads; a profile is a preset that is copied into it. All of
// this goes through updateConfig, and every change drops the probe cache so the
// «на связи» status reflects the new tutor. Admin-only: config is global.

// detachLLMProfileIfEdited clears llm_active when services.llm no longer
// equals the active profile (an edit made through the ordinary settings path),
// so the UI shows the tutor as "not saved" instead of naming a stale profile.
func detachLLMProfileIfEdited(cfg *config.Config) {
	sv := &cfg.Services
	if sv.LLMActive == "" {
		return
	}
	for _, p := range sv.LLMProfiles {
		if p.ID == sv.LLMActive {
			if p.Provider != sv.LLM.Provider || p.Endpoint != sv.LLM.Endpoint || p.Model != sv.LLM.Model {
				sv.LLMActive = ""
			}
			return
		}
	}
	sv.LLMActive = ""
}

func findLLMProfile(cfg *config.Config, id string) int {
	for i, p := range cfg.Services.LLMProfiles {
		if p.ID == id {
			return i
		}
	}
	return -1
}

func newProfileID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// applyLLMProfile copies a profile into the active slot. Paused is kept: a
// paused tutor stays paused across a switch.
func applyLLMProfile(cfg *config.Config, p config.LLMProfile) {
	cfg.Services.LLM.Provider = p.Provider
	cfg.Services.LLM.Endpoint = p.Endpoint
	cfg.Services.LLM.Model = p.Model
	cfg.Services.LLMActive = p.ID
}

func (r LLMProfileRequest) clean() LLMProfileRequest {
	r.Name = strings.TrimSpace(r.Name)
	r.Provider = strings.TrimSpace(r.Provider)
	r.Endpoint = strings.TrimSpace(r.Endpoint)
	r.Model = strings.TrimSpace(r.Model)
	if r.Provider == config.ProviderOff {
		r.Endpoint, r.Model = "", ""
	}
	return r
}

// settingsAfterProfileChange answers every profile endpoint with fresh settings.
func (s *Server) settingsAfterProfileChange(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	s.invalidateCapabilityCache()
	secrets, err := s.secretPresence(u.ID)
	if err != nil {
		return err
	}
	return c.JSON(settingsDTO(s.Config(), secrets))
}

// handleLLMProfileCreate saves a new named preset and makes it the active one
// (the learner saves what they are using now, or switches right after).
func (s *Server) handleLLMProfileCreate(c fiber.Ctx) error {
	var req LLMProfileRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	req = req.clean()
	if _, err := s.updateConfig(func(cfg *config.Config) {
		p := config.LLMProfile{ID: newProfileID(), Name: req.Name, Provider: req.Provider, Endpoint: req.Endpoint, Model: req.Model}
		cfg.Services.LLMProfiles = append(cfg.Services.LLMProfiles, p)
		applyLLMProfile(cfg, p)
	}); err != nil {
		return err
	}
	return s.settingsAfterProfileChange(c)
}

// handleLLMProfileUpdate edits a preset; if it is the active one, the active
// slot follows.
func (s *Server) handleLLMProfileUpdate(c fiber.Ctx) error {
	id := c.Params("id")
	var req LLMProfileRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	req = req.clean()
	var missing bool
	if _, err := s.updateConfig(func(cfg *config.Config) {
		i := findLLMProfile(cfg, id)
		if i < 0 {
			missing = true
			return
		}
		p := config.LLMProfile{ID: id, Name: req.Name, Provider: req.Provider, Endpoint: req.Endpoint, Model: req.Model}
		cfg.Services.LLMProfiles[i] = p
		if cfg.Services.LLMActive == id {
			applyLLMProfile(cfg, p)
		}
	}); err != nil {
		return err
	}
	if missing {
		return fiber.NewError(fiber.StatusNotFound, "errors.llmProfileUnknown")
	}
	return s.settingsAfterProfileChange(c)
}

// handleLLMProfileDelete removes a preset. The active tutor keeps working as it
// is; it just no longer names a profile.
func (s *Server) handleLLMProfileDelete(c fiber.Ctx) error {
	id := c.Params("id")
	var missing bool
	if _, err := s.updateConfig(func(cfg *config.Config) {
		i := findLLMProfile(cfg, id)
		if i < 0 {
			missing = true
			return
		}
		cfg.Services.LLMProfiles = append(cfg.Services.LLMProfiles[:i], cfg.Services.LLMProfiles[i+1:]...)
		if cfg.Services.LLMActive == id {
			cfg.Services.LLMActive = ""
		}
	}); err != nil {
		return err
	}
	if missing {
		return fiber.NewError(fiber.StatusNotFound, "errors.llmProfileUnknown")
	}
	return s.settingsAfterProfileChange(c)
}

// handleLLMProfileActivate switches the tutor: copies the preset into
// services.llm. No call site elsewhere changes.
func (s *Server) handleLLMProfileActivate(c fiber.Ctx) error {
	id := c.Params("id")
	var missing bool
	if _, err := s.updateConfig(func(cfg *config.Config) {
		i := findLLMProfile(cfg, id)
		if i < 0 {
			missing = true
			return
		}
		applyLLMProfile(cfg, cfg.Services.LLMProfiles[i])
	}); err != nil {
		return err
	}
	if missing {
		return fiber.NewError(fiber.StatusNotFound, "errors.llmProfileUnknown")
	}
	return s.settingsAfterProfileChange(c)
}
