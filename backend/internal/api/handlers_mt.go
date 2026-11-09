package api

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/mt"
)

// Managing the translation sidecar from Settings, instead of over SSH.
//
// The problem these two endpoints solve is a reporting one. `/api/capabilities`
// probes every optional service the same way — `GET /health`, anything under
// 500 is reachable — and that is right for four of the five. It is wrong for
// `mt`, because this service has a long, legitimate not-ready state: on first
// boot it downloads and converts 2.4 GB of weights, and answers 503 for the
// several minutes that takes. Flattened through the probe, "your model is
// downloading" and "you never deployed this" both arrive at Settings as «нет
// связи», and the only way to tell them apart was to read container logs.
//
// So `mt` gets its own status route, which passes the service's own words
// through, and a warmup route, which makes a failed download retryable without
// restarting the container.
//
// What this deliberately does NOT do: start the sidecar. The core is a
// distroless static binary with no shell and no Docker socket, and it cannot
// conjure a process nobody deployed. Claiming otherwise with a button that
// quietly fails would be exactly the kind of promise ADR-010 refused to make
// about the language pack. The honest boundary is: you bring the container up
// once, and everything after that is in the UI.

// MTStatusResponse reports what the translation service is doing.
type MTStatusResponse struct {
	// Configured is false when `services.mt.provider` is off — a settings
	// question, not a deployment one, and the client says so differently.
	Configured bool `json:"configured"`
	// Reachable is false when the service is not answering at all. With
	// Configured true this is the "not deployed / not running" case.
	Reachable bool `json:"reachable"`
	// State is the service's own: "loading", "ready" or "failed". Empty when
	// unreachable, because then we genuinely do not know.
	State           string `json:"state,omitempty"`
	Detail          string `json:"detail,omitempty"`
	Model           string `json:"model,omitempty"`
	DownloadedBytes int64  `json:"downloadedBytes,omitempty"`
	TotalBytes      int64  `json:"totalBytes,omitempty"`
}

func mtStatusFrom(st mt.Status) MTStatusResponse {
	return MTStatusResponse{
		Configured: true, Reachable: true,
		State: st.State, Detail: st.Detail, Model: st.Model,
		DownloadedBytes: st.DownloadedBytes, TotalBytes: st.TotalBytes,
	}
}

// handleMTStatus answers for the Settings row, and is polled while loading.
//
// Any-authenticated rather than admin-only: this is a read of a service state,
// the same thing `/api/capabilities` already exposes to every signed-in user,
// and a learner who cannot see why translation is unavailable is back to
// guessing.
func (s *Server) handleMTStatus(c fiber.Ctx) error {
	if _, err := mustUser(c); err != nil {
		return err
	}
	client := mt.New(s.service("mt"))
	if client == nil {
		return c.JSON(MTStatusResponse{})
	}
	st, err := client.Status(c.Context())
	if err != nil {
		// Not an error response. "Configured but not answering" is a state the
		// screen has to render, not a request that failed.
		if errors.Is(err, mt.ErrOff) {
			return c.JSON(MTStatusResponse{})
		}
		return c.JSON(MTStatusResponse{Configured: true})
	}
	return c.JSON(mtStatusFrom(st))
}

// handleMTWarmup asks the service to fetch its model, or to retry after a
// failure. Admin-only, matching `PUT /api/settings`: it costs bandwidth and
// disk on a shared install, so it is not a per-learner button.
func (s *Server) handleMTWarmup(c fiber.Ctx) error {
	if _, err := mustUser(c); err != nil {
		return err
	}
	client := mt.New(s.service("mt"))
	if client == nil {
		// The same i18n key the reader's translate button uses, so the two
		// screens cannot describe the same configuration differently.
		return fiber.NewError(fiber.StatusServiceUnavailable, "translation.notConfigured")
	}
	st, err := client.Warmup(c.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "translation.unreachable")
	}
	return c.JSON(mtStatusFrom(st))
}
