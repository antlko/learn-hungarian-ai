package components

import (
	"context"
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

	"tanulom/internal/components/bundlefix"
	"tanulom/internal/components/ttsfetch"
)

// ---- the download-once tts install (ADR-081) ----
//
// A bundlefix.Mirror (repository-shaped tree) is served by an httptest server;
// the manager's own manifests pin exactly those bytes. No real 25-190 MB file is
// ever fetched, and the real python is never unpacked.

const mirrorRef = "v9.9.9"

type mirror struct {
	*bundledHarness
	srv  *httptest.Server
	root string
	hits atomic.Int64
	mu   sync.Mutex
	per  map[string]int // repo path -> requests
	hook func(w http.ResponseWriter, r *http.Request, p string) bool
}

func newMirrorHarness(t *testing.T, bo bundlefix.Options, mods ...func(*Options)) *mirror {
	t.Helper()
	root := t.TempDir()
	mf, err := bundlefix.Mirror(root, fixturePlatform, bo)
	if err != nil {
		t.Fatal(err)
	}
	mr := &mirror{root: root, per: map[string]int{}}
	files := http.FileServer(http.Dir(root))
	mr.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr.hits.Add(1)
		p, ok := strings.CutPrefix(r.URL.Path, "/"+ttsfetch.Repo+"/"+mirrorRef+"/")
		if !ok {
			http.NotFound(w, r)
			return
		}
		mr.mu.Lock()
		mr.per[p]++
		hook := mr.hook
		mr.mu.Unlock()
		if hook != nil && hook(w, r, p) {
			return
		}
		r.URL.Path = "/" + p
		files.ServeHTTP(w, r)
	}))
	t.Cleanup(mr.srv.Close)
	mr.bundledHarness = newBundledHarness(t, bo, append([]func(*Options){func(o *Options) {
		o.TTSRuntimeDir, o.VoicesDir = "", ""
		o.Manifests = &mf
		o.Fetch = ttsfetch.Config{BaseURL: mr.srv.URL, Ref: mirrorRef, Backoff: func(int) time.Duration { return time.Millisecond }}
	}}, mods...)...)
	return mr
}

func (m *mirror) requests(repoPath string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.per[repoPath]
}

func (m *mirror) setHook(h func(w http.ResponseWriter, r *http.Request, p string) bool) {
	m.mu.Lock()
	m.hook = h
	m.mu.Unlock()
}

func (m *mirror) cache() string { return m.m.cacheDir(TTS) }

func (m *mirror) leftovers(t *testing.T) {
	t.Helper()
	for _, p := range []string{".unpack", "python", stampName} {
		if _, err := os.Stat(filepath.Join(m.root_(), p)); err == nil {
			t.Errorf("%s was left behind", p)
		}
	}
	_ = filepath.Walk(m.root_(), func(p string, _ os.FileInfo, _ error) error {
		if strings.HasSuffix(p, ".part") {
			t.Errorf("partial download left behind: %s", p)
		}
		return nil
	})
}

func (m *mirror) root_() string { return m.m.final(TTS) }

func TestInstallDownloadsThenVerifiesUnpacksAndStarts(t *testing.T) {
	mr := newMirrorHarness(t, bundlefix.Options{})
	if st := mr.m.Status(TTS); st.Installed || st.State != StateAbsent || !st.Available {
		t.Fatalf("before: %+v", st)
	}
	st := mr.install(t, TTS)
	if got := strings.Join(mr.stepList(), ","); got != StepFetch+","+StepVerify+","+StepUnpack+","+StepVoices {
		t.Errorf("steps = %q", got)
	}
	if !st.Installed || strings.Join(st.Voices, ",") != "anna,berta,imre" {
		t.Errorf("after: installed=%v voices=%v", st.Installed, st.Voices)
	}
	if got := mr.hits.Load(); got != 3+6 {
		t.Errorf("requests = %d, want 9 (tarball + 2 wheels + 6 voice files)", got)
	}
	if got := envOf(mr.hr.starts[0], "TTS_VOICES_DIR"); got != filepath.Join(mr.root_(), "voices") {
		t.Errorf("TTS_VOICES_DIR = %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(mr.root_(), "voices", VoiceFile("anna"))); string(got) != string(bundlefix.Voices()[VoiceFile("anna")]) {
		t.Error("the installed voice differs from the pinned one")
	}
	if _, err := os.Stat(mr.cache()); err == nil {
		t.Error("the download cache was kept after a successful install")
	}
	if len(mr.installs) != 1 {
		t.Errorf("OnInstalled = %v", mr.installs)
	}
}

func TestStatusShowsMonotonicDownloadProgressDuringTheFetchStep(t *testing.T) {
	mr := newMirrorHarness(t, bundlefix.Options{})
	gate := make(chan struct{})
	var once sync.Once
	mr.setHook(func(w http.ResponseWriter, r *http.Request, p string) bool {
		if !strings.HasSuffix(p, ".tar.gz") {
			return false
		}
		// Announce the full length, send half, wait: the card must be able to see a half-done fetch.
		b, _ := os.ReadFile(filepath.Join(mr.root, filepath.FromSlash(p)))
		w.Header().Set("Content-Length", fmt.Sprint(len(b)))
		_, _ = w.Write(b[:len(b)/2])
		w.(http.Flusher).Flush()
		once.Do(func() {})
		select {
		case <-gate:
		case <-r.Context().Done():
			return true
		}
		_, _ = w.Write(b[len(b)/2:])
		return true
	})
	if err := mr.m.Install(context.Background(), TTS); err != nil {
		t.Fatal(err)
	}
	var last int64
	deadline := time.Now().Add(5 * time.Second)
	for {
		st := mr.m.Status(TTS)
		if st.Step == StepFetch && st.DownloadTotal > 0 && st.DownloadBytes > 0 {
			if st.DownloadBytes < last {
				t.Fatalf("downloadBytes went backwards: %d -> %d", last, st.DownloadBytes)
			}
			last = st.DownloadBytes
			if st.DownloadBytes < st.DownloadTotal && st.Progress > 0 && st.Progress < 0.5 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no fetch progress seen: %+v", st)
		}
		time.Sleep(2 * time.Millisecond)
	}
	close(gate)
	st := mr.waitState(t, TTS, StateRunning, true)
	if st.DownloadBytes != 0 || st.DownloadTotal != 0 || st.Step != "" {
		t.Errorf("download counters outside the fetch step: %+v", st)
	}
}

// After install nothing touches the network: start, stop, restart, enable and
// the boot autostart make zero HTTP requests (a transport that fails the test).
func TestNothingTouchesTheNetworkAfterInstall(t *testing.T) {
	mr := newMirrorHarness(t, bundlefix.Options{})
	mr.install(t, TTS)
	var calls atomic.Int64
	mr.m.opt.Fetch.Client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("network is cut")
	})}
	srvHits := mr.hits.Load()

	if err := mr.m.Stop(TTS); err != nil {
		t.Fatal(err)
	}
	if st := mr.m.Status(TTS); st.State != StateStopped || !st.Installed {
		t.Fatalf("stopped: %+v", st)
	}
	if err := mr.m.Start(TTS); err != nil { // restart path
		t.Fatal(err)
	}
	mr.waitState(t, TTS, StateRunning, false)
	_ = mr.m.Stop(TTS)
	mr.m.AutoStart(func(id string) bool { return id == TTS }) // boot path
	mr.waitState(t, TTS, StateRunning, true)
	_ = mr.m.Stop(TTS)
	mr.install(t, TTS) // «Включить»: Install on an installed, stopped tts is the fast Enable

	// A stale stamp (an upgraded binary) still starts the environment it has.
	_ = mr.m.Stop(TTS)
	_ = os.WriteFile(filepath.Join(mr.root_(), stampName), []byte("manifest=old\n"), 0o644)
	if err := mr.m.Start(TTS); err != nil {
		t.Fatal(err)
	}
	mr.waitState(t, TTS, StateRunning, false)

	if calls.Load() != 0 || mr.hits.Load() != srvHits {
		t.Errorf("network used after install: client calls %d, server hits %d -> %d", calls.Load(), srvHits, mr.hits.Load())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBootDoesNotDownloadAnUninstalledComponent(t *testing.T) {
	mr := newMirrorHarness(t, bundlefix.Options{})
	mr.m.AutoStart(func(id string) bool { return id == TTS })
	time.Sleep(50 * time.Millisecond)
	if st := mr.m.Status(TTS); st.State != StateAbsent || mr.hits.Load() != 0 {
		t.Errorf("boot fetched: state %q, %d requests", st.State, mr.hits.Load())
	}
	if err := mr.m.Start(TTS); errKey(err) != KeyNotInstalled || mr.hits.Load() != 0 {
		t.Errorf("Start on an uninstalled tts = %v (%d requests)", err, mr.hits.Load())
	}
}

func TestCorruptDownloadIsKeyedKeepsGoodFilesAndARetryFetchesOnlyWhatIsMissing(t *testing.T) {
	mr := newMirrorHarness(t, bundlefix.Options{})
	var flip atomic.Bool
	flip.Store(true)
	mr.setHook(func(w http.ResponseWriter, r *http.Request, p string) bool {
		if flip.Load() && strings.Contains(p, "onnxruntime") {
			b, _ := os.ReadFile(filepath.Join(mr.root, filepath.FromSlash(p)))
			b[3] ^= 0xff
			w.Header().Set("Content-Length", fmt.Sprint(len(b)))
			_, _ = w.Write(b)
			return true
		}
		return false
	})
	st := mr.failedWith(t)
	if st.ErrorKey != KeyBundleCorrupt {
		t.Fatalf("key = %q (%s)", st.ErrorKey, st.ErrorDetail)
	}
	if _, err := os.Stat(filepath.Join(mr.cache(), fixturePlatform, "onnxruntime-1.32.0-cp312-cp312-fake.whl")); err == nil {
		t.Error("the corrupt file was kept")
	}
	mr.leftovers(t)
	flip.Store(false)
	before := mr.hits.Load()
	mr.install(t, TTS)
	if got := mr.hits.Load() - before; got > 9 {
		t.Errorf("retry made %d requests", got)
	}
	// Files that were already valid were not fetched again.
	if n := mr.requests("services/tts/vendor/" + fixturePlatform + "/" + bundlefix.Tarball); n != 1 {
		t.Errorf("the tarball was requested %d times", n)
	}
}

func TestDownloadFailureHasItsOwnKeyAndResumesWithoutRefetching(t *testing.T) {
	mr := newMirrorHarness(t, bundlefix.Options{})
	var down atomic.Bool
	down.Store(true)
	mr.setHook(func(w http.ResponseWriter, r *http.Request, p string) bool {
		if down.Load() && strings.HasSuffix(p, "onnxruntime-1.32.0-cp312-cp312-fake.whl") {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return true
		}
		return false
	})
	st := mr.failedWith(t)
	if st.ErrorKey != KeyTTSDownloadFailed {
		t.Fatalf("key = %q (%s)", st.ErrorKey, st.ErrorDetail)
	}
	if strings.Contains(st.ErrorDetail, mr.srv.URL) {
		t.Errorf("the URL leaked into the detail: %s", st.ErrorDetail)
	}
	mr.leftovers(t)
	down.Store(false)
	mr.install(t, TTS)
	tar := "services/tts/vendor/" + fixturePlatform + "/" + bundlefix.Tarball
	if n := mr.requests(tar); n != 1 {
		t.Errorf("an already-valid file was fetched again: %d requests for the tarball", n)
	}
}

func TestStopDuringTheDownloadCancelsWithoutAnErrorOrPartialFiles(t *testing.T) {
	mr := newMirrorHarness(t, bundlefix.Options{})
	started := make(chan struct{})
	var once sync.Once // the hook can run again on a retry; a second close would panic
	mr.setHook(func(w http.ResponseWriter, r *http.Request, p string) bool {
		if !strings.HasSuffix(p, ".tar.gz") {
			return false
		}
		// Announce the file's REAL pinned size, then stall after one byte: a size that
		// differs from the pin would be refused as corrupt at once, racing the Stop below.
		size := int64(100000)
		if fi, err := os.Stat(filepath.Join(mr.root, filepath.FromSlash(p))); err == nil {
			size = fi.Size()
		}
		w.Header().Set("Content-Length", fmt.Sprint(size))
		_, _ = w.Write([]byte("x"))
		w.(http.Flusher).Flush()
		once.Do(func() { close(started) })
		<-r.Context().Done()
		return true
	})
	if err := mr.m.Install(context.Background(), TTS); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(30 * time.Second):
		t.Fatal("the download never started")
	}
	if err := mr.m.Stop(TTS); err != nil {
		t.Fatal(err)
	}
	st := mr.waitState(t, TTS, StateAbsent, true)
	if st.ErrorKey != "" || st.Installed {
		t.Errorf("after cancel: %+v", st)
	}
	mr.leftovers(t)
	if mr.m.Installed(TTS) {
		t.Error("installed after a cancelled download")
	}
}

func TestUninstallRemovesTheCacheAndTheUnpackedEnvironment(t *testing.T) {
	// Leftovers of a failed attempt are removed too.
	mr := newMirrorHarness(t, bundlefix.Options{PipFail: true})
	mr.failedWith(t)
	if _, err := os.Stat(mr.cache()); err != nil {
		t.Fatal("a failed install should keep its cache for the retry")
	}
	if err := mr.m.Uninstall(TTS); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(mr.cache()); err == nil {
		t.Error("Uninstall kept the download cache")
	}

	ok := newMirrorHarness(t, bundlefix.Options{})
	ok.install(t, TTS)
	if err := ok.m.Uninstall(TTS); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"python", "voices", "cache", stampName} {
		if _, err := os.Stat(filepath.Join(ok.root_(), p)); err == nil {
			t.Errorf("%s survived Uninstall", p)
		}
	}
	if st := ok.m.Status(TTS); st.Installed || st.State != StateAbsent {
		t.Errorf("after Uninstall: %+v", st)
	}
	// It comes back, from the mirror.
	ok.install(t, TTS)
}

func TestAnOperatorsRuntimeTreeNeverDownloads(t *testing.T) {
	b := newBundledHarness(t, bundlefix.Options{}, func(o *Options) {
		o.Fetch.Client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Error("a request was made with --tts-runtime-dir set")
			return nil, errors.New("no")
		})}
	})
	b.install(t, TTS)
	if got := strings.Join(b.stepList(), ","); strings.Contains(got, StepFetch) {
		t.Errorf("steps = %q", got)
	}
}

func TestOperatorVoicesAreUsedAndNotDownloaded(t *testing.T) {
	voices := t.TempDir()
	writeVoices(t, voices, "anna", "berta", "imre")
	mr := newMirrorHarness(t, bundlefix.Options{}, func(o *Options) { o.VoicesDir = voices })
	mr.install(t, TTS)
	if n := mr.hits.Load(); n != 3 {
		t.Errorf("requests = %d, want 3 (runtime only)", n)
	}
	if got := envOf(mr.hr.starts[0], "TTS_VOICES_DIR"); got != voices {
		t.Errorf("TTS_VOICES_DIR = %q", got)
	}
}

func TestAnInvalidMirrorURLFailsTheInstallCleanly(t *testing.T) {
	mr := newMirrorHarness(t, bundlefix.Options{}, func(o *Options) { o.Fetch.BaseURL = "http://example.com" })
	st := mr.failedWith(t)
	if st.ErrorKey != KeyInstallFailed || mr.hits.Load() != 0 {
		t.Errorf("%q, %d hits", st.ErrorKey, mr.hits.Load())
	}
}

func TestDownloadedFileTamperedAfterArrivalIsCaughtAtVerify(t *testing.T) {
	mr := newMirrorHarness(t, bundlefix.Options{PipFail: true})
	mr.failedWith(t) // leaves a complete cache (pip failed after it)
	// Tamper with a cached wheel, then retry: Fetch re-hashes the cache, finds the bad
	// file and fetches it again, so the install does not trust what is on disk.
	wheel := filepath.Join(mr.cache(), fixturePlatform, "piper_tts-1.8.0-cp39-abi3-fake.whl")
	if err := os.WriteFile(wheel, []byte("evil wheel bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := mr.requests("services/tts/vendor/" + fixturePlatform + "/piper_tts-1.8.0-cp39-abi3-fake.whl")
	mr.failedWith(t)
	if got := mr.requests("services/tts/vendor/" + fixturePlatform + "/piper_tts-1.8.0-cp39-abi3-fake.whl"); got != before+1 {
		t.Errorf("the tampered cache file was not refetched (%d -> %d)", before, got)
	}
}
