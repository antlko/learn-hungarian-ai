package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// newTestServer returns a server backed by a temp data dir, plus a request helper
// that carries the session cookie between calls.
func newTestServer(t *testing.T) (*Server, *client) {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, &client{t: t, app: s}
}

type client struct {
	t      *testing.T
	app    *Server
	cookie string
}

func (c *client) do(method, path string, body any) (*http.Response, []byte) {
	c.t.Helper()

	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}

	// bcrypt hashing makes setup/login slower than the 1s default.
	resp, err := c.app.App().Test(req, fiber.TestConfig{Timeout: 15 * time.Second, FailOnTimeout: true})
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	// Capture any Set-Cookie so subsequent calls are authenticated.
	if sc := resp.Header.Get("Set-Cookie"); sc != "" {
		if name, _, ok := strings.Cut(sc, ";"); ok || name != "" {
			c.cookie = name
		}
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, data
}

func (c *client) json(method, path string, body any, out any) *http.Response {
	c.t.Helper()
	resp, data := c.do(method, path, body)
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			c.t.Fatalf("%s %s: decode %q: %v", method, path, data, err)
		}
	}
	return resp
}

const goodPassword = "elég hosszú jelszó"

func TestBootstrapAndSetupFlow(t *testing.T) {
	_, c := newTestServer(t)

	var boot BootstrapDTO
	if resp := c.json(http.MethodGet, "/api/bootstrap", nil, &boot); resp.StatusCode != 200 {
		t.Fatalf("bootstrap: status %d", resp.StatusCode)
	}
	if !boot.NeedsSetup {
		t.Error("fresh install should need setup")
	}
	if boot.PackPresent {
		t.Error("fresh install should have no language pack")
	}
	if len(boot.Locales) != 2 {
		t.Errorf("want both locales offered, got %v", boot.Locales)
	}

	var me MeDTO
	resp := c.json(http.MethodPost, "/api/setup", SetupRequest{
		Email: "anna@example.com", Password: goodPassword, UILocale: "ru", GlossLang: "both",
	}, &me)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup: status %d", resp.StatusCode)
	}
	if me.User.Role != "admin" {
		t.Errorf("first user should be admin, got %q", me.User.Role)
	}
	if me.Settings.Locale.Gloss != "both" {
		t.Errorf("wizard gloss choice not persisted: %q", me.Settings.Locale.Gloss)
	}

	// Setup is one-shot: it must not become an account-creation endpoint.
	resp, _ = c.do(http.MethodPost, "/api/setup", SetupRequest{
		Email: "mallory@example.com", Password: goodPassword,
	})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("second setup: want 409, got %d", resp.StatusCode)
	}

	if resp := c.json(http.MethodGet, "/api/bootstrap", nil, &boot); resp.StatusCode != 200 {
		t.Fatal("bootstrap after setup failed")
	}
	if boot.NeedsSetup {
		t.Error("setup should no longer be needed")
	}
}

func TestSetupRejectsWeakInput(t *testing.T) {
	tests := []struct {
		name string
		req  SetupRequest
	}{
		{"short password", SetupRequest{Email: "a@example.com", Password: "short"}},
		{"bad email", SetupRequest{Email: "not-an-email", Password: goodPassword}},
		{"empty email", SetupRequest{Email: "", Password: goodPassword}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, c := newTestServer(t)
			resp, body := c.do(http.MethodPost, "/api/setup", tt.req)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("want 400, got %d (%s)", resp.StatusCode, body)
			}
			var e struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(body, &e); err != nil || e.Error == "" {
				t.Errorf("error body should be {\"error\":...}, got %s", body)
			}
		})
	}
}

func TestAuthIsRequired(t *testing.T) {
	_, c := newTestServer(t)
	for _, path := range []string{"/api/me", "/api/settings", "/api/capabilities"} {
		resp, _ := c.do(http.MethodGet, path, nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s without session: want 401, got %d", path, resp.StatusCode)
		}
	}
}

func TestLoginLogout(t *testing.T) {
	_, c := newTestServer(t)
	c.json(http.MethodPost, "/api/setup", SetupRequest{Email: "anna@example.com", Password: goodPassword}, nil)

	// Logging out must invalidate the session.
	if resp, _ := c.do(http.MethodPost, "/api/logout", nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("logout: got %d", resp.StatusCode)
	}
	c.cookie = ""
	if resp, _ := c.do(http.MethodGet, "/api/me", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("after logout: want 401, got %d", resp.StatusCode)
	}

	// Wrong password and unknown account must be indistinguishable.
	var wrongBody, unknownBody []byte
	var respWrong, respUnknown *http.Response
	respWrong, wrongBody = c.do(http.MethodPost, "/api/login", LoginRequest{Email: "anna@example.com", Password: "wrong password!"})
	respUnknown, unknownBody = c.do(http.MethodPost, "/api/login", LoginRequest{Email: "nobody@example.com", Password: "wrong password!"})
	if respWrong.StatusCode != http.StatusUnauthorized || respUnknown.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401/401, got %d/%d", respWrong.StatusCode, respUnknown.StatusCode)
	}
	if string(wrongBody) != string(unknownBody) {
		t.Errorf("login responses leak account existence:\n wrong:   %s\n unknown: %s", wrongBody, unknownBody)
	}

	// Correct credentials, case-insensitive email.
	var me MeDTO
	if resp := c.json(http.MethodPost, "/api/login", LoginRequest{Email: "ANNA@Example.com", Password: goodPassword}, &me); resp.StatusCode != 200 {
		t.Fatalf("login: got %d", resp.StatusCode)
	}
	if me.User.Email != "anna@example.com" {
		t.Errorf("email should be normalised, got %q", me.User.Email)
	}
	if resp, _ := c.do(http.MethodGet, "/api/me", nil); resp.StatusCode != 200 {
		t.Errorf("session should work after login, got %d", resp.StatusCode)
	}
}

func TestSettingsRoundTripAndSecretHandling(t *testing.T) {
	_, c := newTestServer(t)
	c.json(http.MethodPost, "/api/setup", SetupRequest{Email: "anna@example.com", Password: goodPassword}, nil)

	var got SettingsDTO
	c.json(http.MethodGet, "/api/settings", nil, &got)
	if got.Drill.GateThreshold == 0 {
		t.Error("drill gate should be exposed to the UI")
	}
	if got.Secrets["llmKey"] {
		t.Error("no secret should be present on a fresh install")
	}

	// Store a secret.
	review := got.Review
	review.NewPerDay = 15
	var updated UpdateSettingsResponse
	resp := c.json(http.MethodPut, "/api/settings", UpdateSettingsRequest{
		Review:  &review,
		Secrets: map[string]string{"llmKey": "sk-secret-value"},
	}, &updated)
	if resp.StatusCode != 200 {
		t.Fatalf("put settings: got %d", resp.StatusCode)
	}
	if updated.Settings.Review.NewPerDay != 15 {
		t.Errorf("newPerDay not saved: %d", updated.Settings.Review.NewPerDay)
	}
	if !updated.Settings.Secrets["llmKey"] {
		t.Error("secret presence should be reported after storing one")
	}

	// The value itself must never appear on the wire.
	_, raw := c.do(http.MethodGet, "/api/settings", nil)
	if bytes.Contains(raw, []byte("sk-secret-value")) {
		t.Fatalf("secret value leaked in settings response: %s", raw)
	}

	// A blank secret preserves the stored one.
	c.json(http.MethodPut, "/api/settings", UpdateSettingsRequest{
		Secrets: map[string]string{"llmKey": ""},
	}, &updated)
	if !updated.Settings.Secrets["llmKey"] {
		t.Error("blank secret should preserve the stored value, not clear it")
	}
}

func TestSettingsClampIsReported(t *testing.T) {
	_, c := newTestServer(t)
	var me MeDTO
	c.json(http.MethodPost, "/api/setup", SetupRequest{Email: "a@example.com", Password: goodPassword}, &me)

	review := me.Settings.Review
	review.DesiredRetention = 0.99 // above the documented maximum

	var updated UpdateSettingsResponse
	if resp := c.json(http.MethodPut, "/api/settings", UpdateSettingsRequest{Review: &review}, &updated); resp.StatusCode != 200 {
		t.Fatalf("put settings: got %d", resp.StatusCode)
	}
	if len(updated.Clamped) != 1 || updated.Clamped[0].Field != "review.desiredRetention" {
		t.Fatalf("a clamp must be reported to the UI, got %+v", updated.Clamped)
	}
	if updated.Settings.Review.DesiredRetention != 0.95 {
		t.Errorf("clamped value: got %v, want 0.95", updated.Settings.Review.DesiredRetention)
	}
}

func TestSettingsRejectsInvalidAndDoesNotPersist(t *testing.T) {
	s, c := newTestServer(t)
	var me MeDTO
	c.json(http.MethodPost, "/api/setup", SetupRequest{Email: "a@example.com", Password: goodPassword}, &me)

	before := s.Config().Review.NewPerDay

	review := me.Settings.Review
	review.NewPerDay = 42
	review.Goal = "fluency-in-3-months" // invalid

	resp, _ := c.do(http.MethodPut, "/api/settings", UpdateSettingsRequest{Review: &review})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 for invalid goal, got %d", resp.StatusCode)
	}
	// A rejected edit must not partially apply.
	if got := s.Config().Review.NewPerDay; got != before {
		t.Errorf("rejected update leaked through: newPerDay %d -> %d", before, got)
	}
}

func TestUnknownServiceRejectedBeforeSaving(t *testing.T) {
	s, c := newTestServer(t)
	var me MeDTO
	c.json(http.MethodPost, "/api/setup", SetupRequest{Email: "a@example.com", Password: goodPassword}, &me)

	before := s.Config().Review.NewPerDay
	review := me.Settings.Review
	review.NewPerDay = 7

	resp, _ := c.do(http.MethodPut, "/api/settings", UpdateSettingsRequest{
		Review:   &review,
		Services: map[string]ServiceDTO{"telepathy": {Provider: "local", Endpoint: "http://x"}},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 for unknown service, got %d", resp.StatusCode)
	}
	if got := s.Config().Review.NewPerDay; got != before {
		t.Errorf("rest of the edit was persisted despite rejection: %d -> %d", before, got)
	}
}

func TestCapabilitiesReportUnconfiguredServices(t *testing.T) {
	_, c := newTestServer(t)
	c.json(http.MethodPost, "/api/setup", SetupRequest{Email: "a@example.com", Password: goodPassword}, nil)

	var caps []CapabilityDTO
	if resp := c.json(http.MethodGet, "/api/capabilities", nil, &caps); resp.StatusCode != 200 {
		t.Fatalf("capabilities: got %d", resp.StatusCode)
	}
	if len(caps) != 4 {
		t.Fatalf("want tts/asr/nlp/llm, got %d: %+v", len(caps), caps)
	}
	for _, cap := range caps {
		if cap.Configured || cap.Reachable {
			t.Errorf("%s should be unconfigured by default, got %+v", cap.Name, cap)
		}
	}
}

func TestSPAFallbackServesIndexForClientRoutes(t *testing.T) {
	_, c := newTestServer(t)
	resp, body := c.do(http.MethodGet, "/progress", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("client route should fall back to index.html, got %d", resp.StatusCode)
	}
	if !bytes.Contains(body, []byte("<!doctype html>")) {
		t.Errorf("want an HTML document, got %.80s", body)
	}
}

func TestUnknownAPIRouteIs404NotSPA(t *testing.T) {
	_, c := newTestServer(t)
	resp, body := c.do(http.MethodGet, "/api/does-not-exist", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown API route: want 404, got %d", resp.StatusCode)
	}
	if bytes.Contains(body, []byte("<!doctype")) {
		t.Error("API 404 must not fall through to the SPA")
	}
}

// TestAuthRoutesAreRateLimited guards a bug that is invisible without a test:
// Fiber v3 runs route handlers in argument order, so registering the limiter
// after the handler makes it run after the request has already been served —
// i.e. no limiting at all, with no error anywhere.
func TestAuthRoutesAreRateLimited(t *testing.T) {
	_, c := newTestServer(t)

	var limited bool
	for i := range 15 {
		resp, _ := c.do(http.MethodPost, "/api/login",
			LoginRequest{Email: "nobody@example.com", Password: "wrong password!"})
		if resp.StatusCode == http.StatusTooManyRequests {
			limited = true
			if i < 5 {
				t.Fatalf("limiter tripped far too early, at attempt %d", i+1)
			}
			break
		}
	}
	if !limited {
		t.Fatal("login accepted 15 attempts without rate limiting")
	}
}

func TestSPACacheHeaders(t *testing.T) {
	// index.html must never be cached: it is the file that names the current
	// asset hashes, so caching it pins a returning visitor to the previous
	// build for as long as the cache lives. Fingerprinted assets, by contrast,
	// can be cached forever because their name changes when they do.
	_, c := newTestServer(t)

	resp, _ := c.do(http.MethodGet, "/", nil)
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Errorf("index.html Cache-Control = %q, want no-cache", cc)
	}

	// A deep client route also serves index.html and must be equally uncached.
	resp, _ = c.do(http.MethodGet, "/review", nil)
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Errorf("client route Cache-Control = %q, want no-cache", cc)
	}

	resp, _ = c.do(http.MethodGet, "/assets/index-abc123.js", nil)
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("fingerprinted asset Cache-Control = %q, want immutable", cc)
	}
}
