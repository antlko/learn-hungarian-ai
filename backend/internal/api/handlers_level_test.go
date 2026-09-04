package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tanulom/internal/config"
)

// The three level writers and their pecking order (ADR-023): heuristic at
// ingest, LLM as a labelled second opinion, the learner's word above both.

func createLevelText(t *testing.T, c *client) int64 {
	t.Helper()
	var created struct {
		ID          int64  `json:"id"`
		LevelSource string `json:"levelSource"`
	}
	if resp := c.json(http.MethodPost, "/api/texts", map[string]any{
		"title": "teszt", "body": "A házban van a könyv. A ház nagy.", "kind": "plain",
	}, &created); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create text: %d", resp.StatusCode)
	}
	if created.LevelSource != "heuristic" {
		t.Fatalf("ingest did not estimate a level: source %q", created.LevelSource)
	}
	return created.ID
}

func TestManualLevelRoundTripsAndBeatsTheMachine(t *testing.T) {
	_, c := newPackedServer(t)
	id := createLevelText(t, c)

	var text struct {
		Level       string `json:"level"`
		LevelSource string `json:"levelSource"`
	}
	if resp := c.json(http.MethodPut, "/api/texts/"+itoa(id)+"/level",
		map[string]any{"level": "B1"}, &text); resp.StatusCode != http.StatusOK {
		t.Fatalf("manual level: %d", resp.StatusCode)
	}
	if text.Level != "B1" || text.LevelSource != "manual" {
		t.Fatalf("manual level stored as %q/%q", text.Level, text.LevelSource)
	}

	if resp := c.json(http.MethodPut, "/api/texts/"+itoa(id)+"/level",
		map[string]any{"level": "B1-B2"}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("an off-enum level was accepted: %d", resp.StatusCode)
	}
}

func TestLLMLevelIsOffByDefaultAndValidatedWhenOn(t *testing.T) {
	srv, c := newPackedServer(t)
	id := createLevelText(t, c)

	// provider off → an honest 503, not a hang or a guess.
	if resp := c.json(http.MethodPost, "/api/texts/"+itoa(id)+"/level", nil, nil); resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("llm off answered %d, want 503", resp.StatusCode)
	}

	// A fake OpenAI endpoint that answers a fenced-but-parseable verdict.
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant",
			"content":"Sure! {\"level\":\"A2\",\"confidence\":0.8,\"rationale\":\"simple sentences\"}"}}]}`))
	}))
	defer fake.Close()

	if _, err := srv.updateConfig(func(cfg *config.Config) {
		cfg.Services.LLM.Provider = "openai"
		cfg.Services.LLM.Endpoint = fake.URL
		cfg.Services.LLM.Model = "test-model"
	}); err != nil {
		t.Fatalf("configure llm: %v", err)
	}

	var text struct {
		Level       string `json:"level"`
		LevelSource string `json:"levelSource"`
		LevelDetail string `json:"levelDetail"`
	}
	if resp := c.json(http.MethodPost, "/api/texts/"+itoa(id)+"/level", nil, &text); resp.StatusCode != http.StatusOK {
		t.Fatalf("llm level: %d", resp.StatusCode)
	}
	if text.Level != "A2" || text.LevelSource != "llm" {
		t.Fatalf("llm level stored as %q/%q", text.Level, text.LevelSource)
	}
	if !strings.Contains(text.LevelDetail, "test-model") {
		t.Errorf("the model name must travel in level_detail, got %q", text.LevelDetail)
	}

	// Garbage leaves the stored verdict untouched.
	garbage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"intermediate-ish"}}]}`))
	}))
	defer garbage.Close()
	if _, err := srv.updateConfig(func(cfg *config.Config) {
		cfg.Services.LLM.Endpoint = garbage.URL
	}); err != nil {
		t.Fatal(err)
	}
	if resp := c.json(http.MethodPost, "/api/texts/"+itoa(id)+"/level", nil, nil); resp.StatusCode != http.StatusBadGateway {
		t.Errorf("garbage verdict answered %d, want 502", resp.StatusCode)
	}
	var detail struct {
		Text struct {
			Level       string `json:"level"`
			LevelSource string `json:"levelSource"`
		} `json:"text"`
	}
	c.json(http.MethodGet, "/api/texts/"+itoa(id), nil, &detail)
	if detail.Text.Level != "A2" || detail.Text.LevelSource != "llm" {
		t.Errorf("a failed call rewrote the stored level: %q/%q",
			detail.Text.Level, detail.Text.LevelSource)
	}
}
