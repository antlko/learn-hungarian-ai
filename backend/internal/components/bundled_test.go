package components

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tanulom/internal/components/bundlefix"
	"tanulom/internal/components/ttsfetch"
)

// ---- the offline tts runtime (ADR-080) ----
//
// A fixture bundle (bundlefix) stands in for services/tts/vendor: its "python"
// is a shell script that records argv and env. Nothing here unpacks the real
// 25 MB interpreter, opens a socket or needs Python.

const fixturePlatform = "linux-amd64"

// hybridRunner runs the pip step for real (the stub interpreter) and fakes
// everything else, recording every command either way.
type hybridRunner struct {
	*fakeRunner
	block chan struct{} // when set, pip waits on it
}

func (r *hybridRunner) Run(ctx context.Context, c Cmd, onLine func(string)) error {
	if !strings.Contains(strings.Join(c.Args, " "), "pip install") {
		return r.fakeRunner.Run(ctx, c, onLine)
	}
	r.mu.Lock()
	r.runs = append(r.runs, c)
	r.mu.Unlock()
	if r.block != nil {
		select {
		case <-r.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return ExecRunner{}.Run(ctx, c, onLine)
}

func (r *hybridRunner) pipRuns() []Cmd {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Cmd
	for _, c := range r.runs {
		if strings.Contains(strings.Join(c.Args, " "), "pip install") {
			out = append(out, c)
		}
	}
	return out
}

type bundledHarness struct {
	*harness
	hr     *hybridRunner
	bundle string // the runtime dir
	voices string // the --voices-dir
	steps  []string
	smu    sync.Mutex
}

func newBundledHarness(t *testing.T, bo bundlefix.Options, mods ...func(*Options)) *bundledHarness {
	t.Helper()
	bundle, voices := t.TempDir(), t.TempDir()
	if err := bundlefix.Write(bundle, fixturePlatform, bo); err != nil {
		t.Fatal(err)
	}
	writeVoices(t, voices, "anna", "berta", "imre")
	bh := &bundledHarness{bundle: bundle, voices: voices}
	bh.harness = newHarness(t, append([]func(*Options){func(o *Options) {
		o.TTSRuntimeDir, o.VoicesDir = bundle, voices
		o.GOOS, o.GOARCH = "linux", "amd64"
		o.OnLog = func(_, line string) {
			if s, ok := strings.CutPrefix(line, "step: "); ok {
				bh.smu.Lock()
				bh.steps = append(bh.steps, s)
				bh.smu.Unlock()
			}
		}
	}}, mods...)...)
	bh.hr = &hybridRunner{fakeRunner: bh.r}
	bh.m.opt.Runner = bh.hr
	return bh
}

func (b *bundledHarness) stepList() []string {
	b.smu.Lock()
	defer b.smu.Unlock()
	return append([]string(nil), b.steps...)
}

func (b *bundledHarness) failedWith(t *testing.T) Status {
	t.Helper()
	if err := b.m.Install(context.Background(), TTS); err != nil {
		t.Fatal(err)
	}
	return b.waitState(t, TTS, StateFailed, true)
}

func (b *bundledHarness) root() string { return b.m.final(TTS) }

func (b *bundledHarness) noResidue(t *testing.T) {
	t.Helper()
	for _, p := range []string{"python", stampName, ".unpack"} {
		if _, err := os.Stat(filepath.Join(b.root(), p)); err == nil {
			t.Errorf("%s was left behind", p)
		}
	}
}

// writeVoices puts empty stand-ins for the named personas' files into dir.
func writeVoices(t *testing.T, dir string, personas ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range personas {
		for _, suffix := range []string{"", ".json"} {
			if err := os.WriteFile(filepath.Join(dir, VoiceFile(p)+suffix), []byte("x-"+p), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestEnableRunsTheStepsInOrderAndStartsTheBundledInterpreter(t *testing.T) {
	b := newBundledHarness(t, bundlefix.Options{})
	st := b.install(t, TTS)
	if !st.Bundled {
		t.Error("tts is not reported as bundled")
	}
	if got := strings.Join(b.stepList(), ","); got != StepVerify+","+StepUnpack+","+StepVoices {
		t.Errorf("steps = %q", got)
	}
	if got := strings.Join(st.Voices, ","); got != "anna,berta,imre" {
		t.Errorf("voices = %q", got)
	}
	if len(b.hr.starts) != 1 {
		t.Fatalf("starts = %d", len(b.hr.starts))
	}
	c := b.hr.starts[0]
	if c.Path != filepath.Join(b.root(), "python", "bin", "python3") {
		t.Errorf("child path = %q", c.Path)
	}
	if len(c.Args) != 2 || c.Args[0] != "-I" || c.Args[1] != filepath.Join(b.root(), "src", "app.py") {
		t.Errorf("child args = %v", c.Args)
	}
	for k, want := range map[string]string{"TTS_PRELOAD": "all", "TTS_HOST": "127.0.0.1", "TTS_VOICES_DIR": b.voices, "HOME": filepath.Join(b.root(), "home")} {
		if envOf(c, k) != want {
			t.Errorf("%s = %q, want %q", k, envOf(c, k), want)
		}
	}
	if envOf(c, "TTS_PORT") == "" {
		t.Error("no TTS_PORT")
	}
	if len(b.installs) != 1 || b.installs[0] != TTS {
		t.Errorf("OnInstalled = %v", b.installs)
	}
	// What ran is what the binary carries: only app.py, no requirements file.
	if _, err := os.Stat(filepath.Join(b.root(), "src", "requirements.txt")); err == nil {
		t.Error("a requirements.txt was written for tts")
	}
	for _, p := range []string{"python/bin/python3", "python/lib/site-packages/piper/__init__.py", stampName} {
		if _, err := os.Stat(filepath.Join(b.root(), p)); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
	if fi, _ := os.Stat(filepath.Join(b.root(), "python", "bin", "python3.12")); fi == nil || fi.Mode().Perm()&0o100 == 0 {
		t.Error("the interpreter lost its execute bit")
	}
	if _, err := os.Stat(filepath.Join(b.root(), ".unpack")); err == nil {
		t.Error("the staging directory survived")
	}
}

// The offline guarantee: nothing the manager runs for tts can reach a network.
func TestNoTtsCommandCanTouchTheNetwork(t *testing.T) {
	for k, v := range map[string]string{
		"HTTPS_PROXY": "http://proxy.invalid:3128", "http_proxy": "http://proxy.invalid:3128", "ALL_PROXY": "socks5://x",
		"PIP_INDEX_URL": "https://evil.invalid/simple", "PIP_EXTRA_INDEX_URL": "https://evil.invalid", "PYTHONPATH": "/evil",
		"PYTHONHOME": "/evil", "TANULOM_SECRET": "s3cret", "SSL_CERT_FILE": "/etc/ssl/x",
	} {
		t.Setenv(k, v)
	}
	b := newBundledHarness(t, bundlefix.Options{})
	b.install(t, TTS)

	var all []Cmd
	b.hr.mu.Lock()
	for _, c := range b.hr.runs {
		if strings.Contains(strings.Join(c.Args, " "), "pip install") {
			all = append(all, c)
		}
	}
	all = append(all, b.hr.starts...)
	b.hr.mu.Unlock()
	pips := b.hr.pipRuns()
	if len(pips) != 1 {
		t.Fatalf("pip runs = %d", len(pips))
	}
	for _, c := range all {
		line := c.Path + " " + strings.Join(c.Args, " ")
		if strings.Contains(line, "://") {
			t.Errorf("a tts command line carries a URL: %s", line)
		}
		for _, kv := range c.Env {
			k, v, _ := strings.Cut(kv, "=")
			ku := strings.ToUpper(k)
			if strings.Contains(ku, "PROXY") || strings.HasPrefix(ku, "TANULOM_") || strings.Contains(v, "evil") || strings.Contains(v, "://") ||
				ku == "PYTHONPATH" || ku == "PYTHONHOME" || ku == "SSL_CERT_FILE" {
				t.Errorf("env leaks %s", k)
			}
			if strings.HasPrefix(ku, "PIP_") && !map[string]bool{"PIP_NO_INDEX": true, "PIP_CONFIG_FILE": true, "PIP_DISABLE_PIP_VERSION_CHECK": true, "PIP_NO_INPUT": true, "PIP_NO_CACHE_DIR": true}[ku] {
				t.Errorf("env carries %s", k)
			}
		}
	}
	p := pips[0]
	joined := " " + strings.Join(p.Args, " ") + " "
	for _, want := range []string{" -I ", " -m pip install ", " --no-index ", " --require-hashes ", " --no-deps ", " --only-binary=:all: ", " --find-links "} {
		if !strings.Contains(joined, want) {
			t.Errorf("pip argv lacks %q: %s", want, joined)
		}
	}
	for _, bad := range []string{"--index-url", "--extra-index-url", "-i ", "--proxy", "--trusted-host"} {
		if strings.Contains(joined, bad) {
			t.Errorf("pip argv has %q", bad)
		}
	}
	if envOf(p, "PIP_NO_INDEX") != "1" || envOf(p, "PIP_CONFIG_FILE") != "/dev/null" {
		t.Errorf("pip env: NO_INDEX=%q CONFIG_FILE=%q", envOf(p, "PIP_NO_INDEX"), envOf(p, "PIP_CONFIG_FILE"))
	}
	// The real process (the stub) saw the same scrubbed environment.
	env, err := os.ReadFile(filepath.Join(b.root(), "python", "env.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"PROXY", "proxy", "evil", "TANULOM_", "PIP_INDEX_URL", "PYTHONPATH", "SSL_CERT"} {
		if bytes.Contains(env, []byte(bad)) {
			t.Errorf("the pip process inherited %q", bad)
		}
	}
	if !bytes.Contains(env, []byte("PIP_NO_INDEX=1")) {
		t.Error("the pip process did not get PIP_NO_INDEX=1")
	}
}

func TestEnableDisableCycleReusesTheUnpackedRuntime(t *testing.T) {
	b := newBundledHarness(t, bundlefix.Options{})
	b.install(t, TTS)
	if err := b.m.Install(context.Background(), TTS); !errors.Is(err, ErrInstalled) {
		t.Fatalf("Enable while on = %v, want ErrInstalled", err)
	}
	if err := b.m.Stop(TTS); err != nil {
		t.Fatal(err)
	}
	if st := b.m.Status(TTS); st.State != StateStopped || b.m.Endpoint(TTS) != "" {
		t.Errorf("after Stop: %q endpoint %q", st.State, b.m.Endpoint(TTS))
	}
	if !b.m.Installed(TTS) {
		t.Error("Disable removed the unpacked runtime")
	}
	before := len(b.hr.pipRuns())
	b.steps = nil
	b.install(t, TTS)
	if len(b.hr.pipRuns()) != before {
		t.Error("re-enabling ran pip again although the stamp is current")
	}
	if got := strings.Join(b.stepList(), ","); got != StepVoices {
		t.Errorf("steps on re-enable = %q, want only %s", got, StepVoices)
	}
	if b.r.startCount() != 2 {
		t.Errorf("starts = %d", b.r.startCount())
	}

	// Uninstall = remove the unpacked environment, never the bundle.
	if err := b.m.Uninstall(TTS); err != nil {
		t.Fatal(err)
	}
	b.noResidue(t)
	if st := b.m.Status(TTS); st.State != StateAbsent || !st.Available {
		t.Errorf("after Uninstall: %q available=%v", st.State, st.Available)
	}
	for _, p := range []string{"MANIFEST.sha256", "requirements.lock", fixturePlatform + "/" + bundlefix.Tarball} {
		if _, err := os.Stat(filepath.Join(b.bundle, p)); err != nil {
			t.Errorf("Uninstall touched the bundle: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(b.voices, VoiceFile("anna"))); err != nil {
		t.Error("Uninstall touched the operator's voices")
	}
	b.install(t, TTS) // and it comes back, offline
}

func TestStaleStampTriggersAFullReExtract(t *testing.T) {
	b := newBundledHarness(t, bundlefix.Options{})
	b.install(t, TTS)
	if err := b.m.Stop(TTS); err != nil {
		t.Fatal(err)
	}
	// A stale environment: leave a marker inside python/ and change the stamp.
	leftover := filepath.Join(b.root(), "python", "STALE")
	if err := os.WriteFile(leftover, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.root(), stampName), []byte("manifest=old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if b.m.runtimeReady(TTS) {
		t.Fatal("a stale stamp still counts as current")
	}
	// ...but the environment is still there: Start and boot keep using it (ADR-081).
	if !b.m.Installed(TTS) {
		t.Fatal("a stale stamp made the environment disappear")
	}
	b.steps = nil
	b.install(t, TTS)
	if got := strings.Join(b.stepList(), ","); got != StepVerify+","+StepUnpack+","+StepVoices {
		t.Errorf("steps = %q", got)
	}
	if _, err := os.Stat(leftover); err == nil {
		t.Error("the stale environment was not replaced")
	}
	if len(b.hr.pipRuns()) != 2 {
		t.Errorf("pip runs = %d, want 2", len(b.hr.pipRuns()))
	}
	// A new bundle (the MANIFEST changes) makes the stamp stale too.
	if err := b.m.Stop(TTS); err != nil {
		t.Fatal(err)
	}
	man := filepath.Join(b.bundle, manifestName)
	raw, _ := os.ReadFile(man)
	if err := os.WriteFile(man, append([]byte("# refreshed\n"), raw...), 0o644); err != nil {
		t.Fatal(err)
	}
	if b.m.runtimeReady(TTS) {
		t.Error("a changed MANIFEST kept the old environment current")
	}
}

func TestCorruptBundleIsRefusedAndLeavesNothingBehind(t *testing.T) {
	flip := func(t *testing.T, path string) {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		raw[len(raw)/2] ^= 0xff
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cases := map[string]func(t *testing.T, b *bundledHarness){
		"a flipped wheel byte": func(t *testing.T, b *bundledHarness) {
			flip(t, filepath.Join(b.bundle, fixturePlatform, "piper_tts-1.8.0-cp39-abi3-fake.whl"))
		},
		"a flipped interpreter byte": func(t *testing.T, b *bundledHarness) {
			flip(t, filepath.Join(b.bundle, fixturePlatform, bundlefix.Tarball))
		},
		"a missing wheel": func(t *testing.T, b *bundledHarness) {
			_ = os.Remove(filepath.Join(b.bundle, fixturePlatform, "onnxruntime-1.32.0-cp312-cp312-fake.whl"))
		},
		"an unlisted file": func(t *testing.T, b *bundledHarness) {
			_ = os.WriteFile(filepath.Join(b.bundle, fixturePlatform, "extra.whl"), []byte("x"), 0o644)
		},
		"a lock that does not pin the wheels": func(t *testing.T, b *bundledHarness) {
			_ = os.WriteFile(filepath.Join(b.bundle, lockName), []byte("piper-tts==1.8.0 --hash=sha256:"+strings.Repeat("0", 64)+"\n"), 0o644)
		},
		"a garbled MANIFEST": func(t *testing.T, b *bundledHarness) {
			_ = os.WriteFile(filepath.Join(b.bundle, manifestName), []byte("not a manifest\n"), 0o644)
		},
	}
	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			b := newBundledHarness(t, bundlefix.Options{})
			corrupt(t, b)
			st := b.failedWith(t)
			if st.ErrorKey != KeyBundleCorrupt {
				t.Errorf("error key = %q (%s), want %s", st.ErrorKey, st.ErrorDetail, KeyBundleCorrupt)
			}
			b.noResidue(t)
			if len(b.hr.pipRuns()) != 0 || b.r.startCount() != 0 {
				t.Error("pip or the child ran over a corrupt bundle")
			}
			if len(b.installs) != 0 {
				t.Error("provider handed over despite the failure")
			}
			if st := b.m.Status(TTS); st.Step != "" {
				t.Errorf("failed state still shows step %q", st.Step)
			}
		})
	}
}

func TestCorruptBundleDoesNotDestroyAWorkingRuntime(t *testing.T) {
	b := newBundledHarness(t, bundlefix.Options{})
	b.install(t, TTS)
	if err := b.m.Stop(TTS); err != nil {
		t.Fatal(err)
	}
	// A new (corrupt) bundle arrives: the old environment is stale but must
	// survive the failed attempt untouched.
	man := filepath.Join(b.bundle, manifestName)
	raw, _ := os.ReadFile(man)
	_ = os.WriteFile(man, append([]byte("# v2\n"), raw...), 0o644)
	wheel := filepath.Join(b.bundle, fixturePlatform, "piper_tts-1.8.0-cp39-abi3-fake.whl")
	_ = os.WriteFile(wheel, []byte("tampered"), 0o644)
	if st := b.failedWith(t); st.ErrorKey != KeyBundleCorrupt {
		t.Fatalf("key = %q", st.ErrorKey)
	}
	if _, err := os.Stat(filepath.Join(b.root(), "python", "bin", "python3")); err != nil {
		t.Error("a failed refresh removed the previous environment")
	}
}

func TestPipFailureLeavesNothingBehind(t *testing.T) {
	b := newBundledHarness(t, bundlefix.Options{PipFail: true})
	st := b.failedWith(t)
	if st.ErrorKey != KeyInstallFailed {
		t.Errorf("key = %q", st.ErrorKey)
	}
	b.noResidue(t)
}

func TestPipHashMismatchIsReportedAsACorruptBundle(t *testing.T) {
	b := newBundledHarness(t, bundlefix.Options{})
	// Make the real step fail the way pip does on a hash mismatch.
	b.hr.fakeRunner.failRun = nil
	b.m.opt.Runner = funcRunner{Runner: b.hr, run: func(ctx context.Context, c Cmd, on func(string)) error {
		if strings.Contains(strings.Join(c.Args, " "), "pip install") {
			on("ERROR: THESE PACKAGES DO NOT MATCH THE HASHES FROM THE REQUIREMENTS FILE.")
			return errors.New("exit status 1")
		}
		return b.hr.Run(ctx, c, on)
	}}
	st := b.failedWith(t)
	if st.ErrorKey != KeyBundleCorrupt {
		t.Errorf("key = %q", st.ErrorKey)
	}
	b.noResidue(t)
}

type runnerFunc func(ctx context.Context, c Cmd, on func(string)) error

type funcRunner struct {
	run runnerFunc
	Runner
}

func (f funcRunner) Run(ctx context.Context, c Cmd, on func(string)) error { return f.run(ctx, c, on) }

func TestUnsupportedPlatformsAndMissingBundlesMakeTheCardUnavailable(t *testing.T) {
	for _, p := range [][2]string{{"darwin", "amd64"}, {"windows", "amd64"}, {"linux", "386"}, {"freebsd", "amd64"}} {
		b := newBundledHarness(t, bundlefix.Options{}, func(o *Options) { o.GOOS, o.GOARCH = p[0], p[1] })
		st := b.m.Status(TTS)
		if st.Available || st.UnavailableKey != KeyNoTtsRuntime || !st.Bundled {
			t.Errorf("%v: available=%v key=%q", p, st.Available, st.UnavailableKey)
		}
		if err := b.m.Install(context.Background(), TTS); errKey(err) != KeyNoTtsRuntime {
			t.Errorf("%v: Install = %v", p, err)
		}
		if err := b.m.Start(TTS); errKey(err) != KeyNoTtsRuntime {
			t.Errorf("%v: Start = %v", p, err)
		}
		b.noResidue(t)
	}
	// This platform, but the bundle has no directory for it.
	b := newBundledHarness(t, bundlefix.Options{}, func(o *Options) { o.GOARCH = "arm64" })
	if st := b.m.Status(TTS); st.Available || st.UnavailableKey != KeyNoTtsRuntime {
		t.Errorf("bundle without the platform: %+v", st)
	}
	// Download mode, but the pinned manifest has no files for this platform.
	b = newBundledHarness(t, bundlefix.Options{}, func(o *Options) {
		o.TTSRuntimeDir = ""
		o.Manifests = &ttsfetch.Manifests{Vendor: []byte("# nothing\n"), Voices: []byte("# nothing\n")}
	})
	if st := b.m.Status(TTS); st.Available || st.UnavailableKey != KeyNoTtsRuntime {
		t.Errorf("manifest without the platform: %+v", st)
	}
	// mt is untouched by any of this.
	if st := b.m.Status(MT); st.Bundled || st.UnavailableKey == KeyNoTtsRuntime {
		t.Errorf("mt: %+v", st)
	}
}

func TestVoicesResolveFlagThenInstalledThenDataDir(t *testing.T) {
	b := newBundledHarness(t, bundlefix.Options{})
	// 1. the flag dir has voices: it wins.
	if d, v := b.m.voiceSource(); d != b.voices || len(v) != 3 {
		t.Errorf("flag: %q %v", d, v)
	}
	// 2. the flag dir is empty: the voices installed under components/tts/voices.
	b.m.opt.VoicesDir = t.TempDir()
	writeVoices(t, filepath.Join(b.root(), "voices"), "anna", "berta")
	if d, v := b.m.voiceSource(); d != filepath.Join(b.root(), "voices") || strings.Join(v, ",") != "anna,berta" {
		t.Errorf("installed: %q %v", d, v)
	}
	// 3. neither: <data>/voices.
	_ = os.RemoveAll(filepath.Join(b.root(), "voices"))
	writeVoices(t, filepath.Join(b.dir, "voices"), "imre")
	if d, v := b.m.voiceSource(); d != filepath.Join(b.dir, "voices") || strings.Join(v, ",") != "imre" {
		t.Errorf("data dir: %q %v", d, v)
	}
	// With an operator runtime tree and no voices at all: refused with the voices key,
	// before anything is unpacked.
	b2 := newBundledHarness(t, bundlefix.Options{}, func(o *Options) { o.VoicesDir = t.TempDir() })
	if err := b2.m.Install(context.Background(), TTS); errKey(err) != KeyNoVoices {
		t.Fatalf("Install without voices = %v", err)
	}
	b2.noResidue(t)
	// A model without its .onnx.json is not a voice.
	_ = os.WriteFile(filepath.Join(b2.m.opt.VoicesDir, VoiceFile("anna")), []byte("x"), 0o644)
	if err := b2.m.Install(context.Background(), TTS); errKey(err) != KeyNoVoices {
		t.Fatalf("a lone .onnx counted as a voice: %v", err)
	}
}

func TestVoicesDirDefaultsToDataVoices(t *testing.T) {
	h := newHarness(t)
	if got, want := h.m.VoicesDir(), filepath.Join(h.dir, "voices"); got != want {
		t.Errorf("VoicesDir = %q, want %q", got, want)
	}
	writeVoices(t, h.m.VoicesDir(), "berta")
	if v := h.m.Voices(); len(v) != 1 || v[0] != "berta" {
		t.Errorf("Voices = %v", v)
	}
}

func TestBootNeverBlocksOnUnpacking(t *testing.T) {
	gate := make(chan struct{})
	b := newBundledHarness(t, bundlefix.Options{})
	b.hr.block = gate
	started := time.Now()
	b.m.AutoStart(func(id string) bool { return id == TTS })
	if d := time.Since(started); d > time.Second {
		t.Fatalf("AutoStart took %s", d)
	}
	st := b.waitState(t, TTS, StateInstalling, false)
	if st.Step != StepUnpack && st.Step != StepVerify {
		t.Errorf("step while unpacking = %q", st.Step)
	}
	// An unwanted provider does nothing at all.
	b.m.AutoStart(func(string) bool { return false })
	close(gate)
	b.waitState(t, TTS, StateRunning, true)
	if len(b.installs) != 0 {
		t.Errorf("boot rewrote the provider: %v", b.installs)
	}
}

func TestAutoStartWithACurrentStampStartsStraightAway(t *testing.T) {
	b := newBundledHarness(t, bundlefix.Options{})
	b.install(t, TTS)
	if err := b.m.Stop(TTS); err != nil {
		t.Fatal(err)
	}
	before := len(b.hr.pipRuns())
	b.m.AutoStart(func(id string) bool { return id == TTS })
	b.waitState(t, TTS, StateRunning, true)
	if len(b.hr.pipRuns()) != before {
		t.Error("boot ran pip although the stamp is current")
	}
}

func TestStartOfAnUnpackedRuntimeThatWentStaleUnpacksFirst(t *testing.T) {
	b := newBundledHarness(t, bundlefix.Options{})
	if err := b.m.Start(TTS); err != nil { // provider switched to managed in Settings, nothing unpacked yet
		t.Fatal(err)
	}
	b.waitState(t, TTS, StateRunning, true)
	if len(b.hr.pipRuns()) != 1 {
		t.Errorf("pip runs = %d", len(b.hr.pipRuns()))
	}
}

func TestAChildThatCannotLoadItsVoicesFailsTheEnableWithoutRestarting(t *testing.T) {
	b := newBundledHarness(t, bundlefix.Options{})
	b.pr.health = []Health{{Status: "loading"}, {Status: "failed", Error: "preload failed: bad voice file at " + b.voices}}
	st := b.failedWith(t)
	if st.ErrorKey != KeyStartFailed {
		t.Errorf("key = %q", st.ErrorKey)
	}
	if !strings.Contains(st.ErrorDetail, "bad voice file") {
		t.Errorf("the failed-voice reason was lost: %q", st.ErrorDetail)
	}
	if strings.Contains(st.ErrorDetail, b.voices) {
		t.Errorf("detail leaks a host path: %q", st.ErrorDetail)
	}
	time.Sleep(30 * time.Millisecond)
	if n := b.r.startCount(); n != 1 {
		t.Errorf("the child was restarted %d times after a deterministic failure", n)
	}
	if b.m.Endpoint(TTS) != "" {
		t.Error("a failed child still resolves")
	}
	if len(b.installs) != 0 {
		t.Error("provider handed over despite the failure")
	}
}

func TestWhileTheVoicesLoadTheCardKeepsShowingTheVoicesStep(t *testing.T) {
	b := newBundledHarness(t, bundlefix.Options{})
	b.pr.health = []Health{{Status: "loading"}}
	if err := b.m.Install(context.Background(), TTS); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if st := b.m.Status(TTS); st.State == StateInstalling && st.Step == StepVoices && b.r.startCount() == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("never showed the voices step: %+v", b.m.Status(TTS))
		}
		time.Sleep(2 * time.Millisecond)
	}
	if b.m.Endpoint(TTS) == "" {
		t.Error("the endpoint is not resolvable while loading")
	}
	b.pr.mu.Lock()
	b.pr.health = []Health{{Status: "ready"}}
	b.pr.mu.Unlock()
	b.waitState(t, TTS, StateRunning, true)
}

func TestForegroundEnsureInstalledUnpacksButDoesNotStart(t *testing.T) {
	b := newBundledHarness(t, bundlefix.Options{})
	py, err := b.m.EnsureInstalled(context.Background(), TTS)
	if err != nil {
		t.Fatal(err)
	}
	if py != filepath.Join(b.root(), "python", "bin", "python3") {
		t.Errorf("python = %q", py)
	}
	if b.r.startCount() != 0 || len(b.installs) != 0 {
		t.Error("foreground enable started the child or switched the provider")
	}
	if !b.m.Installed(TTS) {
		t.Error("not installed after EnsureInstalled")
	}
	// Second call is a no-op on the unpacked runtime.
	if _, err := b.m.EnsureInstalled(context.Background(), TTS); err != nil {
		t.Fatal(err)
	}
	if len(b.hr.pipRuns()) != 1 {
		t.Errorf("pip runs = %d", len(b.hr.pipRuns()))
	}
}

func TestShutdownDuringUnpackLeavesNothing(t *testing.T) {
	gate := make(chan struct{})
	b := newBundledHarness(t, bundlefix.Options{})
	b.hr.block = gate
	if err := b.m.Install(context.Background(), TTS); err != nil {
		t.Fatal(err)
	}
	b.waitState(t, TTS, StateInstalling, false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	b.m.Shutdown(ctx)
	b.noResidue(t)
}

func TestTtsSourcesAreOnlyAppPy(t *testing.T) {
	if got := specs[TTS].files; len(got) != 1 || got[0] != "app.py" {
		t.Errorf("tts files = %v", got)
	}
	if _, err := Source(TTS, "requirements.txt"); err == nil {
		t.Error("a tts requirements.txt is still embedded")
	}
	app, _ := Source(TTS, "app.py")
	if !bytes.Contains(app, []byte("from piper import")) || !bytes.Contains(app, []byte(`os.environ.get("TTS_VOICES_DIR"`)) {
		t.Error("tts/app.py no longer reads TTS_VOICES_DIR through PiperVoice")
	}
	if bytes.Contains(app, []byte("fastapi")) || bytes.Contains(app, []byte("uvicorn")) {
		t.Error("tts/app.py depends on a third-party web framework again")
	}
}

// ---- tar-slip ----

type tarEntry struct {
	h    tar.Header
	body string
}

func makeTar(t *testing.T, entries ...tarEntry) string {
	t.Helper()
	var raw bytes.Buffer
	gz := gzip.NewWriter(&raw)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		e.h.Size = int64(len(e.body))
		if e.h.Typeflag == 0 {
			e.h.Typeflag = tar.TypeReg
		}
		if err := tw.WriteHeader(&e.h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	_ = tw.Close()
	_ = gz.Close()
	p := filepath.Join(t.TempDir(), "x.tar.gz")
	if err := os.WriteFile(p, raw.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestUntarRefusesAnythingThatEscapesTheDestination(t *testing.T) {
	cases := map[string][]tarEntry{
		"absolute path":          {{h: tar.Header{Name: "/etc/evil", Mode: 0o644}, body: "x"}},
		"parent traversal":       {{h: tar.Header{Name: "../evil", Mode: 0o644}, body: "x"}},
		"nested traversal":       {{h: tar.Header{Name: "python/../../evil", Mode: 0o644}, body: "x"}},
		"cancelling traversal":   {{h: tar.Header{Name: "python/x/../y", Mode: 0o644}, body: "x"}},
		"backslash":              {{h: tar.Header{Name: `python\..\evil`, Mode: 0o644}, body: "x"}},
		"absolute symlink":       {{h: tar.Header{Name: "python/l", Typeflag: tar.TypeSymlink, Linkname: "/etc"}}},
		"escaping symlink":       {{h: tar.Header{Name: "python/l", Typeflag: tar.TypeSymlink, Linkname: "../../outside"}}},
		"deeply escaping link":   {{h: tar.Header{Name: "python/a/b/l", Typeflag: tar.TypeSymlink, Linkname: "../../../../x"}}},
		"empty symlink target":   {{h: tar.Header{Name: "python/l", Typeflag: tar.TypeSymlink, Linkname: ""}}},
		"write through symlink":  {{h: tar.Header{Name: "python/d", Typeflag: tar.TypeSymlink, Linkname: "."}}, {h: tar.Header{Name: "python/d/x", Mode: 0o644}, body: "x"}},
		"chain via dot-dot":      {{h: tar.Header{Name: "q", Typeflag: tar.TypeSymlink, Linkname: "."}}, {h: tar.Header{Name: "r/s", Typeflag: tar.TypeSymlink, Linkname: "../q/.."}}},
		"loop then write":        {{h: tar.Header{Name: "a", Typeflag: tar.TypeSymlink, Linkname: "."}}, {h: tar.Header{Name: "a/a/a/l", Typeflag: tar.TypeSymlink, Linkname: "../../.."}}},
		"hard link out":          {{h: tar.Header{Name: "python/h", Typeflag: tar.TypeLink, Linkname: "../../etc/passwd"}}},
		"hard link absolute":     {{h: tar.Header{Name: "python/h", Typeflag: tar.TypeLink, Linkname: "/etc/passwd"}}},
		"hard link to a symlink": {{h: tar.Header{Name: "python/s", Typeflag: tar.TypeSymlink, Linkname: "t"}}, {h: tar.Header{Name: "python/h", Typeflag: tar.TypeLink, Linkname: "python/s"}}},
		"device node":            {{h: tar.Header{Name: "python/dev", Typeflag: tar.TypeChar, Mode: 0o600}}},
		"fifo":                   {{h: tar.Header{Name: "python/f", Typeflag: tar.TypeFifo, Mode: 0o600}}},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			dest := filepath.Join(parent, "dest")
			if err := os.MkdirAll(dest, 0o755); err != nil {
				t.Fatal(err)
			}
			err := untarGz(context.Background(), makeTar(t, entries...), dest)
			if e, ok := asError(err); !ok || e.Key != KeyBundleCorrupt {
				t.Fatalf("err = %v, want %s", err, KeyBundleCorrupt)
			}
			// Nothing appeared next to dest, and no escaped file anywhere above it.
			ents, _ := os.ReadDir(parent)
			if len(ents) != 1 {
				t.Errorf("files appeared outside dest: %v", ents)
			}
			for _, p := range []string{filepath.Join(parent, "evil"), filepath.Join(parent, "outside"), filepath.Join(dest, "..", "x")} {
				if _, err := os.Lstat(p); err == nil {
					t.Errorf("%s was created", p)
				}
			}
		})
	}
}

func TestUntarKeepsModesAndSafeLinksAndEnforcesCaps(t *testing.T) {
	dest := t.TempDir()
	tarPath := makeTar(t,
		tarEntry{h: tar.Header{Name: "python/", Typeflag: tar.TypeDir, Mode: 0o755}},
		tarEntry{h: tar.Header{Name: "python/bin/run", Mode: 0o755}, body: "#!/bin/sh\n"},
		tarEntry{h: tar.Header{Name: "python/bin/data", Mode: 0o640}, body: "d"},
		tarEntry{h: tar.Header{Name: "python/bin/alias", Typeflag: tar.TypeSymlink, Linkname: "run"}},
		tarEntry{h: tar.Header{Name: "python/lib/up", Typeflag: tar.TypeSymlink, Linkname: "../bin/data"}},
		tarEntry{h: tar.Header{Name: "python/bin/hard", Typeflag: tar.TypeLink, Linkname: "python/bin/run"}},
	)
	if err := untarGz(context.Background(), tarPath, dest); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(filepath.Join(dest, "python/bin/run")); fi.Mode().Perm() != 0o755 {
		t.Errorf("run mode = %v", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(filepath.Join(dest, "python/bin/data")); fi.Mode().Perm() != 0o640 {
		t.Errorf("data mode = %v", fi.Mode().Perm())
	}
	if l, _ := os.Readlink(filepath.Join(dest, "python/lib/up")); l != "../bin/data" {
		t.Errorf("link = %q", l)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "python/bin/hard")); string(b) != "#!/bin/sh\n" {
		t.Errorf("hard link = %q", b)
	}

	oldB, oldN := maxUnpackBytes, maxUnpackEntries
	defer func() { maxUnpackBytes, maxUnpackEntries = oldB, oldN }()
	maxUnpackBytes = 10
	if err := untarGz(context.Background(), makeTar(t, tarEntry{h: tar.Header{Name: "a", Mode: 0o644}, body: strings.Repeat("x", 11)}), t.TempDir()); errKey(err) != KeyBundleCorrupt {
		t.Errorf("oversize archive: %v", err)
	}
	maxUnpackBytes = oldB
	maxUnpackEntries = 2
	three := []tarEntry{{h: tar.Header{Name: "a", Mode: 0o644}}, {h: tar.Header{Name: "b", Mode: 0o644}}, {h: tar.Header{Name: "c", Mode: 0o644}}}
	if err := untarGz(context.Background(), makeTar(t, three...), t.TempDir()); errKey(err) != KeyBundleCorrupt {
		t.Errorf("too many entries: %v", err)
	}
	// Not a gzip stream.
	bad := filepath.Join(t.TempDir(), "bad.tar.gz")
	_ = os.WriteFile(bad, []byte("not gzip at all"), 0o644)
	if err := untarGz(context.Background(), bad, t.TempDir()); errKey(err) != KeyBundleCorrupt {
		t.Errorf("garbage archive: %v", err)
	}
}

func TestPlatformDir(t *testing.T) {
	for in, want := range map[[2]string]string{
		{"darwin", "arm64"}: "darwin-arm64", {"linux", "amd64"}: "linux-amd64", {"linux", "arm64"}: "linux-arm64",
		{"darwin", "amd64"}: "", {"windows", "amd64"}: "", {"linux", "riscv64"}: "",
	} {
		if got := platformDir(in[0], in[1]); got != want {
			t.Errorf("%v -> %q, want %q", in, got, want)
		}
	}
}

func TestFailureBetweenTheSwapRenamesKeepsTheOldRuntimeWorking(t *testing.T) {
	b := newBundledHarness(t, bundlefix.Options{})
	b.install(t, TTS)
	if err := b.m.Stop(TTS); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(b.root(), "python", "KEEP")
	if err := os.WriteFile(old, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Make the stamp stale so a re-extract is needed, then fail the second rename.
	_ = os.WriteFile(filepath.Join(b.root(), stampName), []byte("manifest=old\n"), 0o644)
	n := 0
	b.m.opt.Rename = func(from, to string) error {
		if n++; n == 2 {
			return errors.New("injected rename failure")
		}
		return os.Rename(from, to)
	}
	st := b.failedWith(t)
	if st.ErrorKey != KeyReadOnly {
		t.Errorf("key = %q", st.ErrorKey)
	}
	if _, err := os.Stat(old); err != nil {
		t.Error("the previous runtime was not restored")
	}
	if _, err := os.Stat(filepath.Join(b.root(), "python.old")); err == nil {
		t.Error("python.old left behind")
	}
	if _, err := os.Stat(filepath.Join(b.root(), "python", "bin", "python3")); err != nil {
		t.Error("restored runtime has no interpreter")
	}
	// A stale python.old from a crash is cleaned by the next enable, which succeeds.
	_ = os.MkdirAll(filepath.Join(b.root(), "python.old", "junk"), 0o755)
	b.m.opt.Rename = nil
	b.install(t, TTS)
	if _, err := os.Stat(filepath.Join(b.root(), "python.old")); err == nil {
		t.Error("stale python.old survived")
	}
}
