package api

import (
	"github.com/gofiber/fiber/v3"

	"tanulom/internal/config"
)

// PausedRequest is the body of POST /api/admin/services/:name/paused.
type PausedRequest struct {
	Paused bool `json:"paused"`
}

// handleServicePaused switches one optional service off or on without
// forgetting its provider, endpoint, model or secret (ADR-068). A managed child
// is stopped (or started again) to match. Admin-only: config is global.
func (s *Server) handleServicePaused(c fiber.Ctx) error {
	name := c.Params("name")
	if serviceField(&config.Config{}, name) == nil {
		return fiber.NewError(fiber.StatusNotFound, "errors.serviceUnknown")
	}
	var req PausedRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	prev := s.Config()
	if _, err := s.updateConfig(func(cfg *config.Config) {
		serviceField(cfg, name).Paused = req.Paused
	}); err != nil {
		return err
	}
	s.reconcileManaged(prev, s.Config())
	s.invalidateCapabilityCache()
	return c.JSON(s.capabilities(c.Context()))
}
