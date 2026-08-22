package api

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/config"
)

// Capability probing. Optional services (TTS, ASR, NLP, LLM) are probed so the UI
// can show their state and disable dependent features with a visible reason.
//
// The invariant this supports: the core must remain fully usable with every
// sidecar stopped. A feature that needs an unreachable service is disabled and
// says so — never silently missing. See ADR-001, ADR-005.

const (
	probeTimeout   = 2 * time.Second
	probeCacheTTL  = 30 * time.Second
	healthEndpoint = "/health"
)

type capabilityCache struct {
	mu     sync.Mutex
	at     time.Time
	result []CapabilityDTO
}

var capCache capabilityCache

func (s *Server) handleCapabilities(c fiber.Ctx) error {
	capCache.mu.Lock()
	defer capCache.mu.Unlock()

	if time.Since(capCache.at) < probeCacheTTL && capCache.result != nil {
		return c.JSON(capCache.result)
	}

	cfg := s.Config()
	services := map[string]config.Service{
		"tts": cfg.Services.TTS,
		"asr": cfg.Services.ASR,
		"nlp": cfg.Services.NLP,
		"llm": cfg.Services.LLM,
	}

	ctx, cancel := context.WithTimeout(c.Context(), probeTimeout+time.Second)
	defer cancel()

	out := make([]CapabilityDTO, 0, len(services))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, svc := range services {
		wg.Add(1)
		go func(name string, svc config.Service) {
			defer wg.Done()
			dto := probe(ctx, name, svc)
			mu.Lock()
			out = append(out, dto)
			mu.Unlock()
		}(name, svc)
	}
	wg.Wait()

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	capCache.at, capCache.result = time.Now(), out
	return c.JSON(out)
}

// probe checks one service. An unconfigured service is not an error — it reports
// configured=false and the UI simply hides the dependent features.
func probe(ctx context.Context, name string, svc config.Service) CapabilityDTO {
	dto := CapabilityDTO{Name: name, Provider: svc.Provider}
	if svc.Provider == config.ProviderOff || svc.Endpoint == "" {
		dto.Detail = "not configured"
		return dto
	}
	dto.Configured = true

	reqCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, svc.Endpoint+healthEndpoint, nil)
	if err != nil {
		dto.Detail = "bad endpoint"
		return dto
	}

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		dto.Detail = "unreachable"
		return dto
	}
	defer resp.Body.Close()

	dto.LatencyMs = time.Since(start).Milliseconds()
	// Any non-5xx answer means something is listening and willing to talk. A 404
	// on /health from a service that has no such route still proves reachability.
	if resp.StatusCode < 500 {
		dto.Reachable = true
	} else {
		dto.Detail = "unhealthy"
	}
	return dto
}

// InvalidateCapabilityCache forces the next probe to run. Called after a settings
// change so the UI does not show stale service state for up to 30s.
func InvalidateCapabilityCache() {
	capCache.mu.Lock()
	defer capCache.mu.Unlock()
	capCache.at = time.Time{}
	capCache.result = nil
}
