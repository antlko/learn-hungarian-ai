package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"tanulom/internal/config"
)

// The llm row verifies the model, not just the socket. «на связи» over an
// endpoint that refuses every request — because the config named no model —
// is how the first learner's tutor failed with nothing on the settings screen
// to point at.
func TestCapabilitiesVerifyTheLLMModelAgainstTheEndpoint(t *testing.T) {
	t.Parallel()
	srv, c := newTestServer(t)
	c.json(http.MethodPost, "/api/setup", SetupRequest{Email: "a@example.com", Password: goodPassword}, nil)

	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r) // Ollama has no /health; a 404 still proves it listens
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "qwen3:1.7b"}, {"id": "gemma3:4b"}},
		})
	}))
	t.Cleanup(ollama.Close)

	llmRow := func() CapabilityDTO {
		t.Helper()
		srv.invalidateCapabilityCache()
		var caps []CapabilityDTO
		if resp := c.json(http.MethodGet, "/api/capabilities", nil, &caps); resp.StatusCode != 200 {
			t.Fatalf("capabilities: %d", resp.StatusCode)
		}
		for _, cap := range caps {
			if cap.Name == "llm" {
				return cap
			}
		}
		t.Fatal("no llm row")
		return CapabilityDTO{}
	}
	set := func(model string) {
		t.Helper()
		if _, err := srv.updateConfig(func(cfg *config.Config) {
			cfg.Services.LLM = config.Service{Provider: config.ProviderLocal, Endpoint: ollama.URL, Model: model}
		}); err != nil {
			t.Fatal(err)
		}
	}

	set("")
	row := llmRow()
	if !row.Configured || !row.Reachable || row.Detail != detailNoModel {
		t.Errorf("blank model: want reachable with %q, got %+v", detailNoModel, row)
	}
	if !slices.Equal(row.Models, []string{"qwen3:1.7b", "gemma3:4b"}) {
		t.Errorf("the endpoint's models are offered for the picker, got %v", row.Models)
	}

	set("qwen3:8b")
	if row = llmRow(); row.Detail != detailUnknownModel || !row.Reachable {
		t.Errorf("a model the endpoint does not carry: want %q, got %+v", detailUnknownModel, row)
	}

	set("qwen3:1.7b")
	if row = llmRow(); row.Detail != "" || !row.Reachable || len(row.Models) != 2 {
		t.Errorf("a served model: want reachable with no detail, got %+v", row)
	}

	// No /v1/models at all: nothing verified, nothing claimed beyond reachable.
	mute := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(mute.Close)
	if _, err := srv.updateConfig(func(cfg *config.Config) {
		cfg.Services.LLM = config.Service{Provider: config.ProviderLocal, Endpoint: mute.URL, Model: "anything"}
	}); err != nil {
		t.Fatal(err)
	}
	if row = llmRow(); row.Detail != "" || !row.Reachable || row.Models != nil {
		t.Errorf("an endpoint without the list: want plain reachable, got %+v", row)
	}
	// Other rows never carry a model list.
	for _, cap := range srv.capCache.result {
		if cap.Name != "llm" && cap.Models != nil {
			t.Errorf("%s carries models: %+v", cap.Name, cap)
		}
	}
}

// The reason a probe failed is what tells «https:// in front of an Ollama»
// from a dead server; both used to read only «unreachable».
func TestProbeClassifiesTransportFailures(t *testing.T) {
	t.Parallel()
	svc := func(url string) config.Service {
		return config.Service{Provider: config.ProviderLocal, Endpoint: url}
	}

	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer plain.Close()
	got := probe(context.Background(), "llm", svc(strings.Replace(plain.URL, "http://", "https://", 1)), "")
	if got.Reachable || got.Detail != "unreachable" || got.Reason != reasonTLSToPlainHTTP {
		t.Errorf("plain HTTP behind https://: %+v", got)
	}

	// A server that answers the TLS hello with bytes that are not TLS at all (what
	// Python's http.server did when this was reproduced by hand): same advice.
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	go func() {
		for {
			c, err := raw.Accept()
			if err != nil {
				return
			}
			_, _ = c.Write([]byte("not tls at all\r\n\r\n"))
			_ = c.Close()
		}
	}()
	if got := probe(context.Background(), "llm", svc("https://"+raw.Addr().String()), ""); got.Reason != reasonTLSToPlainHTTP {
		t.Errorf("non-TLS bytes behind https://: %+v", got)
	}

	closed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	url := closed.URL
	closed.Close()
	if got := probe(context.Background(), "llm", svc(url), ""); got.Reason != reasonRefused {
		t.Errorf("closed port: %+v", got)
	}

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	}))
	defer slow.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if got := probe(ctx, "llm", svc(slow.URL), ""); got.Reason != reasonTimeout {
		t.Errorf("slow server: %+v", got)
	}

	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer tlsSrv.Close()
	if got := probe(context.Background(), "llm", svc(tlsSrv.URL), ""); got.Reason != reasonTLSCert {
		t.Errorf("untrusted cert: %+v", got)
	}

	if got := probe(context.Background(), "llm", svc(plain.URL), ""); got.Reason != "" {
		t.Errorf("reachable must carry no reason: %+v", got)
	}
}
