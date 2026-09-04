package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"tanulom/internal/config"
)

// A blank model is a client that can only fail: Ollama and the hosted APIs
// refuse the request, and the failure used to surface as «модель не ответила»
// under a settings row reading «на связи».
func TestNewRefusesABlankModelSeparatelyFromOff(t *testing.T) {
	if _, err := New(config.Service{Provider: config.ProviderOff}, ""); !errors.Is(err, ErrOff) {
		t.Errorf("off: got %v, want ErrOff", err)
	}
	if _, err := New(config.Service{Provider: config.ProviderLocal, Endpoint: "http://x"}, ""); !errors.Is(err, ErrNoModel) {
		t.Errorf("no model: got %v, want ErrNoModel", err)
	}
	if _, err := New(config.Service{Provider: config.ProviderLocal, Endpoint: "http://x", Model: " "}, ""); !errors.Is(err, ErrNoModel) {
		t.Errorf("blank model: got %v, want ErrNoModel", err)
	}
	c, err := New(config.Service{Provider: config.ProviderLocal, Endpoint: "http://x/", Model: "qwen3:1.7b"}, "")
	if err != nil || c.Model() != "qwen3:1.7b" || c.endpoint != "http://x" {
		t.Errorf("named model: %+v %v", c, err)
	}
}

// capture is an OpenAI-shaped endpoint that records the request body and
// answers a one-word stream, plus a model list.
func capture(t *testing.T, models ...string) (*httptest.Server, func() map[string]any) {
	t.Helper()
	var mu sync.Mutex
	var last map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			list := modelList{}
			for _, m := range models {
				list.Data = append(list.Data, struct {
					ID string `json:"id"`
				}{ID: m})
			}
			_ = json.NewEncoder(w).Encode(list)
		case "/v1/chat/completions":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			last = body
			mu.Unlock()
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"ok"}}]}` + "\n\n" + "data: [DONE]\n\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, func() map[string]any {
		mu.Lock()
		defer mu.Unlock()
		return last
	}
}

// A local server is asked not to think: a thinking model spends the answer's
// max_tokens on hidden reasoning first. A hosted API is left at its default,
// because a non-reasoning hosted model rejects the field.
func TestLocalRequestsCarryReasoningEffortNoneAndHostedDoNot(t *testing.T) {
	srv, last := capture(t)
	for _, tc := range []struct {
		provider string
		want     any
	}{
		{config.ProviderLocal, "none"},
		{config.ProviderOpenAI, nil},
	} {
		c, err := New(config.Service{Provider: tc.provider, Endpoint: srv.URL, Model: "m"}, "")
		if err != nil {
			t.Fatal(err)
		}
		var got string
		_, err = c.Stream(context.Background(), []Message{{Role: "user", Content: "hi"}}, 10, 0.3,
			func(d string) error { got += d; return nil })
		if err != nil || got != "ok" {
			t.Fatalf("%s: stream %q %v", tc.provider, got, err)
		}
		// The context meter needs the figures: every stream asks for them.
		if so, _ := last()["stream_options"].(map[string]any); so["include_usage"] != true {
			t.Errorf("%s: stream_options = %v, want include_usage", tc.provider, last()["stream_options"])
		}
		if v := last()["reasoning_effort"]; v != tc.want {
			t.Errorf("%s: reasoning_effort = %v, want %v", tc.provider, v, tc.want)
		}
		if v := last()["model"]; v != "m" {
			t.Errorf("%s: model = %v, want m", tc.provider, v)
		}
	}
}

func TestListModelsReadsTheEndpointsList(t *testing.T) {
	srv, _ := capture(t, "qwen3:1.7b", "llama3:8b")
	got, err := ListModels(context.Background(), config.Service{Provider: config.ProviderLocal, Endpoint: srv.URL}, "")
	if err != nil || len(got) != 2 || got[0] != "qwen3:1.7b" {
		t.Errorf("got %v %v", got, err)
	}
	if _, err := ListModels(context.Background(), config.Service{Provider: config.ProviderOff}, ""); !errors.Is(err, ErrOff) {
		t.Errorf("off: %v", err)
	}
	// An endpoint without the route is an error, not an empty list: nothing
	// was verified, and the probe must not say the model is unknown.
	none := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(none.Close)
	if _, err := ListModels(context.Background(), config.Service{Provider: config.ProviderLocal, Endpoint: none.URL}, ""); err == nil {
		t.Error("a 404 on /v1/models should be an error")
	}
}

// A model that stops because max_tokens ran out has not finished; the caller
// marks the answer as cut rather than passing it off as complete (ADR-030 —
// a translation breakdown is the answer most likely to hit the budget). The
// text delivered before the cut still arrives in full.
func TestStreamReportsACutByMaxTokens(t *testing.T) {
	for _, tc := range []struct {
		reason string
		want   error
	}{
		{"stop", nil},
		{"length", ErrTruncated},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"Eljöttem "},"finish_reason":null}]}` + "\n\n" +
				`data: {"choices":[{"delta":{"content":"ebbe"},"finish_reason":"` + tc.reason + `"}]}` + "\n\n" +
				"data: [DONE]\n\n"))
		}))
		c, err := New(config.Service{Provider: config.ProviderLocal, Endpoint: srv.URL, Model: "m"}, "")
		if err != nil {
			t.Fatal(err)
		}
		var got string
		_, err = c.Stream(context.Background(), []Message{{Role: "user", Content: "hi"}}, 10, 0.3,
			func(d string) error { got += d; return nil })
		srv.Close()
		if !errors.Is(err, tc.want) {
			t.Errorf("finish_reason %q: err = %v, want %v", tc.reason, err, tc.want)
		}
		if got != "Eljöttem ebbe" {
			t.Errorf("finish_reason %q: the delivered text must be whole either way, got %q", tc.reason, got)
		}
	}
}

// The usage figures ride on a final chunk with no choices (the shape Ollama
// and OpenAI send); they come back beside the text, and beside ErrTruncated
// too — a cut answer's figures are the ones the meter needs most.
func TestStreamReturnsTheServersUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"Szia"},"finish_reason":null}]}` + "\n\n" +
			`data: {"choices":[{"delta":{},"finish_reason":"length"}]}` + "\n\n" +
			`data: {"choices":[],"usage":{"prompt_tokens":3210,"completion_tokens":640,"total_tokens":3850}}` + "\n\n" +
			"data: [DONE]\n\n"))
	}))
	defer srv.Close()
	c, err := New(config.Service{Provider: config.ProviderLocal, Endpoint: srv.URL, Model: "m"}, "")
	if err != nil {
		t.Fatal(err)
	}
	var got string
	usage, err := c.Stream(context.Background(), []Message{{Role: "user", Content: "hi"}}, 10, 0.3,
		func(d string) error { got += d; return nil })
	if !errors.Is(err, ErrTruncated) || got != "Szia" {
		t.Fatalf("stream = %q, %v", got, err)
	}
	if usage != (Usage{PromptTokens: 3210, CompletionTokens: 640}) {
		t.Errorf("usage = %+v", usage)
	}
}

// The window comes from Ollama's /api/ps, for the running model only, and is
// 0 — "not reported" — everywhere else: a hosted provider is never asked, and
// a server without the route, or without the model loaded, says nothing.
func TestContextLengthReadsOllamaPs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ps" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"models":[{"name":"gemma3:4b","model":"gemma3:4b","context_length":8192}]}`))
	}))
	defer srv.Close()
	ctx := context.Background()
	if got := ContextLength(ctx, config.Service{Provider: config.ProviderLocal, Endpoint: srv.URL}, "gemma3:4b"); got != 8192 {
		t.Errorf("running model: %d, want 8192", got)
	}
	if got := ContextLength(ctx, config.Service{Provider: config.ProviderLocal, Endpoint: srv.URL}, "qwen3:8b"); got != 0 {
		t.Errorf("a model that is not running: %d, want 0", got)
	}
	if got := ContextLength(ctx, config.Service{Provider: config.ProviderOpenAI, Endpoint: srv.URL}, "gemma3:4b"); got != 0 {
		t.Errorf("a hosted provider is never asked: %d, want 0", got)
	}
	if got := ContextLength(ctx, config.Service{Provider: config.ProviderLocal, Endpoint: "http://127.0.0.1:1"}, "gemma3:4b"); got != 0 {
		t.Errorf("an unreachable server: %d, want 0", got)
	}
}
