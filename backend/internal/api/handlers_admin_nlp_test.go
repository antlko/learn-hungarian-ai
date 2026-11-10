package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"tanulom/internal/config"
	"testing"
)

func fakeNLP(t *testing.T, status string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var warm atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/warmup" {
			warm.Add(1)
			status = "downloading"
		}
		w.WriteHeader(503)
		json.NewEncoder(w).Encode(map[string]any{"status": status, "progress": 0.5, "model": "hu_core_news_md", "version": "3.8.1"})
	}))
	t.Cleanup(srv.Close)
	return srv, &warm
}

func nlpRow(t *testing.T, c *client) ComponentDTO {
	t.Helper()
	var rows []ComponentDTO
	if resp := c.json(http.MethodGet, "/api/admin/components", nil, &rows); resp.StatusCode != 200 {
		t.Fatalf("components: %d", resp.StatusCode)
	}
	for _, r := range rows {
		if r.ID == "nlp" {
			return r
		}
	}
	t.Fatal("no nlp row")
	return ComponentDTO{}
}

func TestNLPComponentIsBuiltAndReportsDownloadProgress(t *testing.T) {
	t.Parallel()
	fake, _ := fakeNLP(t, "downloading")
	s, c := setupWithMT(t, "")
	if r := nlpRow(t, c); !r.Built || r.Configured || r.State != "off" {
		t.Fatalf("off: %+v", r)
	}
	if _, err := s.updateConfig(func(cfg *config.Config) { cfg.Services.NLP = config.Service{Provider: "local", Endpoint: fake.URL} }); err != nil {
		t.Fatal(err)
	}
	s.capCache.mu.Lock()
	s.capCache.result = nil
	s.capCache.mu.Unlock()
	r := nlpRow(t, c)
	if r.State != "loading" || r.Progress != 0.5 || r.Model != "hu_core_news_md" || !r.Reachable {
		t.Fatalf("downloading: %+v", r)
	}
}

func TestNLPWarmup(t *testing.T) {
	t.Parallel()
	s, c := setupWithMT(t, "")
	if resp := c.json(http.MethodPost, "/api/admin/nlp/warmup", nil, nil); resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("not configured: %d", resp.StatusCode)
	}
	fake, warm := fakeNLP(t, "error")
	if _, err := s.updateConfig(func(cfg *config.Config) { cfg.Services.NLP = config.Service{Provider: "local", Endpoint: fake.URL} }); err != nil {
		t.Fatal(err)
	}
	var out ComponentDTO
	if resp := c.json(http.MethodPost, "/api/admin/nlp/warmup", nil, &out); resp.StatusCode != 200 {
		t.Fatalf("warmup: %d", resp.StatusCode)
	}
	if warm.Load() != 1 || out.State != "loading" {
		t.Fatalf("warm=%d out=%+v", warm.Load(), out)
	}
}
