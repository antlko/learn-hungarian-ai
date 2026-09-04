package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"tanulom/internal/config"
)

// A stand-in for services/mt. The real one loads a 600M-parameter model; what
// these tests are about is the contract between the two, which is where the
// failures that would be invisible in production live — a short batch pairing
// one sentence with another's translation, or a cache that drops the model and
// makes the "machine translation" label unrenderable on the second view.
type fakeMT struct {
	*httptest.Server
	calls atomic.Int32
	// short makes the sidecar answer with one fewer translation than asked
	// for, which is the failure the Go client must refuse to interpret.
	short atomic.Bool
}

func newFakeMT(t *testing.T) *fakeMT {
	t.Helper()
	f := &fakeMT{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		f.calls.Add(1)
		var req struct {
			Texts  []string `json:"texts"`
			Target string   `json:"target"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		out := make([]string, 0, len(req.Texts))
		for _, s := range req.Texts {
			out = append(out, req.Target+": "+s)
		}
		if f.short.Load() && len(out) > 0 {
			out = out[:len(out)-1]
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"texts": out, "model": "test/nllb-200"})
	}))
	t.Cleanup(f.Close)
	return f
}

func setupWithMT(t *testing.T, endpoint string) (*Server, *client) {
	t.Helper()
	s, c := newTestServer(t)
	c.json(http.MethodPost, "/api/setup", SetupRequest{Email: "anna@example.com", Password: goodPassword}, nil)
	if endpoint == "" {
		return s, c
	}
	if _, err := s.updateConfig(func(cfg *config.Config) {
		cfg.Services.MT = config.Service{Provider: config.ProviderLocal, Endpoint: endpoint}
	}); err != nil {
		t.Fatalf("configure mt: %v", err)
	}
	return s, c
}

func TestTranslateLabelsAndCaches(t *testing.T) {
	fake := newFakeMT(t)
	_, c := setupWithMT(t, fake.URL)

	var got TranslateResponse
	resp := c.json(http.MethodPost, "/api/translate",
		TranslateRequest{Texts: []string{"Reggel hét órakor felkelek."}, Target: "ru"}, &got)
	if resp.StatusCode != 200 {
		t.Fatalf("translate: got %d", resp.StatusCode)
	}
	if len(got.Translations) != 1 {
		t.Fatalf("want 1 translation, got %d", len(got.Translations))
	}
	tr := got.Translations[0]
	if tr.Text != "rus_Cyrl: Reggel hét órakor felkelek." {
		t.Errorf("unexpected translation %q", tr.Text)
	}
	// The model is what makes the "machine translation" label truthful. Without
	// it the UI can only say "a machine wrote this" and not which one.
	if tr.Model != "test/nllb-200" {
		t.Errorf("the model must travel with the text, got %q", tr.Model)
	}
	if tr.Cached {
		t.Error("first ask cannot be a cache hit")
	}

	// Same sentence again: served from the cache, model intact, model not woken.
	got = TranslateResponse{}
	c.json(http.MethodPost, "/api/translate",
		TranslateRequest{Texts: []string{"Reggel hét órakor felkelek."}, Target: "ru"}, &got)
	if !got.Translations[0].Cached {
		t.Error("second ask should be a cache hit")
	}
	if got.Translations[0].Model != "test/nllb-200" {
		t.Errorf("a cached row that forgot its model cannot be labelled honestly, got %q",
			got.Translations[0].Model)
	}
	if n := fake.calls.Load(); n != 1 {
		t.Errorf("the sidecar should have been asked once, was asked %d times", n)
	}

	// A different target language is a different question.
	got = TranslateResponse{}
	c.json(http.MethodPost, "/api/translate",
		TranslateRequest{Texts: []string{"Reggel hét órakor felkelek."}, Target: "en"}, &got)
	if got.Translations[0].Cached {
		t.Error("the ru cache must not answer an en request")
	}
}

func TestTranslateDeduplicatesWithinABatch(t *testing.T) {
	fake := newFakeMT(t)
	_, c := setupWithMT(t, fake.URL)

	var got TranslateResponse
	c.json(http.MethodPost, "/api/translate", TranslateRequest{
		Texts:  []string{"Igen.", "Nem.", "Igen."},
		Target: "ru",
	}, &got)

	if len(got.Translations) != 3 {
		t.Fatalf("every input needs an answer in its own position, got %d", len(got.Translations))
	}
	if got.Translations[0].Text != got.Translations[2].Text {
		t.Errorf("the same sentence got two different answers: %q vs %q",
			got.Translations[0].Text, got.Translations[2].Text)
	}
	if got.Translations[1].Text != "rus_Cyrl: Nem." {
		t.Errorf("answers are out of order: %q", got.Translations[1].Text)
	}
}

// The one sidecar failure that would look like success: a short batch silently
// shifts every later translation onto the wrong sentence.
func TestTranslateRefusesAShortBatch(t *testing.T) {
	fake := newFakeMT(t)
	fake.short.Store(true)
	_, c := setupWithMT(t, fake.URL)

	resp, body := c.do(http.MethodPost, "/api/translate",
		TranslateRequest{Texts: []string{"Igen.", "Nem."}, Target: "ru"})
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503 on a mismatched batch, got %d (%s)", resp.StatusCode, body)
	}
}

func TestTranslateWithoutAServiceSaysSo(t *testing.T) {
	_, c := setupWithMT(t, "")

	var e struct {
		Error string `json:"error"`
	}
	resp, body := c.do(http.MethodPost, "/api/translate",
		TranslateRequest{Texts: []string{"Igen."}, Target: "ru"})
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503 with no translator configured, got %d (%s)", resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, &e); err != nil || e.Error != "translation.notConfigured" {
		t.Errorf("the client needs a key it can localise, got %s", body)
	}
}

// A learner who already paid for a sentence keeps it when the sidecar stops.
func TestTranslateServesCacheWithTheServiceStopped(t *testing.T) {
	fake := newFakeMT(t)
	s, c := setupWithMT(t, fake.URL)

	var got TranslateResponse
	c.json(http.MethodPost, "/api/translate", TranslateRequest{Texts: []string{"Igen."}, Target: "ru"}, &got)
	if got.Translations[0].Text == "" {
		t.Fatal("setup: nothing was translated")
	}

	if _, err := s.updateConfig(func(cfg *config.Config) {
		cfg.Services.MT = config.Service{Provider: config.ProviderOff}
	}); err != nil {
		t.Fatalf("switch mt off: %v", err)
	}

	got = TranslateResponse{}
	resp := c.json(http.MethodPost, "/api/translate", TranslateRequest{Texts: []string{"Igen."}, Target: "ru"}, &got)
	if resp.StatusCode != 200 {
		t.Fatalf("a fully cached request needs no service, got %d", resp.StatusCode)
	}
	if !got.Translations[0].Cached {
		t.Error("this could only have come from the cache")
	}
}

func TestTranslateRejectsBadInput(t *testing.T) {
	fake := newFakeMT(t)
	_, c := setupWithMT(t, fake.URL)

	tests := []struct {
		name string
		req  TranslateRequest
	}{
		{"empty", TranslateRequest{Texts: []string{"  ", ""}, Target: "ru"}},
		{"unsupported target", TranslateRequest{Texts: []string{"Igen."}, Target: "hu"}},
		{"too many", TranslateRequest{Texts: make([]string, maxTranslateTexts+1), Target: "ru"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := range tt.req.Texts {
				if tt.req.Texts[i] == "" && tt.name == "too many" {
					tt.req.Texts[i] = "Igen."
				}
			}
			resp, body := c.do(http.MethodPost, "/api/translate", tt.req)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("want 400, got %d (%s)", resp.StatusCode, body)
			}
		})
	}
}

// The translator speaks its own protocol, so offering `openai` for it would
// advertise a wire format the client does not talk. [ADR-014]
func TestMTRejectsTheOpenAIProvider(t *testing.T) {
	s, _ := setupWithMT(t, "")
	_, err := s.updateConfig(func(cfg *config.Config) {
		cfg.Services.MT = config.Service{Provider: config.ProviderOpenAI, Endpoint: "http://example.invalid"}
	})
	if err == nil {
		t.Fatal("services.mt should refuse provider openai")
	}
}
