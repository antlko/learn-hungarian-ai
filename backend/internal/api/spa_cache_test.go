package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// newRequest and doRaw exist because the shared `client` helper does not carry
// request headers, and these tests are entirely about conditional headers.
func newRequest(t *testing.T, method, path string) *http.Request {
	t.Helper()
	return httptest.NewRequest(method, path, nil)
}

func doRaw(t *testing.T, c *client, req *http.Request) *http.Response {
	t.Helper()
	resp, err := c.app.App().Test(req, fiber.TestConfig{
		Timeout: 90 * time.Second, FailOnTimeout: true,
	})
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	return resp
}

// The black-screen-after-update bug, pinned.
//
// The embedded filesystem reports the zero instant as every file's modification
// time, so fiber's modtime-based 304 answered "not modified" to EVERY
// conditional request for index.html — whatever date the browser sent. Since
// index.html is the one unfingerprinted file, and it is the file naming the
// current asset hashes, a browser that had opened the app once kept its first
// copy forever and booted into a blank page after any rebuild.
func TestIndexRevalidatesOnContentNotModTime(t *testing.T) {
	_, c := newTestServer(t)

	resp, body := c.do(http.MethodGet, "/", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /: %d", resp.StatusCode)
	}
	if len(body) == 0 {
		t.Fatal("index.html came back empty")
	}
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("index.html must carry an ETag — modification time cannot revalidate it")
	}
	if cc := resp.Header.Get("Cache-Control"); cc == "" || cc == "public" {
		t.Errorf("index.html must not be cached without revalidation, got %q", cc)
	}

	// A stale validator must NOT produce a 304. This is the exact request the
	// old server answered "not modified" to, forever.
	req := newRequest(t, http.MethodGet, "/")
	req.Header.Set("If-Modified-Since", "Mon, 25 Aug 2036 00:00:00 GMT")
	if got := doRaw(t, c, req).StatusCode; got == http.StatusNotModified {
		t.Error("a date-based validator must not satisfy index.html: that is the bug")
	}

	// The matching content validator SHOULD, so revalidation stays cheap.
	req = newRequest(t, http.MethodGet, "/")
	req.Header.Set("If-None-Match", etag)
	if got := doRaw(t, c, req).StatusCode; got != http.StatusNotModified {
		t.Errorf("a matching ETag should return 304, got %d", got)
	}
}

// A missing fingerprinted asset must 404 rather than fall through to the shell:
// HTML served for a `.js` request dies in the parser on `<`, and the symptom is
// a blank page instead of the missing file it actually is.
func TestMissingAssetIs404NotTheAppShell(t *testing.T) {
	_, c := newTestServer(t)
	resp, body := c.do(http.MethodGet, "/assets/index-DELETED0.js", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("missing asset: want 404, got %d", resp.StatusCode)
	}
	if len(body) > 0 && body[0] == '<' {
		t.Error("a missing asset returned HTML; the browser would parse it as JavaScript")
	}
}

// A client-side route still deep-links to the shell — that is what the SPA
// fallback is for, and it must survive the asset guard above.
func TestClientRouteStillFallsBackToIndex(t *testing.T) {
	_, c := newTestServer(t)
	resp, _ := c.do(http.MethodGet, "/grammar/placement", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("client route: want 200, got %d", resp.StatusCode)
	}
}
