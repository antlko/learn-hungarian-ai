package nlp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tanulom/internal/config"
)

func clientFor(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(config.Service{Provider: config.ProviderLocal, Endpoint: srv.URL})
}

func TestOffClientReportsErrOff(t *testing.T) {
	for _, svc := range []config.Service{{}, {Provider: config.ProviderOff, Endpoint: "http://x"}, {Provider: "local"}} {
		c := New(svc)
		if c != nil {
			t.Fatalf("%+v must be off", svc)
		}
		if _, err := c.Analyze(context.Background(), "x"); !errors.Is(err, ErrOff) {
			t.Fatalf("want ErrOff, got %v", err)
		}
	}
}

func TestHealthStatesPassThroughNon2xx(t *testing.T) {
	for _, st := range []string{"downloading", "loading", "ready", "error"} {
		c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
			code := 503
			if st == "ready" {
				code = 200
			}
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(Health{Status: st, Progress: 0.4, Model: "hu_core_news_md", Version: "3.8.1"})
		})
		h, err := c.Health(context.Background())
		if err != nil || h.Status != st || h.Progress != 0.4 {
			t.Fatalf("%s: %+v %v", st, h, err)
		}
	}
}

func TestHealthRejectsGarbage(t *testing.T) {
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>secret-ish</html>")) })
	_, err := c.Health(context.Background())
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("want an error without the remote body, got %v", err)
	}
}

func TestAnalyzeKeepsByteOffsetsOnMultibyteText(t *testing.T) {
	text := "Őszi űrlap várában."
	// Byte offsets: Ő=2 bytes, so "Őszi" = [0,5).
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		var req analyzeReq
		json.NewDecoder(r.Body).Decode(&req)
		if req.Text != text {
			t.Errorf("sidecar got %q", req.Text)
		}
		json.NewEncoder(w).Encode(analyzeResp{Model: "m", Tokens: []Token{
			{Start: 0, End: 5, Text: "Őszi", Lemma: "őszi", UPOS: "ADJ"},
			{Start: 6, End: 12, Text: "űrlap", Lemma: "űrlap", UPOS: "NOUN"},
			{Start: 13, End: 14, Text: "x", UPOS: "X"},    // wrong text for the span
			{Start: 1, End: 5, Text: "szi", UPOS: "X"},    // inside the 2-byte Ő
			{Start: 0, End: 999, Text: "Őszi", UPOS: "X"}, // out of range
			{Start: 5, End: 3, Text: "", UPOS: "X"},       // inverted
		}})
	})
	res, err := c.Analyze(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Tokens) != 2 || res.Model != "m" {
		t.Fatalf("invalid tokens must be dropped: %+v", res)
	}
	for _, tk := range res.Tokens {
		if text[tk.Start:tk.End] != tk.Text {
			t.Fatalf("offset does not slice to %q", tk.Text)
		}
	}
}

func TestAnalyze413And5xx(t *testing.T) {
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(413); w.Write([]byte("leak")) })
	if _, err := c.Analyze(context.Background(), "x"); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("want ErrTooLarge, got %v", err)
	}
	c = clientFor(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503); w.Write([]byte("leak")) })
	_, err := c.Analyze(context.Background(), "x")
	if err == nil || strings.Contains(err.Error(), "leak") {
		t.Fatalf("5xx must error without the body: %v", err)
	}
	if _, err := c.Analyze(context.Background(), strings.Repeat("a", MaxChars+1)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversize text must not be sent: %v", err)
	}
}

func TestAnalyzeHonoursContextDeadline(t *testing.T) {
	release := make(chan struct{})
	defer close(release) // runs before the server's Close (registered earlier)
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := c.Analyze(ctx, "x"); err == nil || time.Since(start) > 2*time.Second {
		t.Fatalf("want a prompt error, got %v after %v", err, time.Since(start))
	}
}

func TestConnectionRefusedIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	c := New(config.Service{Provider: config.ProviderLocal, Endpoint: url})
	if _, err := c.Analyze(context.Background(), "x"); err == nil {
		t.Fatal("want an error")
	}
	if _, err := c.Health(context.Background()); err == nil {
		t.Fatal("want an error")
	}
}

func resetBreakers() {
	breakerMu.Lock()
	breakers = map[string]*breakerState{}
	breakerMu.Unlock()
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	var hits int
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer target.Close()
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	})
	if _, err := c.Analyze(context.Background(), "x"); err == nil {
		t.Fatal("a redirect must be an error, not a success")
	}
	if hits != 0 {
		t.Fatal("redirect was followed")
	}
}

func TestCircuitBreakerOpensAndRecovers(t *testing.T) {
	resetBreakers()
	clock := time.Now()
	now = func() time.Time { return clock }
	defer func() { now = time.Now; resetBreakers() }()

	var calls int
	fail := true
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if fail {
			w.WriteHeader(503)
			return
		}
		json.NewEncoder(w).Encode(analyzeResp{Model: "m"})
	})
	for i := 0; i < breakerThreshold; i++ {
		if _, err := c.Analyze(context.Background(), "x"); err == nil || errors.Is(err, ErrCircuitOpen) {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if _, err := c.Analyze(context.Background(), "x"); !errors.Is(err, ErrCircuitOpen) || calls != breakerThreshold {
		t.Fatalf("breaker should be open without calling: err=%v calls=%d", err, calls)
	}
	clock = clock.Add(breakerCooldown + time.Second)
	fail = false
	if _, err := c.Analyze(context.Background(), "x"); err != nil {
		t.Fatalf("after cooldown: %v", err)
	}
	// Success resets the count: two failures afterwards do not reopen it.
	fail = true
	for i := 0; i < breakerThreshold-1; i++ {
		c.Analyze(context.Background(), "x")
	}
	fail = false
	if _, err := c.Analyze(context.Background(), "x"); err != nil {
		t.Fatalf("should still be closed: %v", err)
	}
}

func Test413DoesNotTripTheBreaker(t *testing.T) {
	resetBreakers()
	defer resetBreakers()
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(413) })
	for i := 0; i < breakerThreshold+1; i++ {
		if _, err := c.Analyze(context.Background(), "x"); !errors.Is(err, ErrTooLarge) {
			t.Fatalf("call %d: %v", i, err)
		}
	}
}
