package ttsfetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Everything here talks to httptest servers with tiny files and their own
// manifests: no test downloads a real 25 MB archive.

const testRef = "v9.9.9"

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

type fixture struct {
	files []File
	body  map[string][]byte // repo path -> bytes
}

func newFixture() *fixture {
	f := &fixture{body: map[string][]byte{}}
	f.add("a/one.bin", "services/tts/vendor/linux-amd64/one.bin", strings.Repeat("1", 200_000))
	f.add("a/two+x.bin", "services/tts/vendor/linux-amd64/two+x.bin", strings.Repeat("2", 70_000))
	f.add("voices/v.onnx", "langpack/voices/v.onnx", strings.Repeat("3", 5_000))
	return f
}

func (f *fixture) add(rel, repo, body string) {
	f.body[repo] = []byte(body)
	f.files = append(f.files, File{Rel: rel, Repo: repo, Sum: sum([]byte(body)), Size: int64(len(body))})
}

func (f *fixture) total() (n int64) {
	for _, fl := range f.files {
		n += fl.Size
	}
	return n
}

// server serves f at /<Repo>/<ref>/<path>; h may wrap or replace the answer.
func (f *fixture) server(t *testing.T, h func(w http.ResponseWriter, r *http.Request, path string, body []byte) bool) (*httptest.Server, *int64) {
	t.Helper()
	var n int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&n, 1)
		p, ok := strings.CutPrefix(r.URL.Path, "/"+Repo+"/"+testRef+"/")
		body, has := f.body[p]
		if !ok || !has {
			http.NotFound(w, r)
			return
		}
		if h != nil && h(w, r, p, body) {
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

func fetcher(t *testing.T, base string, mod ...func(*Config)) *Fetcher {
	t.Helper()
	cfg := Config{BaseURL: base, Ref: testRef, Backoff: func(int) time.Duration { return time.Millisecond }, IdleTimeout: 5 * time.Second}
	for _, m := range mod {
		m(&cfg)
	}
	f, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func noPart(t *testing.T, dir string) {
	t.Helper()
	_ = filepath.Walk(dir, func(p string, _ os.FileInfo, _ error) error {
		if strings.HasSuffix(p, ".part") {
			t.Errorf("partial file left behind: %s", p)
		}
		return nil
	})
}

func TestFetchDownloadsVerifiesAndReportsMonotonicProgress(t *testing.T) {
	fx := newFixture()
	srv, n := fx.server(t, nil)
	dir := t.TempDir()
	var mu sync.Mutex
	var seen []int64
	var gotTotal int64
	err := fetcher(t, srv.URL).Fetch(context.Background(), dir, fx.files, func(d, tot int64) {
		mu.Lock()
		seen, gotTotal = append(seen, d), tot
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	if *n != 3 {
		t.Errorf("requests = %d, want 3", *n)
	}
	for _, fl := range fx.files {
		if got, _ := os.ReadFile(filepath.Join(dir, fl.Rel)); sum(got) != fl.Sum {
			t.Errorf("%s was not written intact", fl.Rel)
		}
	}
	if gotTotal != fx.total() || seen[len(seen)-1] != fx.total() {
		t.Errorf("progress ended at %d of %d (total %d)", seen[len(seen)-1], fx.total(), gotTotal)
	}
	for i := 1; i < len(seen); i++ {
		if seen[i] < seen[i-1] {
			t.Fatalf("progress went backwards: %v", seen)
		}
	}
	if len(seen) < 5 {
		t.Errorf("only %d progress reports for 275 kB in 64 kB chunks", len(seen))
	}
	noPart(t, dir)
}

func TestHashMismatchIsCorruptDeletesTheFileAndARetrySucceeds(t *testing.T) {
	fx := newFixture()
	flip := atomic.Bool{}
	flip.Store(true)
	srv, _ := fx.server(t, func(w http.ResponseWriter, r *http.Request, p string, body []byte) bool {
		if flip.Load() && strings.HasSuffix(p, "two+x.bin") {
			bad := append([]byte(nil), body...)
			bad[100] ^= 0xff
			w.Header().Set("Content-Length", fmt.Sprint(len(bad)))
			_, _ = w.Write(bad)
			return true
		}
		return false
	})
	dir := t.TempDir()
	f := fetcher(t, srv.URL)
	err := f.Fetch(context.Background(), dir, fx.files, nil)
	var fe *Error
	if !errors.As(err, &fe) || fe.Kind != KindCorrupt || !errors.Is(err, ErrCorrupt) || errors.Is(err, ErrDownload) {
		t.Fatalf("err = %v, want KindCorrupt", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a", "two+x.bin")); err == nil {
		t.Error("the corrupt file was kept")
	}
	noPart(t, dir)
	flip.Store(false)
	if err := f.Fetch(context.Background(), dir, fx.files, nil); err != nil {
		t.Fatalf("retry: %v", err)
	}
}

func TestTruncatedAndOversizedBodiesAreCorrupt(t *testing.T) {
	for name, mutate := range map[string]func([]byte) []byte{
		"truncated": func(b []byte) []byte { return b[:len(b)-1] },
		"oversized": func(b []byte) []byte { return append(append([]byte(nil), b...), 'x') },
	} {
		t.Run(name, func(t *testing.T) {
			fx := newFixture()
			srv, n := fx.server(t, func(w http.ResponseWriter, r *http.Request, p string, body []byte) bool {
				// chunked: no Content-Length, so only the byte count can tell.
				_, _ = w.Write(mutate(body))
				return true
			})
			dir := t.TempDir()
			err := fetcher(t, srv.URL).Fetch(context.Background(), dir, fx.files, nil)
			if !errors.Is(err, ErrCorrupt) {
				t.Fatalf("err = %v", err)
			}
			if *n != 1 {
				t.Errorf("a corrupt file was retried: %d requests", *n)
			}
			noPart(t, dir)
			if _, err := os.Stat(filepath.Join(dir, fx.files[0].Rel)); err == nil {
				t.Error("the bad file was kept")
			}
		})
	}
	t.Run("announced length differs", func(t *testing.T) {
		fx := newFixture()
		srv, _ := fx.server(t, func(w http.ResponseWriter, r *http.Request, p string, body []byte) bool {
			w.Header().Set("Content-Length", fmt.Sprint(len(body)+5))
			_, _ = w.Write(body)
			return true
		})
		if err := fetcher(t, srv.URL).Fetch(context.Background(), t.TempDir(), fx.files, nil); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestRedirectOffHostIsRefusedAndSameHostIsFollowed(t *testing.T) {
	fx := newFixture()
	other, hits := fx.server(t, nil)
	srv, _ := fx.server(t, func(w http.ResponseWriter, r *http.Request, p string, body []byte) bool {
		// localhost vs 127.0.0.1: another host name for the same machine is still another host.
		http.Redirect(w, r, strings.Replace(other.URL, "127.0.0.1", "localhost", 1)+r.URL.Path, http.StatusFound)
		return true
	})
	err := fetcher(t, srv.URL, func(c *Config) { c.Attempts = 1 }).Fetch(context.Background(), t.TempDir(), fx.files, nil)
	if !errors.Is(err, ErrDownload) || !strings.Contains(err.Error(), "another host") {
		t.Fatalf("err = %v", err)
	}
	if *hits != 0 {
		t.Errorf("the off-host server was contacted %d times", *hits)
	}

	// Same host: followed.
	var first atomic.Bool
	srv2, _ := fx.server(t, func(w http.ResponseWriter, r *http.Request, p string, body []byte) bool {
		if first.CompareAndSwap(false, true) {
			http.Redirect(w, r, r.URL.Path, http.StatusFound)
			return true
		}
		return false
	})
	if err := fetcher(t, srv2.URL).Fetch(context.Background(), t.TempDir(), fx.files, nil); err != nil {
		t.Fatalf("same-host redirect: %v", err)
	}
}

func TestBaseURLRules(t *testing.T) {
	for _, ok := range []string{"", "https://mirror.example.org", "https://mirror.example.org/some/prefix/", "http://127.0.0.1:8000", "http://localhost:8000", "http://[::1]:8000"} {
		if err := ValidateBaseURL(ok); err != nil {
			t.Errorf("%q refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"http://example.com", "http://192.168.1.5:8000", "ftp://x.example", "file:///tmp/x", "https://u:p@example.com", "https://example.com/?a=b", "https://example.com/#x", "example.com", "http://127.0.0.1.evil.example", "https://"} {
		if err := ValidateBaseURL(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestDefaultURLIsFixedFromRepoRefAndManifestOnly(t *testing.T) {
	f, err := New(Config{Ref: "v1.5.0"})
	if err != nil {
		t.Fatal(err)
	}
	u, err := f.url(File{Rel: "linux-amd64/cpython-3.12.16+20261110-x.tar.gz", Repo: "services/tts/vendor/linux-amd64/cpython-3.12.16+20261110-x.tar.gz"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://raw.githubusercontent.com/antlko/learn-hungarian-ai/v1.5.0/services/tts/vendor/linux-amd64/cpython-3.12.16+20261110-x.tar.gz"; u != want {
		t.Errorf("url = %s", u)
	}
	for _, bad := range []string{"services/../../etc/passwd", "a//b", "a/./b", "/etc/passwd"} {
		if _, err := f.url(File{Repo: bad}); err == nil {
			t.Errorf("path %q accepted", bad)
		}
	}
	for _, ref := range []string{"main/../x", "a b", "", ".."} {
		if _, err := New(Config{Ref: ref, Repo: Repo}); err == nil && ref != "" {
			t.Errorf("ref %q accepted", ref)
		}
	}
}

func TestPlanRefusesUnsafeNamesAndKeepsSourceOut(t *testing.T) {
	h := strings.Repeat("a", 64)
	ok := Manifests{
		Vendor: []byte(h + "  10  linux-amd64/good-1.0.whl\n" + h + "  10  source/piper.tar.gz\n" + h + "  10  darwin-arm64/other.whl\n"),
		Voices: []byte(h + "  5  hu_HU-anna-medium.onnx\n"),
	}
	files, err := Plan(ok, "linux-amd64", true, true)
	if err != nil || len(files) != 2 || files[0].Repo != "services/tts/vendor/linux-amd64/good-1.0.whl" || files[1].Repo != "langpack/voices/hu_HU-anna-medium.onnx" {
		t.Fatalf("%v %+v", err, files)
	}
	if files, _ := Plan(ok, "linux-amd64", false, true); len(files) != 1 {
		t.Errorf("voices only = %+v", files)
	}
	for _, bad := range []string{
		"linux-amd64/../../x", "linux-amd64/a/b", "linux-amd64/a?b", "linux-amd64/a b", "linux-amd64/.hidden", "linux-amd64/..%2fx", "linux-amd64/x#y",
	} {
		m := Manifests{Vendor: []byte(h + "  10  " + bad + "\n")}
		if _, err := Plan(m, "linux-amd64", true, false); err == nil {
			t.Errorf("name %q accepted", bad)
		}
	}
	if _, err := Plan(ok, "windows-amd64", true, false); err == nil {
		t.Error("unsupported platform accepted")
	}
	if _, err := Plan(Manifests{Vendor: []byte(h + "  10  linux-arm64/x.whl\n")}, "linux-amd64", true, false); err == nil {
		t.Error("a platform without files accepted")
	}
	if _, err := Plan(Manifests{Voices: []byte(h + "  5  ../x.onnx\n")}, "linux-amd64", false, true); err == nil {
		t.Error("a voice path with .. accepted")
	}
}

func TestValidFilesAreKeptAndCostNoRequest(t *testing.T) {
	fx := newFixture()
	srv, n := fx.server(t, nil)
	dir := t.TempDir()
	f := fetcher(t, srv.URL)
	if err := f.Fetch(context.Background(), dir, fx.files, nil); err != nil {
		t.Fatal(err)
	}
	if *n != 3 {
		t.Fatalf("first fetch: %d requests", *n)
	}
	var last int64
	if err := f.Fetch(context.Background(), dir, fx.files, func(d, _ int64) { last = d }); err != nil {
		t.Fatal(err)
	}
	if *n != 3 || last != fx.total() {
		t.Errorf("resume made requests: %d, progress %d", *n, last)
	}
	// Damage one file and drop another: only those two are fetched again.
	_ = os.WriteFile(filepath.Join(dir, fx.files[0].Rel), []byte("damaged"), 0o644)
	_ = os.Remove(filepath.Join(dir, fx.files[2].Rel))
	if err := f.Fetch(context.Background(), dir, fx.files, nil); err != nil {
		t.Fatal(err)
	}
	if *n != 5 {
		t.Errorf("requests = %d, want 5", *n)
	}
	// A stale .part from a killed run is never trusted.
	_ = os.WriteFile(filepath.Join(dir, fx.files[1].Rel+".part"), []byte("half"), 0o644)
	if err := f.Fetch(context.Background(), dir, fx.files, nil); err != nil {
		t.Fatal(err)
	}
	noPart(t, dir)
}

func TestContextCancelAbortsAndLeavesNoPartial(t *testing.T) {
	fx := newFixture()
	// Buffered: the handler may run BEFORE the test reaches its receive (slow CI under
	// -race). With an unbuffered channel the non-blocking send below would be dropped
	// for good and the test would wait for a signal that never comes.
	started := make(chan struct{}, 1)
	srv, _ := fx.server(t, func(w http.ResponseWriter, r *http.Request, p string, body []byte) bool {
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		_, _ = w.Write(body[:1000])
		w.(http.Flusher).Flush()
		select {
		case started <- struct{}{}:
		default:
		}
		<-r.Context().Done()
		return true
	})
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- fetcher(t, srv.URL).Fetch(ctx, dir, fx.files, nil) }()
	select {
	case <-started:
	case err := <-done:
		// Fetch ended before the first byte was served: say WHY instead of timing out.
		t.Fatalf("Fetch returned before the download started: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("the download never started")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Fetch ignored the cancellation")
	}
	noPart(t, dir)
	if _, err := os.Stat(filepath.Join(dir, fx.files[0].Rel)); err == nil {
		t.Error("a half-written file was kept")
	}
}

func TestServerErrorsRetryThenFailAsDownloadErrors(t *testing.T) {
	fx := newFixture()
	srv, n := fx.server(t, func(w http.ResponseWriter, r *http.Request, p string, body []byte) bool {
		http.Error(w, "boom", http.StatusServiceUnavailable)
		return true
	})
	err := fetcher(t, srv.URL).Fetch(context.Background(), t.TempDir(), fx.files, nil)
	var fe *Error
	if !errors.As(err, &fe) || fe.Kind != KindDownload || !errors.Is(err, ErrDownload) {
		t.Fatalf("err = %v", err)
	}
	if *n != 3 {
		t.Errorf("attempts = %d, want 3", *n)
	}
	if strings.Contains(err.Error(), srv.URL) {
		t.Errorf("the URL leaked into the error: %v", err)
	}

	// A transient 503 then success: the file arrives.
	var calls atomic.Int32
	srv2, _ := fx.server(t, func(w http.ResponseWriter, r *http.Request, p string, body []byte) bool {
		if calls.Add(1) <= 2 {
			http.Error(w, "later", http.StatusBadGateway)
			return true
		}
		return false
	})
	if err := fetcher(t, srv2.URL).Fetch(context.Background(), t.TempDir(), fx.files, nil); err != nil {
		t.Fatalf("transient errors were not retried: %v", err)
	}

	// 404 is a definite answer: no retry.
	srv3, n3 := fx.server(t, func(w http.ResponseWriter, r *http.Request, p string, body []byte) bool {
		http.NotFound(w, r)
		return true
	})
	if err := fetcher(t, srv3.URL).Fetch(context.Background(), t.TempDir(), fx.files, nil); !errors.Is(err, ErrDownload) || *n3 != 1 {
		t.Fatalf("404: err=%v requests=%d", err, *n3)
	}
}

func TestConnectionDroppedMidBodyIsRetried(t *testing.T) {
	fx := newFixture()
	var calls atomic.Int32
	srv, _ := fx.server(t, func(w http.ResponseWriter, r *http.Request, p string, body []byte) bool {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Length", fmt.Sprint(len(body)))
			_, _ = w.Write(body[:100])
			w.(http.Flusher).Flush()
			panic(http.ErrAbortHandler) // closes the connection under the client
		}
		return false
	})
	dir := t.TempDir()
	if err := fetcher(t, srv.URL).Fetch(context.Background(), dir, fx.files, nil); err != nil {
		t.Fatalf("%v", err)
	}
	noPart(t, dir)
}

func TestStalledServerHitsTheIdleTimeout(t *testing.T) {
	fx := newFixture()
	srv, _ := fx.server(t, func(w http.ResponseWriter, r *http.Request, p string, body []byte) bool {
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		_, _ = w.Write(body[:10])
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		return true
	})
	start := time.Now()
	err := fetcher(t, srv.URL, func(c *Config) { c.Attempts = 1; c.IdleTimeout = 150 * time.Millisecond }).Fetch(context.Background(), t.TempDir(), fx.files, nil)
	if !errors.Is(err, ErrDownload) || time.Since(start) > 3*time.Second {
		t.Fatalf("err = %v after %s", err, time.Since(start))
	}
}

func TestRequestHeadersAndNoCredentials(t *testing.T) {
	fx := newFixture()
	var hdr http.Header
	srv, _ := fx.server(t, func(w http.ResponseWriter, r *http.Request, p string, body []byte) bool {
		hdr = r.Header.Clone()
		return false
	})
	if err := fetcher(t, srv.URL).Fetch(context.Background(), t.TempDir(), fx.files[:1], nil); err != nil {
		t.Fatal(err)
	}
	if hdr.Get("User-Agent") != "tanulom-component-fetch" || hdr.Get("Accept-Encoding") != "identity" || hdr.Get("Authorization") != "" || hdr.Get("Cookie") != "" {
		t.Errorf("headers = %v", hdr)
	}
}

func TestSizeCaps(t *testing.T) {
	fx := newFixture()
	old, oldT := maxFileBytes, maxTotalBytes
	defer func() { maxFileBytes, maxTotalBytes = old, oldT }()
	maxFileBytes = 100_000
	srv, n := fx.server(t, nil)
	if err := fetcher(t, srv.URL).Fetch(context.Background(), t.TempDir(), fx.files, nil); !errors.Is(err, ErrCorrupt) || *n != 0 {
		t.Errorf("a file over the cap: err=%v requests=%d", err, *n)
	}
	maxFileBytes, maxTotalBytes = 1<<20, 100_000
	if err := fetcher(t, srv.URL).Fetch(context.Background(), t.TempDir(), fx.files, nil); !errors.Is(err, ErrCorrupt) || *n != 0 {
		t.Errorf("a plan over the total cap: err=%v requests=%d", err, *n)
	}
}

func TestRefFor(t *testing.T) {
	for in, want := range map[string]string{"v1.5.0": "v1.5.0", "v10.20.30": "v10.20.30", "dev": "main", "": "main", "v1.5.0-3-gabc": "main", "v1.5.0-dirty": "main", "1.5.0": "main", "v1.5": "main", "../x": "main"} {
		if got := RefFor(in); got != want {
			t.Errorf("RefFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSlowDripIsCutOffByTheOverallDeadline(t *testing.T) {
	fx := newFixture()
	old := deadlineSlack
	deadlineSlack = 300 * time.Millisecond
	defer func() { deadlineSlack = old }()
	srv, _ := fx.server(t, func(w http.ResponseWriter, r *http.Request, p string, body []byte) bool {
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		for i := 0; i < 100 && r.Context().Err() == nil; i++ { // never idle long enough for the watchdog
			_, _ = w.Write(body[i : i+1])
			w.(http.Flusher).Flush()
			time.Sleep(50 * time.Millisecond)
		}
		return true
	})
	start := time.Now()
	dir := t.TempDir()
	err := fetcher(t, srv.URL, func(c *Config) { c.Attempts = 1; c.MinBytesPerSecond = 1 << 40; c.IdleTimeout = time.Hour }).Fetch(context.Background(), dir, []File{fx.files[2]}, nil)
	if !errors.Is(err, ErrDownload) || time.Since(start) > 3*time.Second {
		t.Fatalf("err = %v after %s", err, time.Since(start))
	}
	noPart(t, dir)
}
