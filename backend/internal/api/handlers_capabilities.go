package api

import (
	"context"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/config"
	"tanulom/internal/llm"
)

// Capability probing. Optional services (TTS, ASR, NLP, LLM, MT) are probed so the UI
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
		"mt":  cfg.Services.MT,
	}

	// Two probes in sequence for llm (see probe), so the deadline covers both.
	ctx, cancel := context.WithTimeout(c.Context(), 2*probeTimeout+time.Second)
	defer cancel()
	llmKey := s.llmKey()

	out := make([]CapabilityDTO, 0, len(services))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, svc := range services {
		wg.Add(1)
		go func(name string, svc config.Service) {
			defer wg.Done()
			key := ""
			if name == "llm" {
				key = llmKey
			}
			dto := probe(ctx, name, svc, key)
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

// Details the llm row can carry beyond the shared ones. The client maps them
// to copy; they are part of the API (docs/API.md).
const (
	// detailNoModel: the endpoint answers but the config names no model, so
	// every request it gets will be refused. Models lists what it serves.
	detailNoModel = "no model"
	// detailUnknownModel: the endpoint lists its models and ours is not one.
	detailUnknownModel = "unknown model"
)

// probe checks one service. An unconfigured service is not an error — it reports
// configured=false and the UI simply hides the dependent features.
//
// For llm, reachable is not enough: «на связи» on a row whose requests are
// all refused is the state that stranded the first learner to plug in an
// Ollama, with a blank model and a tutor saying only «модель не ответила».
// So a reachable llm endpoint is also asked GET /v1/models — the list goes
// out for the settings screen to offer, and Detail says when the configured
// name is blank or not among them. An endpoint that does not serve the list
// (some llama.cpp builds) is left at reachable: nothing has been verified,
// and nothing is claimed beyond that.
func probe(ctx context.Context, name string, svc config.Service, key string) CapabilityDTO {
	dto := CapabilityDTO{Name: name, Provider: svc.Provider}
	if svc.Provider == config.ProviderOff {
		dto.Detail = "not configured"
		return dto
	}
	// Device speech runs in the learner's browser. It is configured, and this
	// process has no way to reach it or to know whether that browser has a
	// Hungarian voice installed — so it says exactly that and lets the client,
	// which CAN check, report what it found. Asserting reachability we have
	// not tested is the one thing this endpoint must not do.
	if svc.Provider == config.ProviderDevice {
		dto.Configured = true
		dto.Detail = "device"
		return dto
	}
	if svc.Endpoint == "" {
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
		return dto
	}
	if name == "llm" {
		probeModels(ctx, svc, key, &dto)
	}
	return dto
}

// probeModels fills Models and the model-related Detail on a reachable llm row.
func probeModels(ctx context.Context, svc config.Service, key string, dto *CapabilityDTO) {
	reqCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	models, err := llm.ListModels(reqCtx, svc, key)
	if err == nil {
		dto.Models = models
	}
	switch {
	case strings.TrimSpace(svc.Model) == "":
		dto.Detail = detailNoModel
	case err == nil && len(models) > 0 && !slices.Contains(models, svc.Model):
		dto.Detail = detailUnknownModel
	}
}

// InvalidateCapabilityCache forces the next probe to run. Called after a settings
// change so the UI does not show stale service state for up to 30s.
func InvalidateCapabilityCache() {
	capCache.mu.Lock()
	defer capCache.mu.Unlock()
	capCache.at = time.Time{}
	capCache.result = nil
}
