package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/config"
	"tanulom/internal/llm"
)

// Capability probing. Optional services (TTS, ASR, LLM, MT) are probed so the UI
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

func (s *Server) handleCapabilities(c fiber.Ctx) error {
	return c.JSON(s.capabilities(c.Context()))
}

// capabilities probes every optional service, cached for probeCacheTTL. Shared
// by /api/capabilities and /api/admin/components so the two cannot disagree.
func (s *Server) capabilities(parent context.Context) []CapabilityDTO {
	s.capCache.mu.Lock()
	defer s.capCache.mu.Unlock()

	if time.Since(s.capCache.at) < probeCacheTTL && s.capCache.result != nil {
		return s.capCache.result
	}

	cfg := s.Config()
	services := map[string]config.Service{
		"tts": cfg.Services.TTS,
		"asr": cfg.Services.ASR,
		"llm": cfg.Services.LLM,
		"mt":  cfg.Services.MT,
	}
	// A managed service has no stored endpoint; its port is resolved here, by
	// the same function every client uses (resolveService).
	managed := map[string]bool{}
	stored := make(map[string]config.Service, len(services))
	for name, svc := range services {
		stored[name] = svc
		if svc.Provider == config.ProviderManaged && !svc.Paused {
			managed[name] = true
		}
		services[name] = s.resolveService(name, svc)
	}

	// Two probes in sequence for llm (see probe), so the deadline covers both.
	ctx, cancel := context.WithTimeout(parent, 2*probeTimeout+time.Second)
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
			if stored[name].Paused {
				dto := CapabilityDTO{Name: name, Provider: stored[name].Provider, Detail: "paused", Paused: true}
				mu.Lock()
				out = append(out, dto)
				mu.Unlock()
				return
			}
			dto := probe(ctx, name, svc, key)
			if managed[name] {
				dto.Provider = config.ProviderManaged // the resolved view says "local"
			}
			mu.Lock()
			out = append(out, dto)
			mu.Unlock()
		}(name, svc)
	}
	wg.Wait()

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	s.capCache.at, s.capCache.result = time.Now(), out
	return out
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
	// Managed and no endpoint: selected, but the child is not running (stopped,
	// starting, failed or still installing). Configured, not reachable — and not
	// an error: the Components card has the state machine.
	if svc.Provider == config.ProviderManaged && svc.Endpoint == "" {
		dto.Configured = true
		dto.Detail = "not running"
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
		dto.Reason = classifyProbeError(err)
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

// Reasons a probe could not connect, carried in CapabilityDTO.Reason. Machine
// codes only: the client maps them to a sentence, and neither the raw error
// nor the URL/IP it names ever reaches the DTO.
const (
	reasonTLSToPlainHTTP = "tlsToPlainHttp"
	reasonRefused        = "refused"
	reasonTimeout        = "timeout"
	reasonDNS            = "dns"
	reasonTLSCert        = "tlsCert"
	reasonUnreachable    = "unreachable"
)

// classifyProbeError turns a transport error into one of the reason codes. The
// common home mistake — https:// in front of an Ollama, which speaks plain HTTP —
// is only visible in the error text; net/http exposes no typed error for it.
func classifyProbeError(err error) string {
	if err == nil {
		return ""
	}
	var (
		dnsErr   *net.DNSError
		certErr  *tls.CertificateVerificationError
		unknown  x509.UnknownAuthorityError
		hostErr  x509.HostnameError
		invalid  x509.CertificateInvalidError
		recordHd tls.RecordHeaderError
		netErr   net.Error
	)
	switch {
	case strings.Contains(err.Error(), "server gave HTTP response to HTTPS client"),
		// The peer answered a TLS hello with something that is not TLS at all
		// (Go reports tls.RecordHeaderError): in practice a plain-HTTP server
		// such as Ollama behind an https:// address, whatever bytes it replied.
		errors.As(err, &recordHd):
		return reasonTLSToPlainHTTP
	case errors.Is(err, syscall.ECONNREFUSED):
		return reasonRefused
	case errors.Is(err, context.DeadlineExceeded):
		return reasonTimeout
	case errors.As(err, &dnsErr):
		return reasonDNS
	case errors.As(err, &certErr), errors.As(err, &unknown), errors.As(err, &hostErr),
		errors.As(err, &invalid):
		return reasonTLSCert
	case errors.As(err, &netErr) && netErr.Timeout():
		return reasonTimeout
	}
	return reasonUnreachable
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

// invalidateCapabilityCache forces the next probe to run. Called after a settings
// change so the UI does not show stale service state for up to 30s.
func (s *Server) invalidateCapabilityCache() {
	s.capCache.mu.Lock()
	defer s.capCache.mu.Unlock()
	s.capCache.at = time.Time{}
	s.capCache.result = nil
}
