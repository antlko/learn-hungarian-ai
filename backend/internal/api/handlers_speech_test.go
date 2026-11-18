package api

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/components"
	"tanulom/internal/components/bundlefix"
	"tanulom/internal/speech"
)

// ttsRuntime points the manager at a fixture offline bundle for linux-amd64
// (whatever the host is), so the enable pipeline runs for real on a stub
// interpreter and never touches the network.
func ttsRuntime(t *testing.T) func(*components.Options) {
	t.Helper()
	dir := t.TempDir()
	if err := bundlefix.Write(dir, "linux-amd64", bundlefix.Options{}); err != nil {
		t.Fatal(err)
	}
	return func(o *components.Options) { o.TTSRuntimeDir, o.GOOS, o.GOARCH = dir, "linux", "amd64" }
}

func speechWAV(seconds int) []byte {
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

// ttsChild is the managed `tts` service: /health, /voices, /synthesize.
type ttsChild struct {
	port  int
	synth atomic.Int32
	delay atomic.Int64
}

func newTTSChild(t *testing.T) *ttsChild {
	t.Helper()
	c := &ttsChild{}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ready"})
	})
	mux.HandleFunc("/voices", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]speech.Voice{
			{ID: "imre", SampleRate: 22050, ModelHash: "i1"},
			{ID: "anna", SampleRate: 22050, ModelHash: "a1"},
		})
	})
	mux.HandleFunc("/synthesize", func(w http.ResponseWriter, r *http.Request) {
		c.synth.Add(1)
		select {
		case <-time.After(time.Duration(c.delay.Load())):
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write(speechWAV(3))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	_, p, _ := strings.Cut(srv.Listener.Addr().String(), ":")
	for _, ch := range p {
		c.port = c.port*10 + int(ch-'0')
	}
	return c
}

// newSpeechServer is an admin server whose tts component is installed (against
// fake Python/venv) with the named personas' files present, and running as the
// fake child.
func newSpeechServer(t *testing.T) (*Server, *client, *ttsChild) {
	t.Helper()
	s, c := newAdmin(t)
	voices := t.TempDir()
	for _, p := range components.Personas {
		for _, suf := range []string{"", ".json"} {
			if err := os.WriteFile(filepath.Join(voices, components.VoiceFile(p)+suf), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	child := newTTSChild(t)
	withFakeManager(t, s, child.port, ttsRuntime(t), func(o *components.Options) { o.VoicesDir = voices })
	if resp, _ := c.do(http.MethodPost, "/api/admin/components/tts/install", nil); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("install tts = %d", resp.StatusCode)
	}
	waitManaged(t, c, "tts", "running")
	return s, c, child
}

func TestSpeechVoicesUnavailableWithoutTheComponent(t *testing.T) {
	t.Parallel()
	_, c := newAdmin(t)
	var v SpeechVoicesDTO
	if resp := c.json(http.MethodGet, "/api/speech/voices", nil, &v); resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if v.Available || v.Voices == nil || len(v.Voices) != 0 {
		t.Errorf("voices = %+v, want available:false and an empty (non-null) list", v)
	}
	anon := &client{t: t, app: c.app}
	if resp := anon.json(http.MethodGet, "/api/speech/voices", nil, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous voices = %d", resp.StatusCode)
	}
	var e struct{ Error string }
	if resp := c.json(http.MethodPost, "/api/speech/tts", SpeechTTSRequest{Text: "Szia", Voice: "anna", Style: "sentence"}, &e); resp.StatusCode != http.StatusServiceUnavailable || e.Error != "errors.ttsUnavailable" {
		t.Errorf("tts with no component: %d %q", resp.StatusCode, e.Error)
	}
}

func TestComponentInstallWithoutVoicesIsRefusedAndListsNone(t *testing.T) {
	t.Parallel()
	s, c := newAdmin(t)
	withFakeManager(t, s, 1, ttsRuntime(t), func(o *components.Options) { o.VoicesDir = t.TempDir() })
	var e struct{ Error string }
	if resp := c.json(http.MethodPost, "/api/admin/components/tts/install", nil, &e); resp.StatusCode != http.StatusUnprocessableEntity || e.Error != "errors.componentNoVoices" {
		t.Fatalf("install without voices: %d %q", resp.StatusCode, e.Error)
	}
	m := managedOf(t, c, "tts")
	if m == nil || m.State != "absent" || len(m.Voices) != 0 {
		t.Errorf("tts managed = %+v", m)
	}
	var rows []ComponentDTO
	c.json(http.MethodGet, "/api/admin/components", nil, &rows)
	for _, r := range rows {
		if r.ID == "tts" && (!r.Built || r.ComposeProfile != "tts") {
			t.Errorf("tts row = %+v", r)
		}
	}
}

func TestSpeechVoicesTTSAndAudioHappyPath(t *testing.T) {
	t.Parallel()
	s, c, child := newSpeechServer(t)

	// The admin card lists the personas found, in order.
	if m := managedOf(t, c, "tts"); strings.Join(m.Voices, ",") != "anna,berta,imre" {
		t.Errorf("managed voices = %v", m.Voices)
	}
	var v SpeechVoicesDTO
	c.json(http.MethodGet, "/api/speech/voices", nil, &v)
	if !v.Available || len(v.Voices) != 2 || v.Voices[0] != (SpeechVoiceDTO{ID: "anna", Name: "Anna", Gender: "female"}) ||
		v.Voices[1] != (SpeechVoiceDTO{ID: "imre", Name: "Imre", Gender: "male"}) {
		t.Errorf("voices = %+v (the child serves anna and imre only)", v)
	}

	var out SpeechTTSResponse
	req := SpeechTTSRequest{Text: "Szia! Ma együtt olvasunk.", Voice: "anna", Style: "sentence"}
	if resp := c.json(http.MethodPost, "/api/speech/tts", req, &out); resp.StatusCode != 200 {
		t.Fatalf("tts = %d", resp.StatusCode)
	}
	if !strings.HasPrefix(out.URL, "/api/speech/audio/") || !strings.HasSuffix(out.URL, ".wav") || out.DurationS < 2.99 || out.DurationS > 3.01 {
		t.Fatalf("response = %+v", out)
	}
	// Replay: same URL, no second synthesis.
	var again SpeechTTSResponse
	c.json(http.MethodPost, "/api/speech/tts", req, &again)
	if again != out || child.synth.Load() != 1 {
		t.Errorf("replay %+v, synth calls %d", again, child.synth.Load())
	}
	// Style omitted means sentence: same key, still one synthesis.
	c.json(http.MethodPost, "/api/speech/tts", SpeechTTSRequest{Text: req.Text, Voice: "anna"}, &again)
	if again != out || child.synth.Load() != 1 {
		t.Errorf("empty style: %+v, synth calls %d", again, child.synth.Load())
	}

	resp, body := c.do(http.MethodGet, out.URL, nil)
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "audio/wav" ||
		!strings.Contains(resp.Header.Get("Cache-Control"), "immutable") || !strings.Contains(resp.Header.Get("Cache-Control"), "private") ||
		len(body) != len(speechWAV(3)) {
		t.Fatalf("audio: %d %q %q %d bytes", resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header.Get("Cache-Control"), len(body))
	}
	// Range, so the browser can seek.
	rq := httptest.NewRequest(http.MethodGet, out.URL, nil)
	rq.Header.Set("Cookie", c.cookie)
	rq.Header.Set("Range", "bytes=0-9")
	rr, err := s.App().Test(rq, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	part, _ := io.ReadAll(rr.Body)
	rr.Body.Close()
	if rr.StatusCode != http.StatusPartialContent || len(part) != 10 {
		t.Errorf("range: %d, %d bytes", rr.StatusCode, len(part))
	}
	// The cache is outside /media: it is not reachable without the auth check.
	anon := &client{t: t, app: s}
	if resp, _ := anon.do(http.MethodGet, out.URL, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous audio = %d", resp.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(s.dataDir, "cache", "tts", out.URL[len("/api/speech/audio/"):][:2])); err != nil {
		t.Errorf("cache not under <data>/cache/tts: %v", err)
	}
}

func TestSpeechTTSRejectsBadInputWith400(t *testing.T) {
	t.Parallel()
	_, c, child := newSpeechServer(t)
	for name, req := range map[string]SpeechTTSRequest{
		"empty":     {Text: "  \n", Voice: "anna", Style: "sentence"},
		"too long":  {Text: strings.Repeat("a", speech.MaxTextBytes+1), Voice: "anna", Style: "sentence"},
		"voice":     {Text: "Szia", Voice: "../../etc/passwd", Style: "sentence"},
		"no voice":  {Text: "Szia", Style: "sentence"},
		"style":     {Text: "Szia", Voice: "anna", Style: "opera"},
		"bad voice": {Text: "Szia", Voice: "Anna", Style: "word"},
	} {
		if resp := c.json(http.MethodPost, "/api/speech/tts", req, nil); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", name, resp.StatusCode)
		}
	}
	if resp, _ := c.do(http.MethodPost, "/api/speech/tts", "not an object"); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("malformed body: %d", resp.StatusCode)
	}
	if child.synth.Load() != 0 {
		t.Errorf("a refused request reached the child (%d)", child.synth.Load())
	}
}

func TestSpeechTTS503WhenStoppedOrPausedAnd504WhenSlow(t *testing.T) {
	t.Parallel()
	s, c, child := newSpeechServer(t)
	req := SpeechTTSRequest{Text: "Lassú mondat.", Voice: "anna", Style: "sentence"}

	// 504: the child answers after the deadline.
	s.speech.Timeout = 150 * time.Millisecond
	child.delay.Store(int64(3 * time.Second))
	var e struct{ Error string }
	if resp := c.json(http.MethodPost, "/api/speech/tts", req, &e); resp.StatusCode != http.StatusGatewayTimeout || e.Error != "errors.ttsTimeout" {
		t.Errorf("slow child: %d %q", resp.StatusCode, e.Error)
	}
	s.speech.Timeout = 30 * time.Second
	child.delay.Store(0)
	if resp := c.json(http.MethodPost, "/api/speech/tts", req, nil); resp.StatusCode != 200 {
		t.Fatalf("recovered request = %d", resp.StatusCode)
	}

	// Paused resolves to off (ADR-068): 503, voices unavailable.
	if resp := c.json(http.MethodPost, "/api/admin/services/tts/paused", PausedRequest{Paused: true}, nil); resp.StatusCode != 200 {
		t.Fatalf("pause = %d", resp.StatusCode)
	}
	e.Error = ""
	if resp := c.json(http.MethodPost, "/api/speech/tts", SpeechTTSRequest{Text: "Más mondat.", Voice: "anna", Style: "sentence"}, &e); resp.StatusCode != http.StatusServiceUnavailable || e.Error != "errors.ttsUnavailable" {
		t.Errorf("paused: %d %q", resp.StatusCode, e.Error)
	}
	var v SpeechVoicesDTO
	c.json(http.MethodGet, "/api/speech/voices", nil, &v)
	if v.Available {
		t.Errorf("paused service still lists voices: %+v", v)
	}
	// Un-pause then stop the child: still 503, not a 500.
	c.json(http.MethodPost, "/api/admin/services/tts/paused", PausedRequest{Paused: false}, nil)
	if resp, _ := c.do(http.MethodPost, "/api/admin/components/tts/stop", nil); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("stop = %d", resp.StatusCode)
	}
	if resp := c.json(http.MethodPost, "/api/speech/tts", SpeechTTSRequest{Text: "Harmadik mondat.", Voice: "anna", Style: "sentence"}, nil); resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("stopped: %d", resp.StatusCode)
	}
}

func TestSpeechAudioOnlyServesCacheShapedNames(t *testing.T) {
	t.Parallel()
	s, c, _ := newSpeechServer(t)
	// A secret next to the cache must stay unreachable through any spelling.
	if err := os.WriteFile(filepath.Join(s.dataDir, "secret.wav"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	hex64 := strings.Repeat("ab", 32)
	for _, name := range []string{
		"secret.wav", "..%2Fsecret.wav", "%2e%2e%2fsecret.wav", "..%2F..%2Fconfig.yaml", "ab%2F" + hex64 + ".wav",
		strings.ToUpper(hex64) + ".wav", hex64 + ".mp3", hex64, hex64 + ".wav%00", hex64[:62] + ".wav", "%2Fetc%2Fpasswd",
	} {
		resp, body := c.do(http.MethodGet, "/api/speech/audio/"+name, nil)
		if resp.StatusCode != http.StatusNotFound || strings.Contains(string(body), "secret") {
			t.Errorf("%q: %d %q, want 404", name, resp.StatusCode, body)
		}
	}
	// A well-formed name that was never rendered is a 404 as well.
	if resp, _ := c.do(http.MethodGet, "/api/speech/audio/"+hex64+".wav", nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown but valid name: %d", resp.StatusCode)
	}
	// Raw ../ segments never route here at all.
	if resp, _ := c.do(http.MethodGet, "/api/speech/audio/../../secret.wav", nil); resp.StatusCode == 200 {
		t.Errorf("raw ../ traversal answered 200")
	}
}

func TestSpeechTTSAnonymousIs401AndOversizeBodyIsRefusedEarly(t *testing.T) {
	t.Parallel()
	s, c, child := newSpeechServer(t)
	anon := &client{t: t, app: s}
	if resp := anon.json(http.MethodPost, "/api/speech/tts", SpeechTTSRequest{Text: "Szia", Voice: "anna", Style: "sentence"}, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous tts = %d, want 401", resp.StatusCode)
	}
	big := SpeechTTSRequest{Text: strings.Repeat("a", 20<<10), Voice: "anna", Style: "sentence"}
	if resp := c.json(http.MethodPost, "/api/speech/tts", big, nil); resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("oversize body = %d, want 413", resp.StatusCode)
	}
	if child.synth.Load() != 0 {
		t.Error("an oversize request reached the child")
	}
}

// Enable and Disable ride the install/stop routes (ADR-080): Enable hands the
// provider over to `managed`, Disable puts it back on the device voice, and both
// survive a restart because they go through updateConfig.
func TestTtsEnableDisableSwitchTheProviderAndOffersNoDownload(t *testing.T) {
	s, c, _ := newSpeechServer(t)
	if got := serviceOf(t, c, "tts"); got.Provider != "managed" {
		t.Fatalf("after Enable: %+v", got)
	}
	m := managedOf(t, c, "tts")
	if !m.Bundled || !m.Available || m.State != "running" || strings.Join(m.Voices, ",") != "anna,berta,imre" {
		t.Fatalf("managed = %+v", m)
	}
	if resp, _ := c.do(http.MethodPost, "/api/admin/components/tts/install", nil); resp.StatusCode != http.StatusConflict {
		t.Errorf("Enable while on = %d, want 409", resp.StatusCode)
	}
	if resp, _ := c.do(http.MethodPost, "/api/admin/components/tts/stop", nil); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("Disable = %d", resp.StatusCode)
	}
	if got := serviceOf(t, c, "tts"); got.Provider != "device" || got.Endpoint != "" {
		t.Fatalf("after Disable: %+v", got)
	}
	if got := s.Config().Services.TTS.Provider; got != "device" {
		t.Errorf("config provider = %q", got)
	}
	if m := managedOf(t, c, "tts"); m.State != "stopped" {
		t.Errorf("state after Disable = %q", m.State)
	}
	// The start route is Enable for a bundled component, and re-selects managed.
	if resp, _ := c.do(http.MethodPost, "/api/admin/components/tts/start", nil); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("start = %d", resp.StatusCode)
	}
	waitManaged(t, c, "tts", "running")
	if got := serviceOf(t, c, "tts"); got.Provider != "managed" {
		t.Fatalf("after re-Enable: %+v", got)
	}
	// Uninstall = remove the unpacked environment; the provider goes to the floor, not `off`.
	if resp, _ := c.do(http.MethodPost, "/api/admin/components/tts/uninstall", nil); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("uninstall = %d", resp.StatusCode)
	}
	if got := serviceOf(t, c, "tts"); got.Provider != "device" {
		t.Errorf("provider after uninstall: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(s.dataDir, "components", "tts", "python")); err == nil {
		t.Error("the unpacked runtime remains")
	}
	if m := managedOf(t, c, "tts"); m.State != "absent" || !m.Available {
		t.Errorf("after uninstall: %+v", m)
	}
}

func TestTtsWithoutARuntimeIsUnavailableAndRefusesEnable(t *testing.T) {
	t.Parallel()
	s, c := newAdmin(t)
	withFakeManager(t, s, 1, func(o *components.Options) { o.GOOS, o.GOARCH = "darwin", "amd64"; o.TTSRuntimeDir = t.TempDir() })
	m := managedOf(t, c, "tts")
	if m.Available || !m.Bundled || m.UnavailableKey != "errors.componentNoTtsRuntime" {
		t.Fatalf("managed = %+v", m)
	}
	var e struct{ Error string }
	if resp := c.json(http.MethodPost, "/api/admin/components/tts/install", nil, &e); resp.StatusCode != http.StatusUnprocessableEntity || e.Error != "errors.componentNoTtsRuntime" {
		t.Fatalf("Enable without a runtime: %d %q", resp.StatusCode, e.Error)
	}
	if got := serviceOf(t, c, "tts"); got.Provider == "managed" {
		t.Errorf("provider = %+v", got)
	}
}
