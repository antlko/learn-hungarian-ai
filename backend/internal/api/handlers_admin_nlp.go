package api

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/config"
	"tanulom/internal/nlp"
)

// nlpComponentState maps the sidecar's own words onto the vocabulary the
// Components card already draws (loading / ready / failed), so no frontend
// change is needed: downloading and loading are both "loading" with progress.
func nlpComponentState(status string) string {
	switch status {
	case "downloading", "loading":
		return "loading"
	case "ready":
		return "ready"
	default:
		return "failed"
	}
}

// fillNLPComponent adds the sidecar's own state to its Components row.
func (s *Server) fillNLPComponent(ctx context.Context, d *ComponentDTO, svc config.Service, configured bool) {
	client := nlp.New(svc)
	if client == nil || !configured {
		return
	}
	h, err := client.Health(ctx)
	if err != nil {
		return // unreachable: the generic state stands
	}
	d.Reachable, d.State, d.Model = true, nlpComponentState(h.Status), h.Model
	if h.Status != "ready" {
		d.Progress = h.Progress
	}
}

// handleNLPWarmup asks the sidecar to download/load its model. Admin-only, like
// /api/mt/warmup: a 127 MB fetch on a shared install is not a per-learner
// button. Progress is read back through GET /api/admin/components.
func (s *Server) handleNLPWarmup(c fiber.Ctx) error {
	if _, err := mustUser(c); err != nil {
		return err
	}
	client := nlp.New(s.service("nlp"))
	if client == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "errors.nlpNotConfigured")
	}
	h, err := client.Warmup(c.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "errors.nlpUnreachable")
	}
	return c.JSON(ComponentDTO{
		ID: "nlp", Configured: true, Reachable: true,
		State: nlpComponentState(h.Status), Progress: h.Progress, Model: h.Model,
		ComposeProfile: "nlp", Built: true,
	})
}
