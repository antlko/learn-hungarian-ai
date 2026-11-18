package speech

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
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

// makeWAV builds a 16-bit mono 1000 Hz-rate PCM WAV of the given seconds.
func makeWAV(seconds int) []byte {
	const rate = 1000
	data := make([]byte, seconds*rate*2)
	h := make([]byte, 44)
	copy(h[0:], "RIFF")
	binary.LittleEndian.PutUint32(h[4:], uint32(36+len(data)))
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1)
	binary.LittleEndian.PutUint16(h[22:], 1)
	binary.LittleEndian.PutUint32(h[24:], rate)
	binary.LittleEndian.PutUint32(h[28:], rate*2)
	binary.LittleEndian.PutUint16(h[32:], 2)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], uint32(len(data)))
	return append(h, data...)
}

// fakeChild stands in for services/tts.
type fakeChild struct {
	*httptest.Server
	synth   atomic.Int32
	hash    atomic.Value // string
	delay   atomic.Int64 // time.Duration
	status  atomic.Int32
	lastReq atomic.Value // map[string]any
}

func newChild(t *testing.T) *fakeChild {
	t.Helper()
	c := &fakeChild{}
	c.status.Store(http.StatusOK)
	c.hash.Store("h1")
	mux := http.NewServeMux()
	mux.HandleFunc("/voices", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]Voice{
			{ID: "anna", SampleRate: 22050, ModelHash: c.hash.Load().(string)},
			{ID: "imre", SampleRate: 22050, ModelHash: "imrehash"},
		})
	})
	mux.HandleFunc("/synthesize", func(w http.ResponseWriter, r *http.Request) {
		c.synth.Add(1)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		c.lastReq.Store(body)
		select {
		case <-time.After(time.Duration(c.delay.Load())):
		case <-r.Context().Done():
			return
		}
		if st := int(c.status.Load()); st != http.StatusOK {
			http.Error(w, "boom", st)
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write(makeWAV(2))
	})
	c.Server = httptest.NewServer(mux)
	t.Cleanup(c.Close)
	return c
}

func TestNormalize(t *testing.T) {
	got, err := Normalize("  Szia \n\t világ  ")
	if err != nil || got != "Szia világ" {
		t.Fatalf("%q %v", got, err)
	}
	// NFC: "e" + combining acute becomes the precomposed é.
	a, _ := Normalize("é")
	b, _ := Normalize("é")
	if a != b {
		t.Errorf("NFC not applied: %q vs %q", a, b)
	}
	for _, bad := range []string{"", "   \n", strings.Repeat("a", MaxTextBytes+1), "\xff\xfe"} {
		if _, err := Normalize(bad); !errors.Is(err, ErrBadText) {
			t.Errorf("Normalize(%q) = %v, want ErrBadText", bad, err)
		}
	}
	if _, err := Normalize(strings.Repeat("a", MaxTextBytes)); err != nil {
		t.Errorf("exactly the limit refused: %v", err)
	}
}

func TestKeyChangesWithEachInputAndNothingBleeds(t *testing.T) {
	base := Key("anna", "h", "sentence", "kór")
	for name, k := range map[string]string{
		"voice": Key("imre", "h", "sentence", "kór"),
		"model": Key("anna", "h2", "sentence", "kór"),
		"style": Key("anna", "h", "word", "kór"),
		"text":  Key("anna", "h", "sentence", "kór."),
		// "ab"+"c" must not collide with "a"+"bc".
		"bleed": Key("anna", "hs", "entence", "kór"),
	} {
		if k == base {
			t.Errorf("key unchanged when %s changes", name)
		}
	}
	if Key("anna", "h", "sentence", "kór") != base || len(base) != 64 {
		t.Error("key not stable / not 64 hex")
	}
}

func TestSynthesizeCachesAndReplayMakesNoChildCall(t *testing.T) {
	child := newChild(t)
	s := New(t.TempDir())
	r := Request{Text: "Szia! Ma együtt olvasunk.", Voice: "anna", Style: StyleSentence}
	res, err := s.Synthesize(context.Background(), child.URL, r)
	if err != nil {
		t.Fatal(err)
	}
	if res.DurationS < 1.99 || res.DurationS > 2.01 {
		t.Errorf("duration %v, want 2", res.DurationS)
	}
	if !ValidFile(res.File) || s.Path(res.File) == "" {
		t.Fatalf("bad file name %q", res.File)
	}
	if _, err := os.Stat(s.Path(res.File)); err != nil {
		t.Fatal(err)
	}
	again, err := s.Synthesize(context.Background(), child.URL, r)
	if err != nil || again != res {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if n := child.synth.Load(); n != 1 {
		t.Errorf("child synthesized %d times, want 1", n)
	}
	// Whitespace-different text is the same key.
	if _, err := s.Synthesize(context.Background(), child.URL,
		Request{Text: "Szia!  Ma együtt\nolvasunk.", Voice: "anna", Style: StyleSentence}); err != nil || child.synth.Load() != 1 {
		t.Errorf("normalised duplicate re-rendered (%d)", child.synth.Load())
	}
	// A replaced model file (new hash) never serves the old audio.
	child.hash.Store("h2")
	s.mu.Lock()
	s.hashes = map[string]hashEntry{}
	s.mu.Unlock()
	if _, err := s.Synthesize(context.Background(), child.URL, r); err != nil || child.synth.Load() != 2 {
		t.Errorf("model change did not re-render (%d, %v)", child.synth.Load(), err)
	}
}

func TestWordStyleRendersASlowOneWordSentence(t *testing.T) {
	child := newChild(t)
	s := New(t.TempDir())
	if _, err := s.Synthesize(context.Background(), child.URL, Request{Text: "kór", Voice: "imre", Style: StyleWord}); err != nil {
		t.Fatal(err)
	}
	b := child.lastReq.Load().(map[string]any)
	if b["text"] != "kór." || b["lengthScale"] != 1.4 || b["voice"] != "imre" {
		t.Errorf("word request = %v", b)
	}
	if _, err := s.Synthesize(context.Background(), child.URL, Request{Text: "Jó napot!", Voice: "imre", Style: StyleSentence}); err != nil {
		t.Fatal(err)
	}
	b = child.lastReq.Load().(map[string]any)
	if b["text"] != "Jó napot!" || b["lengthScale"] != 1.0 {
		t.Errorf("sentence request = %v", b)
	}
}

func TestSingleFlightRendersOncePerKey(t *testing.T) {
	child := newChild(t)
	child.delay.Store(int64(150 * time.Millisecond))
	s := New(t.TempDir())
	r := Request{Text: "Ugyanaz a mondat.", Voice: "anna", Style: StyleSentence}
	var wg sync.WaitGroup
	files := make([]string, 8)
	for i := range files {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := s.Synthesize(context.Background(), child.URL, r)
			if err != nil {
				t.Error(err)
			}
			files[i] = res.File
		}()
	}
	wg.Wait()
	if n := child.synth.Load(); n != 1 {
		t.Errorf("%d concurrent callers caused %d syntheses, want 1", len(files), n)
	}
	for _, f := range files {
		if f != files[0] || f == "" {
			t.Errorf("callers got different files: %v", files)
			break
		}
	}
}

func TestChildIsNeverGivenTwoSyntheses(t *testing.T) {
	var inflight, peak atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/voices", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]Voice{{ID: "anna", SampleRate: 22050, ModelHash: "h"}})
	})
	mux.HandleFunc("/synthesize", func(w http.ResponseWriter, r *http.Request) {
		n := inflight.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
		inflight.Add(-1)
		_, _ = w.Write(makeWAV(1))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	s := New(t.TempDir())
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Synthesize(context.Background(), srv.URL,
				Request{Text: "mondat " + string(rune('a'+i)), Voice: "anna", Style: StyleSentence})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if peak.Load() != 1 {
		t.Errorf("peak concurrent syntheses = %d, want 1", peak.Load())
	}
}

func TestTimeoutAndUnavailable(t *testing.T) {
	child := newChild(t)
	child.delay.Store(int64(2 * time.Second))
	s := New(t.TempDir())
	s.Timeout = 100 * time.Millisecond
	_, err := s.Synthesize(context.Background(), child.URL, Request{Text: "lassú", Voice: "anna", Style: StyleSentence})
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("slow child: %v, want ErrTimeout", err)
	}
	// The failed attempt leaves no file and no stuck flight.
	if len(s.flights) != 0 {
		t.Error("a flight outlived its call")
	}

	child.delay.Store(0)
	child.status.Store(http.StatusInternalServerError)
	s.Timeout = 5 * time.Second
	if _, err := s.Synthesize(context.Background(), child.URL, Request{Text: "hiba", Voice: "anna", Style: StyleSentence}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("500 from child: %v, want ErrUnavailable", err)
	}
	dead := httptest.NewServer(http.NotFoundHandler())
	url := dead.URL
	dead.Close()
	if _, err := s.Synthesize(context.Background(), url, Request{Text: "halott", Voice: "anna", Style: StyleSentence}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("closed port: %v, want ErrUnavailable", err)
	}
	if _, err := s.Synthesize(context.Background(), "", Request{Text: "x", Voice: "anna", Style: StyleSentence}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("no endpoint: %v, want ErrUnavailable", err)
	}
	// A persona the child has no model for is unavailable, not an error page.
	if _, err := s.Synthesize(context.Background(), child.URL, Request{Text: "x", Voice: "berta", Style: StyleSentence}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("voice without model: %v", err)
	}
}

func TestValidationPrecedesTheChild(t *testing.T) {
	s := New(t.TempDir())
	for name, tc := range map[string]struct {
		r    Request
		want error
	}{
		"text":  {Request{Text: " ", Voice: "anna", Style: StyleSentence}, ErrBadText},
		"voice": {Request{Text: "x", Voice: "../anna", Style: StyleSentence}, ErrBadVoice},
		"style": {Request{Text: "x", Voice: "anna", Style: "song"}, ErrBadStyle},
	} {
		// Endpoint "" would be ErrUnavailable: seeing the 400-class error proves order.
		if _, err := s.Synthesize(context.Background(), "", tc.r); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v, want %v", name, err, tc.want)
		}
	}
}

func TestPathAcceptsOnlyTheCacheShape(t *testing.T) {
	s := New("/cache")
	good := strings.Repeat("ab", 32) + ".wav"
	if p := s.Path(good); p != filepath.Join("/cache", "ab", good) {
		t.Errorf("Path = %q", p)
	}
	for _, bad := range []string{
		"", "../" + good, good + "/..", "/" + good, strings.Repeat("AB", 32) + ".wav",
		strings.Repeat("ab", 31) + ".wav", strings.Repeat("ab", 33) + ".wav", strings.Repeat("ab", 32) + ".mp3",
		strings.Repeat("ab", 32) + ".wav\n", "ab/" + good, strings.Repeat("g", 64) + ".wav",
	} {
		if s.Path(bad) != "" || ValidFile(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestPruneDropsOldestFirstDownToTarget(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	s.MaxBytes, s.TargetBytes = 1000, 500
	mk := func(i int, age time.Duration) string {
		name := strings.Repeat(string(rune('a'+i)), 64) + ".wav"
		p := s.Path(name)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, make([]byte, 300), 0o644); err != nil {
			t.Fatal(err)
		}
		ts := time.Now().Add(-age)
		_ = os.Chtimes(p, ts, ts)
		return p
	}
	oldest, old, mid, fresh := mk(0, 4*time.Hour), mk(1, 3*time.Hour), mk(2, 2*time.Hour), mk(3, time.Hour)
	removed := s.Prune() // 1200 > 1000 -> down to <= 500 means keep one file
	if removed != 900 {
		t.Errorf("removed %d, want 900", removed)
	}
	for _, p := range []string{oldest, old, mid} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s survived", filepath.Base(p))
		}
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error("the newest file was pruned")
	}
	// Under the limit nothing is touched.
	if s.Prune() != 0 {
		t.Error("pruned a cache under its limit")
	}
}

func TestWriteTriggersPrune(t *testing.T) {
	child := newChild(t)
	s := New(t.TempDir())
	s.MaxBytes, s.TargetBytes = 5000, 1 // each rendering is 4044 bytes
	for _, txt := range []string{"egy", "kettő"} {
		if _, err := s.Synthesize(context.Background(), child.URL, Request{Text: txt, Voice: "anna", Style: StyleSentence}); err != nil {
			t.Fatal(err)
		}
	}
	var left int
	_ = filepath.WalkDir(s.dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			left++
		}
		return nil
	})
	if left > 1 {
		t.Errorf("%d files left after exceeding the cap", left)
	}
}

func TestFollowerRetriesWhenTheLeadersContextDies(t *testing.T) {
	child := newChild(t)
	child.delay.Store(int64(300 * time.Millisecond))
	s := New(t.TempDir())
	r := Request{Text: "Közös mondat.", Voice: "anna", Style: StyleSentence}

	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	leaderErr := make(chan error, 1)
	go func() {
		_, err := s.Synthesize(leaderCtx, child.URL, r)
		leaderErr <- err
	}()
	// Wait until the leader's flight exists.
	for i := 0; i < 200; i++ {
		s.mu.Lock()
		n := len(s.flights)
		s.mu.Unlock()
		if n == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	followerRes := make(chan error, 1)
	go func() {
		_, err := s.Synthesize(context.Background(), child.URL, r)
		followerRes <- err
	}()
	time.Sleep(50 * time.Millisecond)
	cancelLeader()
	if err := <-leaderErr; err == nil {
		t.Error("the cancelled leader succeeded")
	}
	if err := <-followerRes; err != nil {
		t.Errorf("follower inherited the leader's failure: %v", err)
	}
	if n := child.synth.Load(); n != 2 {
		t.Errorf("child synthesis calls = %d, want 2 (aborted leader + retrying follower)", n)
	}
}

type panicTransport struct {
	next  http.RoundTripper
	panic atomic.Bool
}

func (p *panicTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if p.panic.Load() && strings.HasSuffix(r.URL.Path, "/synthesize") {
		panic("boom")
	}
	return p.next.RoundTrip(r)
}

func TestPanicInRenderLeavesNoStuckFlight(t *testing.T) {
	child := newChild(t)
	s := New(t.TempDir())
	pt := &panicTransport{next: http.DefaultTransport}
	pt.panic.Store(true)
	s.client = &http.Client{Transport: pt}
	r := Request{Text: "Pánik.", Voice: "anna", Style: StyleSentence}
	func() {
		defer func() { _ = recover() }()
		_, _ = s.Synthesize(context.Background(), child.URL, r)
	}()
	s.mu.Lock()
	n := len(s.flights)
	s.mu.Unlock()
	if n != 0 {
		t.Fatalf("%d flights left after a panic", n)
	}
	// The semaphore was released too: a later call works.
	pt.panic.Store(false)
	if _, err := s.Synthesize(context.Background(), child.URL, r); err != nil {
		t.Fatalf("after panic: %v", err)
	}
}

func TestPruneNeverTouchesInFlightTempFiles(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	s.MaxBytes, s.TargetBytes = 1000, 100
	sub := filepath.Join(dir, "ab")
	_ = os.MkdirAll(sub, 0o755)
	tmp := filepath.Join(sub, ".w-12345")
	stranger := filepath.Join(sub, "notes.txt")
	for _, p := range []string{tmp, stranger} {
		if err := os.WriteFile(p, make([]byte, 5000), 0o644); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-48 * time.Hour)
		_ = os.Chtimes(p, old, old)
	}
	done := filepath.Join(sub, strings.Repeat("c", 64)+".wav")
	if err := os.WriteFile(done, make([]byte, 1200), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Prune()
	if _, err := os.Stat(tmp); err != nil {
		t.Error("an in-flight temp file was pruned")
	}
	if _, err := os.Stat(stranger); err != nil {
		t.Error("a non-cache file was pruned")
	}
	if _, err := os.Stat(done); err == nil {
		t.Error("the completed rendering over the cap survived")
	}
}
