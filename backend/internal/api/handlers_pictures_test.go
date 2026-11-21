package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestPictureEndpoint(t *testing.T) {
	t.Parallel()
	_, c := newTestServer(t)
	if resp, _ := c.do(http.MethodGet, "/api/pictures/cat.svg", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", resp.StatusCode)
	}
	c.json(http.MethodPost, "/api/setup", SetupRequest{Email: "a@example.com", Password: goodPassword}, nil)

	resp, body := c.do(http.MethodGet, "/api/pictures/cat.svg", nil)
	if resp.StatusCode != 200 || !strings.Contains(string(body), "<svg") {
		t.Fatalf("cat: %d", resp.StatusCode)
	}
	for k, want := range map[string]string{
		"Content-Type":            "image/svg+xml",
		"Cache-Control":           "public, max-age=31536000, immutable",
		"Content-Security-Policy": "default-src 'none'; style-src 'unsafe-inline'",
		"X-Content-Type-Options":  "nosniff",
	} {
		if got := resp.Header.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if resp.Header.Get("ETag") == "" {
		t.Error("no ETag")
	}
	for _, p := range []string{"/api/pictures/nope.svg", "/api/pictures/cat", "/api/pictures/cat.png"} {
		if r, _ := c.do(http.MethodGet, p, nil); r.StatusCode != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", p, r.StatusCode)
		}
	}
}
