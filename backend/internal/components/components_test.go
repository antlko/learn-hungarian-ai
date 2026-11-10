package components

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

// ---- fakes: no network, no real Python, no model ----

type fakeProc struct {
	once sync.Once
	done chan struct{}
	term atomic.Int32
	code atomic.Int32 // what ExitCode reports (component run)
}

func (p *fakeProc) ExitCode() int { return int(p.code.Load()) }

func newProc() *fakeProc  { return &fakeProc{done: make(chan struct{})} }
func (p *fakeProc) exit() { p.once.Do(func() { close(p.done) }) }
func (p *fakeProc) Terminate() {
	p.term.Add(1)
	p.exit()
}
func (p *fakeProc) Pid() int              { return 4242 }
func (p *fakeProc) Kill()                 { p.exit() }
func (p *fakeProc) Done() <-chan struct{} { return p.done }

type fakeRunner struct {
	mu       sync.Mutex
	runs     []Cmd
	starts   []Cmd
	procs    []*fakeProc
	pyOK     bool
	failRun  func(c Cmd) (lines []string, err error) // nil = succeed
	block    chan struct{}                           // when set, the venv step waits on it
	onStart  func(n int, onLine func(string)) (exitNow bool)
	startErr error
}

func (r *fakeRunner) Run(ctx context.Context, c Cmd, onLine func(string)) error {
	r.mu.Lock()
	r.runs = append(r.runs, c)
	r.mu.Unlock()
	joined := strings.Join(c.Args, " ")
	switch {
	case strings.Contains(joined, "version_info"):
		if !r.pyOK {
			return errors.New("exit status 2")
		}
		onLine("3.12.1")
		return nil
	case strings.Contains(joined, "-m venv"):
		if r.block != nil {
			select {
			case <-r.block:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		target := c.Args[len(c.Args)-1]
		if err := os.MkdirAll(filepath.Join(target, "bin"), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(target, "bin", "python"), []byte("#!fake"), 0o755)
	case strings.Contains(joined, "pip install"):
		for _, l := range []string{"Collecting fastapi==1", "Collecting uvicorn==1", "Downloading x", "Collecting spacy", "Installing collected packages: a, b"} {
			onLine(l)
		}
	}
	if r.failRun != nil {
		if lines, err := r.failRun(c); err != nil {
			for _, l := range lines {
				onLine(l)
			}
			return err
		}
	}
	return nil
}

func (r *fakeRunner) Start(c Cmd, onLine func(string)) (Proc, error) {
	r.mu.Lock()
	r.starts = append(r.starts, c)
	n := len(r.starts)
	r.mu.Unlock()
	if r.startErr != nil {
		return nil, r.startErr
	}
	p := newProc()
	r.mu.Lock()
	r.procs = append(r.procs, p)
	r.mu.Unlock()
	if r.onStart != nil && r.onStart(n, onLine) {
		p.exit()
	}
	return p, nil
}

func (r *fakeRunner) startCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.starts)
}

func (r *fakeRunner) lastProc() *fakeProc {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.procs[len(r.procs)-1]
}

type fakeProber struct {
	mu     sync.Mutex
	silent bool
	health []Health // consumed in order; the last repeats
	warm   atomic.Int32
}

func (p *fakeProber) Alive(context.Context, int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.silent
}
func (p *fakeProber) Warmup(context.Context, int) error {
	p.warm.Add(1)
	return nil
}
func (p *fakeProber) Health(context.Context, int) (Health, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.health) == 0 {
		return Health{Status: "ready"}, nil
	}
	h := p.health[0]
	if len(p.health) > 1 {
		p.health = p.health[1:]
	}
	return h, nil
}

type fakeDisk struct{ free uint64 }

func (d fakeDisk) Free(string) (uint64, error) { return d.free, nil }

type harness struct {
	m        *Manager
	r        *fakeRunner
	pr       *fakeProber
	dir      string
	mu       sync.Mutex
	changes  []Status
	installs []string
}

func newHarness(t *testing.T, mod ...func(*Options)) *harness {
	t.Helper()
	h := &harness{r: &fakeRunner{pyOK: true}, pr: &fakeProber{}, dir: t.TempDir()}
	var port atomic.Int32
	port.Store(40000)
	o := Options{
		DataDir: h.dir, Runner: h.r, Prober: h.pr,
		LookPath:     func(n string) (string, error) { return "/usr/bin/" + n, nil },
		Disk:         fakeDisk{free: 100 * gb},
		PickPort:     func() (int, error) { return int(port.Add(1)), nil },
		PollInterval: 2 * time.Millisecond, StopGrace: 50 * time.Millisecond,
		Backoff:     func(int) time.Duration { return time.Millisecond },
		OnInstalled: func(id string) error { h.mu.Lock(); h.installs = append(h.installs, id); h.mu.Unlock(); return nil },
	}
	for _, f := range mod {
		f(&o)
	}
	h.m = New(o)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		h.m.Shutdown(ctx)
	})
	return h
}

func (h *harness) waitState(t *testing.T, id string, want State, busyOff bool) Status {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		st := h.m.Status(id)
		h.m.mu.Lock()
		busy := h.m.comps[id].busy
		h.m.mu.Unlock()
		if st.State == want && (!busyOff || !busy) {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: state %q (err %q %q), want %q", id, st.State, st.ErrorKey, st.ErrorDetail, want)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func (h *harness) install(t *testing.T, id string) Status {
	t.Helper()
	if err := h.m.Install(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	return h.waitState(t, id, StateRunning, true)
}

// ---- tests ----

func TestPythonMissingMakesComponentUnavailable(t *testing.T) {
	h := newHarness(t, func(o *Options) {
		o.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	})
	st := h.m.Status(MT)
	if st.Available || st.UnavailableKey != KeyNoPython || st.State != StateAbsent {
		t.Fatalf("%+v", st)
	}
	err := h.m.Install(context.Background(), MT)
	if k := errKey(err); k != KeyNoPython {
		t.Fatalf("install = %v", err)
	}
	if st.DiskNeed == "" || st.RAMNeed == "" {
		t.Fatalf("estimates missing: %+v", st)
	}
}

func TestPythonTooOldOrWithoutVenvIsUnavailable(t *testing.T) {
	h := newHarness(t)
	h.r.pyOK = false
	if st := h.m.Status(NLP); st.Available || st.UnavailableKey != KeyNoPython {
		t.Fatalf("%+v", st)
	}
}

func TestPythonDetectedReportsVersion(t *testing.T) {
	h := newHarness(t)
	st := h.m.Status(MT)
	if !st.Available || st.PythonVersion != "3.12.1" || st.UnavailableKey != "" {
		t.Fatalf("%+v", st)
	}
}

func TestReadOnlyDataDirIsUnavailable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory modes")
	}
	h := newHarness(t)
	if err := os.Chmod(h.dir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(h.dir, 0o755)
	st := h.m.Status(MT)
	if st.Available || st.UnavailableKey != KeyReadOnly {
		t.Fatalf("%+v", st)
	}
}

func TestDiskPreflightRefusesBeforeTouchingAnything(t *testing.T) {
	h := newHarness(t, func(o *Options) { o.Disk = fakeDisk{free: gb} })
	err := h.m.Install(context.Background(), MT)
	if errKey(err) != KeyDiskFull {
		t.Fatalf("install = %v", err)
	}
	if _, e := os.Stat(h.m.stage(MT)); e == nil {
		t.Fatal("staging dir created despite the refusal")
	}
	if len(h.r.runs) > 1 { // only the python detection
		t.Fatalf("commands ran: %v", h.r.runs[1:])
	}
}

func TestHappyPathMTStateMachineAndCommands(t *testing.T) {
	var seen []Status
	var smu sync.Mutex
	var hh *harness
	hh = newHarness(t, func(o *Options) {
		o.GOOS = "linux"
		o.OnChange = func(id string) {
			if hh == nil {
				return
			}
			st := hh.m.Status(id)
			smu.Lock()
			seen = append(seen, st)
			smu.Unlock()
		}
	})
	if st := hh.m.Status(MT); st.State != StateAbsent {
		t.Fatalf("initial %+v", st)
	}
	st := hh.install(t, MT)
	if st.Port == 0 || st.Progress != 1 || st.Step != "" || st.ErrorKey != "" {
		t.Fatalf("final %+v", st)
	}
	if ep := hh.m.Endpoint(MT); ep != "http://127.0.0.1:"+strconv.Itoa(st.Port) {
		t.Fatalf("endpoint %q", ep)
	}
	if hh.m.Endpoint(NLP) != "" {
		t.Fatal("nlp has an endpoint without being installed")
	}
	if len(hh.installs) != 1 || hh.installs[0] != MT {
		t.Fatalf("OnInstalled = %v", hh.installs)
	}

	// States seen on the way: installing first, running last; progress never goes back.
	smu.Lock()
	defer smu.Unlock()
	sawInstalling, prev := false, -1.0
	for _, s := range seen {
		if s.State == StateInstalling {
			sawInstalling = true
		}
		if s.Progress < prev {
			t.Fatalf("progress went back: %v -> %v", prev, s.Progress)
		}
		prev = s.Progress
	}
	if !sawInstalling {
		t.Fatal("never reported installing")
	}
	steps := map[string]bool{}
	for _, s := range seen {
		steps[s.Step] = true
	}
	for _, k := range []string{StepVenv, StepPip, StepModel, StepStart} {
		if !steps[k] {
			t.Errorf("step %s never reported (%v)", k, steps)
		}
	}

	// Disk: final dir committed, staging gone, sources equal to the embedded ones.
	if _, err := os.Stat(marker(hh.m.final(MT))); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(hh.m.stage(MT)); err == nil {
		t.Fatal(".installing left behind")
	}
	want, _ := Source(MT, "app.py")
	got, _ := os.ReadFile(filepath.Join(hh.m.final(MT), "src", "app.py"))
	if !bytes.Equal(want, got) {
		t.Fatal("extracted app.py differs from the embedded copy")
	}

	// The pipeline, in order, including the torch dance.
	var joined []string
	for _, c := range hh.r.runs[1:] {
		joined = append(joined, strings.Join(c.Args, " "))
	}
	all := strings.Join(joined, "\n")
	for _, frag := range []string{"-m venv", "--no-deps --index-url https://download.pytorch.org/whl/cpu torch==2.5.1", "-r requirements.txt", "app.prepare_model()", "pip uninstall -y"} {
		if !strings.Contains(all, frag) {
			t.Errorf("missing command containing %q in:\n%s", frag, all)
		}
	}
	if !strings.Contains(all, "--no-input --disable-pip-version-check") {
		t.Error("pip flags missing")
	}
}

func TestHappyPathNLPFollowsTheServiceModelDownload(t *testing.T) {
	h := newHarness(t)
	h.pr.health = []Health{{Status: "error", Error: "model not downloaded: POST /warmup"}, {Status: "downloading", Progress: 0.5}, {Status: "ready"}}
	st := h.install(t, NLP)
	if h.pr.warm.Load() != 1 {
		t.Fatalf("warmup calls = %d", h.pr.warm.Load())
	}
	if st.Progress != 1 {
		t.Fatalf("%+v", st)
	}
	// nlp has no torch step.
	for _, c := range h.r.runs {
		if strings.Contains(strings.Join(c.Args, " "), "torch") {
			t.Fatalf("nlp installed torch: %v", c.Args)
		}
	}
}

func TestNLPModelDownloadFailureLeavesNothing(t *testing.T) {
	h := newHarness(t)
	h.pr.health = []Health{{Status: "error", Error: "download failed: Temporary failure in name resolution"}}
	if err := h.m.Install(context.Background(), NLP); err != nil {
		t.Fatal(err)
	}
	st := h.waitState(t, NLP, StateFailed, true)
	if st.ErrorKey != KeyNetwork {
		t.Fatalf("%+v", st)
	}
	if _, err := os.Stat(h.m.final(NLP)); err == nil {
		t.Fatal("a half-installed component was left behind")
	}
	if h.m.Endpoint(NLP) != "" || len(h.installs) != 0 {
		t.Fatal("handed over a failed install")
	}
}

func TestFailedPipLeavesNothingBehind(t *testing.T) {
	h := newHarness(t)
	h.r.failRun = func(c Cmd) ([]string, error) {
		if strings.Contains(strings.Join(c.Args, " "), "-r requirements.txt") {
			return []string{"WARNING: Retrying", "ERROR: Temporary failure in name resolution for /Users/someone/secret/path"}, errors.New("exit status 1")
		}
		return nil, nil
	}
	if err := h.m.Install(context.Background(), MT); err != nil {
		t.Fatal(err)
	}
	st := h.waitState(t, MT, StateFailed, true)
	if st.ErrorKey != KeyNetwork || st.ErrorDetail == "" {
		t.Fatalf("%+v", st)
	}
	if strings.Contains(st.ErrorDetail, "/Users/someone") {
		t.Fatalf("detail leaks a host path: %q", st.ErrorDetail)
	}
	if _, err := os.Stat(h.m.stage(MT)); err == nil {
		t.Fatal(".installing left behind")
	}
	// Only pip's download cache may remain (so a retry does not re-fetch torch).
	if _, err := os.Stat(marker(h.m.final(MT))); err == nil {
		t.Fatal("marker written for a failed install")
	}
	if ents, _ := os.ReadDir(h.m.final(MT)); len(ents) > 0 && (len(ents) != 1 || ents[0].Name() != "cache") {
		t.Fatalf("leftovers in final dir: %v", ents)
	}
	if got := envOf(h.r.runs[len(h.r.runs)-1], "PIP_CACHE_DIR"); got != filepath.Join(h.m.final(MT), "cache", "pip") {
		t.Fatalf("PIP_CACHE_DIR = %q", got)
	}
	if h.m.Endpoint(MT) != "" || len(h.installs) != 0 {
		t.Fatal("provider handed over after a failure")
	}
	// And it can simply be tried again — and a success frees the pip cache.
	h.r.failRun = nil
	h.install(t, MT)
	if _, err := os.Stat(filepath.Join(h.m.final(MT), "cache")); err == nil {
		t.Fatal("pip cache kept after a successful install")
	}
}

func TestOtherPipFailureIsInstallFailed(t *testing.T) {
	h := newHarness(t)
	h.r.failRun = func(c Cmd) ([]string, error) {
		if strings.Contains(strings.Join(c.Args, " "), "-r requirements.txt") {
			return []string{"ERROR: Failed building wheel for x"}, errors.New("exit status 1")
		}
		return nil, nil
	}
	_ = h.m.Install(context.Background(), NLP)
	if st := h.waitState(t, NLP, StateFailed, true); st.ErrorKey != KeyInstallFailed {
		t.Fatalf("%+v", st)
	}
}

func TestInstallIsSingleFlight(t *testing.T) {
	h := newHarness(t)
	h.r.block = make(chan struct{})
	if err := h.m.Install(context.Background(), MT); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Install(context.Background(), MT); !errors.Is(err, ErrBusy) {
		t.Fatalf("second install = %v", err)
	}
	if err := h.m.Uninstall(MT); !errors.Is(err, ErrBusy) {
		t.Fatalf("uninstall during install = %v", err)
	}
	if err := h.m.Stop(MT); !errors.Is(err, ErrBusy) {
		t.Fatalf("stop during install = %v", err)
	}
	// Manager-wide: a second component may not install at the same time either.
	if err := h.m.Install(context.Background(), NLP); !errors.Is(err, ErrBusy) {
		t.Fatalf("concurrent install of another component = %v", err)
	}
	close(h.r.block)
	h.waitState(t, MT, StateRunning, true)
	// ...and the slot is free again afterwards.
	h.install(t, NLP)
}

func TestInstallTwiceIsRefusedWhenInstalled(t *testing.T) {
	h := newHarness(t)
	h.install(t, NLP)
	if err := h.m.Install(context.Background(), NLP); !errors.Is(err, ErrBusy) && !errors.Is(err, ErrInstalled) {
		t.Fatalf("reinstall = %v", err)
	}
}

func TestUnknownComponent(t *testing.T) {
	h := newHarness(t)
	for _, err := range []error{
		h.m.Install(context.Background(), "tts"), h.m.Start("llm"), h.m.Stop("x"), h.m.Uninstall("../etc"),
	} {
		if !errors.Is(err, ErrUnknown) {
			t.Errorf("err = %v", err)
		}
	}
}

func TestStopAndStart(t *testing.T) {
	h := newHarness(t)
	st := h.install(t, MT)
	proc := h.r.lastProc()
	if err := h.m.Stop(MT); err != nil {
		t.Fatal(err)
	}
	if proc.term.Load() == 0 {
		t.Fatal("child was not asked to terminate")
	}
	if st := h.m.Status(MT); st.State != StateStopped || st.Port != 0 || h.m.Endpoint(MT) != "" {
		t.Fatalf("after stop %+v", st)
	}
	if err := h.m.Start(MT); err != nil {
		t.Fatal(err)
	}
	st2 := h.waitState(t, MT, StateRunning, false)
	if st2.Port == 0 || st2.Port == st.Port && h.r.startCount() < 2 {
		t.Fatalf("restart %+v", st2)
	}
	// Idempotent.
	if err := h.m.Start(MT); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Stop(MT); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Stop(MT); err != nil {
		t.Fatalf("stopping a stopped component: %v", err)
	}
}

func TestStartRequiresInstall(t *testing.T) {
	h := newHarness(t)
	if err := h.m.Start(MT); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("%v", err)
	}
}

func TestCrashRestartsWithBackoffThenFails(t *testing.T) {
	h := newHarness(t, func(o *Options) { o.MaxRestarts = 3 })
	h.install(t, MT)
	// From now on every start dies at once.
	h.r.mu.Lock()
	base := len(h.r.starts)
	h.r.onStart = func(n int, onLine func(string)) bool {
		onLine("Traceback: boom")
		return true
	}
	h.r.mu.Unlock()
	h.r.lastProc().exit() // the running one crashes
	st := h.waitState(t, MT, StateFailed, false)
	if st.ErrorKey != KeyCrashed || !strings.Contains(st.ErrorDetail, "boom") {
		t.Fatalf("%+v", st)
	}
	// 1 original crash + restarts until the cap (3) is exceeded: 4 crashes, 4 starts after base... bounded.
	if n := h.r.startCount() - base; n != 3 {
		t.Fatalf("restarts = %d, want 3", n)
	}
	if h.m.Endpoint(MT) != "" {
		t.Fatal("endpoint of a failed component")
	}
	// A failed component can be started again by hand.
	h.r.mu.Lock()
	h.r.onStart = nil
	h.r.mu.Unlock()
	if err := h.m.Start(MT); err != nil {
		t.Fatal(err)
	}
	h.waitState(t, MT, StateRunning, false)
}

func TestAddressInUseRepicksThePortWithoutCountingACrash(t *testing.T) {
	h := newHarness(t, func(o *Options) { o.MaxRestarts = 1 })
	h.install(t, NLP)
	_ = h.m.Stop(NLP)
	h.r.mu.Lock()
	base := len(h.r.starts)
	h.r.onStart = func(n int, onLine func(string)) bool {
		if n-base <= 2 {
			onLine("OSError: [Errno 98] error while attempting to bind: address already in use")
			return true
		}
		return false
	}
	h.r.mu.Unlock()
	if err := h.m.Start(NLP); err != nil {
		t.Fatal(err)
	}
	h.waitState(t, NLP, StateRunning, false)
	h.r.mu.Lock()
	defer h.r.mu.Unlock()
	if len(h.r.starts)-base != 3 {
		t.Fatalf("starts = %d", len(h.r.starts)-base)
	}
	if h.r.starts[base].Env == nil || h.r.starts[base+1].Env == nil {
		t.Fatal("no env")
	}
	if envOf(h.r.starts[base], "NLP_PORT") == envOf(h.r.starts[base+1], "NLP_PORT") {
		t.Fatal("same port re-used after EADDRINUSE")
	}
}

func TestShutdownStopsChildren(t *testing.T) {
	h := newHarness(t)
	h.install(t, MT)
	h.install(t, NLP)
	p1, p2 := h.r.procs[0], h.r.procs[1]
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h.m.Shutdown(ctx)
	for i, p := range []*fakeProc{p1, p2} {
		select {
		case <-p.done:
		default:
			t.Fatalf("child %d still running after Shutdown", i)
		}
	}
	if h.m.Endpoint(MT) != "" {
		t.Fatal("endpoint after shutdown")
	}
}

func TestShutdownCancelsAnInstallAndLeavesNothing(t *testing.T) {
	h := newHarness(t)
	h.r.block = make(chan struct{})
	if err := h.m.Install(context.Background(), MT); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	h.m.Shutdown(context.Background())
	if _, err := os.Stat(h.m.stage(MT)); err == nil {
		t.Fatal(".installing left behind after cancel")
	}
}

func TestChildEnvironmentIsAnAllowlist(t *testing.T) {
	t.Setenv("TANULOM_SECRET_KEY", "hunter2")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "x")
	t.Setenv("HTTPS_PROXY", "http://proxy.invalid:3128")
	h := newHarness(t)
	h.install(t, MT)

	check := func(what string, c Cmd) {
		for _, kv := range c.Env {
			k, _, _ := strings.Cut(kv, "=")
			if strings.HasPrefix(k, "TANULOM_") || k == "OPENAI_API_KEY" || k == "AWS_SECRET_ACCESS_KEY" || strings.Contains(kv, "hunter2") || strings.Contains(kv, "sk-test") {
				t.Errorf("%s inherits %q", what, k)
			}
		}
	}
	for _, c := range h.r.runs {
		check("install command", c)
	}
	for _, c := range h.r.starts {
		check("service", c)
		if envOf(c, "HTTPS_PROXY") != "" {
			t.Error("proxy settings leaked into the running service")
		}
	}
	c := h.r.starts[0]
	root := h.m.final(MT)
	if envOf(c, "MT_HOST") != "127.0.0.1" {
		t.Errorf("MT_HOST = %q", envOf(c, "MT_HOST"))
	}
	if p, err := strconv.Atoi(envOf(c, "MT_PORT")); err != nil || p == 0 {
		t.Errorf("MT_PORT = %q", envOf(c, "MT_PORT"))
	}
	if envOf(c, "MT_MODEL_DIR") != filepath.Join(root, "models", "nllb-ct2") {
		t.Errorf("MT_MODEL_DIR = %q", envOf(c, "MT_MODEL_DIR"))
	}
	if envOf(c, "HOME") != filepath.Join(root, "home") {
		t.Errorf("HOME = %q", envOf(c, "HOME"))
	}
	if c.Path != filepath.Join(root, "venv", "bin", "python") || c.Args[0] != filepath.Join(root, "src", "app.py") {
		t.Errorf("command %v %v", c.Path, c.Args)
	}
	// The proxy IS available to pip, which needs it behind one.
	found := false
	for _, r := range h.r.runs {
		if envOf(r, "HTTPS_PROXY") != "" {
			found = true
		}
	}
	if !found {
		t.Error("pip did not get the proxy setting")
	}
}

func TestUninstallRemovesEverything(t *testing.T) {
	h := newHarness(t)
	h.install(t, NLP)
	proc := h.r.lastProc()
	if err := h.m.Uninstall(NLP); err != nil {
		t.Fatal(err)
	}
	select {
	case <-proc.done:
	default:
		t.Fatal("child still running after uninstall")
	}
	if _, err := os.Stat(h.m.final(NLP)); err == nil {
		t.Fatal("component dir remains")
	}
	if st := h.m.Status(NLP); st.State != StateAbsent {
		t.Fatalf("%+v", st)
	}
	// Reinstallable afterwards.
	h.install(t, NLP)
}

func TestAutoStartOnlyWhatIsWantedAndInstalled(t *testing.T) {
	h := newHarness(t)
	for _, id := range IDs() {
		if err := os.MkdirAll(h.m.final(id), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(marker(h.m.final(id)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if st := h.m.Status(MT); st.State != StateInstalled {
		t.Fatalf("installed-on-disk state = %q", st.State)
	}
	h.m.AutoStart(func(id string) bool { return id == NLP })
	h.waitState(t, NLP, StateRunning, false)
	if st := h.m.Status(MT); st.State != StateInstalled {
		t.Fatalf("mt must not start: %q", st.State)
	}
	// sources were refreshed from the embedded copy.
	if _, err := os.Stat(filepath.Join(h.m.final(NLP), "src", "app.py")); err != nil {
		t.Fatal(err)
	}
}

func TestRunSourcesAreReverifiedOnEveryStart(t *testing.T) {
	h := newHarness(t)
	h.install(t, NLP)
	_ = h.m.Stop(NLP)
	app := filepath.Join(h.m.final(NLP), "src", "app.py")
	if err := os.WriteFile(app, []byte("import os; os.system('evil')"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Start(NLP); err != nil {
		t.Fatal(err)
	}
	h.waitState(t, NLP, StateRunning, false)
	want, _ := Source(NLP, "app.py")
	got, _ := os.ReadFile(app)
	if !bytes.Equal(want, got) {
		t.Fatal("tampered app.py was run instead of being restored")
	}
}

func TestEndpointIsNilSafe(t *testing.T) {
	var m *Manager
	if m.Endpoint(MT) != "" {
		t.Fatal("nil manager resolved an endpoint")
	}
	m.Shutdown(context.Background())
}

func TestLogsAreRedactedAndBounded(t *testing.T) {
	h := newHarness(t)
	h.r.failRun = func(c Cmd) ([]string, error) { return nil, nil }
	h.install(t, NLP)
	h.m.addLog(h.m.comps[NLP], "reading /home/alice/.cache/foo and "+h.dir+"/components/nlp/venv/lib x https://pypi.org/simple/x")
	for i := 0; i < 500; i++ {
		h.m.addLog(h.m.comps[NLP], "line "+strconv.Itoa(i))
	}
	st := h.m.Status(NLP)
	if len(st.Log) > 40 {
		t.Fatalf("log not bounded: %d", len(st.Log))
	}
	h.m.addLog(h.m.comps[NLP], "reading /home/alice/.cache/foo and "+h.dir+"/components/nlp/venv/lib x https://pypi.org/simple/x")
	st = h.m.Status(NLP)
	last := st.Log[len(st.Log)-1]
	if strings.Contains(last, "alice") || strings.Contains(last, "/home/") || !strings.Contains(last, "<data>/components/nlp/venv/lib") || !strings.Contains(last, "https://pypi.org/simple/x") {
		t.Fatalf("redaction wrong: %q", last)
	}
}

func TestClassify(t *testing.T) {
	cases := map[string]string{
		"ERROR: No matching distribution found for x":       KeyInstallFailed,
		"OSError: [Errno 28] No space left on device":       KeyDiskFull,
		"urllib3 Max retries exceeded with url":             KeyNetwork,
		"SyntaxError: invalid syntax":                       KeyInstallFailed,
		"Temporary failure in name resolution":              KeyNetwork,
		"Failed building wheel for something with a C ext.": KeyInstallFailed,
	}
	for in, want := range cases {
		if got := classify([]string{in}); got != want {
			t.Errorf("%q -> %s, want %s", in, got, want)
		}
	}
}

// The embedded copies are the ones the binary runs; services/<id>/ is what the
// Dockerfiles build. They must not drift: `make sync-components` fixes a failure.
func TestEmbeddedSourcesMatchServices(t *testing.T) {
	for id, sp := range specs {
		for _, name := range sp.files {
			emb, err := Source(id, name)
			if err != nil {
				t.Fatal(err)
			}
			real, err := os.ReadFile(filepath.Join("..", "..", "..", "services", id, name))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(emb, real) {
				t.Errorf("services/%s/%s differs from the embedded copy — run `make sync-components`", id, name)
			}
		}
	}
}

func TestEmbeddedAppsBindToTheConfiguredHost(t *testing.T) {
	for id, host := range map[string]string{MT: "MT_HOST", NLP: "NLP_HOST"} {
		b, _ := Source(id, "app.py")
		if !bytes.Contains(b, []byte(`os.environ.get("`+host+`", "0.0.0.0")`)) {
			t.Errorf("%s/app.py does not read %s", id, host)
		}
	}
}

// ---- the real runner, against /bin/sh ----

func TestExecRunnerIsolatesEnvironmentAndStopsTheGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix only")
	}
	t.Setenv("TANULOM_LEAK", "yes")
	var out []string
	var mu sync.Mutex
	err := ExecRunner{}.Run(context.Background(),
		Cmd{Path: "/bin/sh", Args: []string{"-c", "env; printf 'a\\rb\\n'"}, Env: []string{"ONLY=1", "PATH=/usr/bin:/bin"}},
		func(l string) { mu.Lock(); out = append(out, l); mu.Unlock() })
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(out, "\n")
	if strings.Contains(joined, "TANULOM_LEAK") || !strings.Contains(joined, "ONLY=1") {
		t.Fatalf("env not isolated:\n%s", joined)
	}
	if !strings.Contains(joined, "\na\nb") {
		t.Fatalf("\\r not split into lines:\n%s", joined)
	}

	// A child that spawns a grandchild: stopping the group stops both.
	pidFile := filepath.Join(t.TempDir(), "pid")
	p, err := ExecRunner{}.Start(Cmd{Path: "/bin/sh", Args: []string{"-c", "sleep 60 & echo $! > " + pidFile + "; wait"},
		Env: []string{"PATH=/usr/bin:/bin"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var pid int
	for i := 0; i < 200 && pid == 0; i++ {
		if b, err := os.ReadFile(pidFile); err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("grandchild never started")
	}
	p.Terminate()
	select {
	case <-p.Done():
	case <-time.After(5 * time.Second):
		p.Kill()
		t.Fatal("process did not stop on SIGTERM")
	}
	time.Sleep(100 * time.Millisecond)
	if processAlive(pid) {
		t.Fatalf("grandchild %d survived the group signal", pid)
	}
}

func errKey(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Key
	}
	return ""
}

func envOf(c Cmd, k string) string {
	for _, kv := range c.Env {
		if v, ok := strings.CutPrefix(kv, k+"="); ok {
			return v
		}
	}
	return ""
}

// Fixture: the owner's first real install (macOS, python 3.11), trimmed.
var evidenceLog = []string{
	"Error processing line 1 of /Library/Frameworks/Python.framework/Versions/3.11/lib/python3.11/site-packages/json_extensions-0.2.1-py3.11-nspkg.pth:",
	"",
	"  Traceback (most recent call last):",
	"    File \"<frozen site>\", line 186, in addpackage",
	"  AttributeError: 'NoneType' object has no attribute 'loader'",
	"",
	"Remainder of file ignored",
	"Looking in indexes: https://download.pytorch.org/whl/cpu",
	"Collecting torch==2.5.1",
	"Collecting typing-extensions>=4.8.0 (from torch==2.5.1)",
	"  Downloading typing_extensions-4.16.0.tar.gz (61 kB)",
	"  Installing build dependencies: started",
	"ERROR: Could not find a version that satisfies the requirement flit_core<4,>=3.11 (from versions: none)",
	"ERROR: No matching distribution found for flit_core<4,>=3.11",
}

func TestResolutionFailureIsNotANetworkError(t *testing.T) {
	if got := classify(evidenceLog); got != KeyInstallFailed {
		t.Fatalf("classify = %s", got)
	}
	var f noiseFilter
	var kept []string
	for _, l := range evidenceLog {
		if !f.skip(l) {
			kept = append(kept, l)
		}
	}
	d := relevantTail(kept, 8)
	if !strings.Contains(d, "flit_core") || strings.Contains(d, "json_extensions") {
		t.Fatalf("detail: %q", d)
	}
	if len(strings.Split(d, "\n")) > 8 {
		t.Fatal("detail too long")
	}
}

func TestNetworkAndDiskClasses(t *testing.T) {
	for _, l := range []string{
		"WARNING: Retrying (Retry(total=4)) after connection broken by 'NewConnectionError: Failed to establish a new connection: [Errno 8] nodename nor servname provided'",
		"urllib3.exceptions.MaxRetryError: HTTPSConnectionPool: Max retries exceeded with url: /simple/x/",
		"pip._vendor.urllib3.exceptions.SSLError: certificate verify failed",
		"ReadTimeoutError: HTTPSConnectionPool(host='files.pythonhosted.org'): Read timed out.",
		"Temporary failure in name resolution", "OSError: [Errno 101] Network is unreachable",
	} {
		if got := classify([]string{"Collecting x", l}); got != KeyNetwork {
			t.Errorf("%q -> %s", l, got)
		}
	}
	if got := classify([]string{"OSError: [Errno 28] No space left on device"}); got != KeyDiskFull {
		t.Fatal(got)
	}
}

func TestSystemPythonSiteNoiseIsDropped(t *testing.T) {
	h := newHarness(t)
	c := h.m.comps[MT]
	for _, l := range evidenceLog {
		h.m.addLog(c, l)
	}
	got := strings.Join(h.m.Status(MT).Log, "\n")
	for _, bad := range []string{"json_extensions", "Traceback", "Remainder of file", "frozen site", "NoneType"} {
		if strings.Contains(got, bad) {
			t.Errorf("noise %q kept:\n%s", bad, got)
		}
	}
	if !strings.Contains(got, "Collecting torch==2.5.1") || !strings.Contains(got, "flit_core") {
		t.Fatalf("real lines lost:\n%s", got)
	}
}

func TestTorchNeverResolvesDependenciesFromThePyTorchIndex(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "freebsd"} {
		steps := torchSteps(goos)
		for _, st := range steps {
			joined := strings.Join(st, " ")
			if strings.Contains(joined, "pytorch.org") && !strings.Contains(joined, "--no-deps") {
				t.Errorf("%s: pytorch index without --no-deps: %s", goos, joined)
			}
		}
		if goos != "linux" && (len(steps) != 1 || strings.Contains(strings.Join(steps[0], " "), "index")) {
			t.Errorf("%s: %v", goos, steps)
		}
		if goos == "linux" && (len(steps) != 2 || !strings.Contains(strings.Join(steps[1], " "), "sympy==1.13.1")) {
			t.Errorf("linux: %v", steps)
		}
	}
	// And the runner really receives them, with binary-only wheels.
	for _, goos := range []string{"darwin", "linux"} {
		h := newHarness(t, func(o *Options) { o.GOOS = goos })
		h.install(t, MT)
		n := 0
		for _, c := range h.r.runs {
			j := strings.Join(c.Args, " ")
			if strings.Contains(j, "pip install") {
				n++
				if !strings.Contains(j, "--only-binary=:all:") {
					t.Errorf("%s: pip without --only-binary: %s", goos, j)
				}
			}
		}
		want := map[string]int{"darwin": 2, "linux": 3}[goos]
		if n != want {
			t.Errorf("%s: %d pip installs, want %d", goos, n, want)
		}
	}
}

func TestBrokenPipFailsFastWithItsOwnKey(t *testing.T) {
	h := newHarness(t)
	h.r.failRun = func(c Cmd) ([]string, error) {
		if strings.Contains(strings.Join(c.Args, " "), "pip --version") {
			return []string{"No module named pip"}, errors.New("exit status 1")
		}
		return nil, nil
	}
	_ = h.m.Install(context.Background(), MT)
	st := h.waitState(t, MT, StateFailed, true)
	if st.ErrorKey != KeyPipBroken {
		t.Fatalf("%+v", st)
	}
	for _, c := range h.r.runs {
		if strings.Contains(strings.Join(c.Args, " "), "pip install") {
			t.Fatal("heavy install attempted with a broken pip")
		}
	}
}

func TestVenvIsCreatedIsolated(t *testing.T) {
	h := newHarness(t)
	h.install(t, NLP)
	for _, c := range h.r.runs {
		if j := strings.Join(c.Args, " "); strings.Contains(j, "-m venv") && !strings.HasPrefix(j, "-I ") {
			t.Fatalf("venv not created with -I: %s", j)
		}
	}
}

func TestUninstallRefusedWhileAStartHoldsTheClaim(t *testing.T) {
	h := newHarness(t)
	h.install(t, NLP)
	_ = h.m.Stop(NLP)
	h.m.mu.Lock()
	h.m.comps[NLP].claim = true
	h.m.mu.Unlock()
	if err := h.m.Uninstall(NLP); !errors.Is(err, ErrBusy) {
		t.Fatalf("uninstall during a start = %v", err)
	}
	if err := h.m.Start(NLP); !errors.Is(err, ErrBusy) {
		t.Fatalf("second start = %v", err)
	}
	h.m.mu.Lock()
	h.m.comps[NLP].claim = false
	h.m.mu.Unlock()
	if err := h.m.Start(NLP); err != nil {
		t.Fatal(err)
	}
}

func TestStaleChildIsReapedAtManagerStart(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "components", MT)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	pidf := filepath.Join(root, "run.pid")
	var mu sync.Mutex
	alive := map[int]bool{4001: true, 4002: true}
	var signalled []int
	cmds := map[int]string{
		4001: filepath.Join(root, "venv", "bin", "python") + " " + filepath.Join(root, "src", "app.py"),
		4002: "/usr/bin/vim notes.txt", // a recycled pid
	}
	opts := func() Options {
		return Options{DataDir: dir, StopGrace: 50 * time.Millisecond,
			ProcInfo: func(pid int) (string, bool) { mu.Lock(); defer mu.Unlock(); return cmds[pid], alive[pid] },
			SignalPID: func(pid int, force bool) {
				mu.Lock()
				defer mu.Unlock()
				signalled = append(signalled, pid)
				alive[pid] = false
			}}
	}
	for _, pid := range []int{4001, 4002} {
		if err := os.WriteFile(pidf, []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		New(opts())
		if _, err := os.Stat(pidf); err == nil {
			t.Fatalf("pid file for %d kept", pid)
		}
	}
	if len(signalled) != 1 || signalled[0] != 4001 {
		t.Fatalf("signalled %v, want only the orphan 4001", signalled)
	}
}

func TestPidFileFollowsTheChild(t *testing.T) {
	h := newHarness(t)
	h.install(t, NLP)
	pidf := filepath.Join(h.m.final(NLP), "run.pid")
	if b, err := os.ReadFile(pidf); err != nil || strings.TrimSpace(string(b)) != "4242" {
		t.Fatalf("pid file: %q %v", b, err)
	}
	_ = h.m.Stop(NLP)
	if _, err := os.Stat(pidf); err == nil {
		t.Fatal("pid file left after a clean stop")
	}
}

func TestStatusDoesNotProbeOnEveryPoll(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 25; i++ {
		h.m.Status(MT)
		h.m.Status(NLP)
	}
	detects := 0
	for _, c := range h.r.runs {
		if strings.Contains(strings.Join(c.Args, " "), "version_info") {
			detects++
			if c.Args[0] != "-I" || c.Dir != os.TempDir() {
				t.Errorf("probe not isolated: %v dir=%q", c.Args, c.Dir)
			}
		}
	}
	if detects != 1 {
		t.Fatalf("python probed %d times for 50 polls", detects)
	}
}

func TestFailureDetailsNeverCarryHostPaths(t *testing.T) {
	h := newHarness(t)
	h.r.failRun = func(c Cmd) ([]string, error) {
		if strings.Contains(strings.Join(c.Args, " "), "pip --version") {
			return nil, errors.New("fork/exec /Users/alice/private/python: permission denied")
		}
		return nil, nil
	}
	_ = h.m.Install(context.Background(), NLP)
	st := h.waitState(t, NLP, StateFailed, true)
	if strings.Contains(st.ErrorDetail, "alice") || strings.Contains(st.ErrorDetail, "/Users/") {
		t.Fatalf("detail: %q", st.ErrorDetail)
	}
}

func TestHungChildIsKilledAndFailedNotRestarted(t *testing.T) {
	h := newHarness(t, func(o *Options) { o.ReadyTimeout = 40 * time.Millisecond })
	h.install(t, NLP)
	_ = h.m.Stop(NLP)
	h.pr.mu.Lock()
	h.pr.silent = true
	h.pr.mu.Unlock()
	base := h.r.startCount()
	if err := h.m.Start(NLP); err != nil {
		t.Fatal(err)
	}
	st := h.waitState(t, NLP, StateFailed, false)
	if st.ErrorKey != KeyStartFailed {
		t.Fatalf("%+v", st)
	}
	if h.r.startCount()-base != 1 || h.r.lastProc().term.Load() == 0 {
		t.Fatalf("starts=%d term=%d", h.r.startCount()-base, h.r.lastProc().term.Load())
	}
}

func TestRingTruncationKeepsUTF8(t *testing.T) {
	var r ring
	r.add(strings.Repeat("é", 400))
	r.add(strings.Repeat("a", 299) + "é" + "tail")
	for _, l := range r.lines {
		if !utf8.ValidString(l) {
			t.Fatalf("invalid UTF-8 after truncation: %q", l)
		}
	}
}

func TestTransformersAdvisoryIsFilteredAndChildEnvQuietsIt(t *testing.T) {
	h := newHarness(t)
	h.install(t, MT)
	h.m.addLog(h.m.comps[MT], "None of PyTorch, TensorFlow >= 2.0, or Flax have been found. Models won't be available and only tokenizers, configuration and file/data utilities can be used.")
	h.m.addLog(h.m.comps[MT], "real line")
	log := strings.Join(h.m.Status(MT).Log, "\n")
	if strings.Contains(log, "None of PyTorch") || !strings.Contains(log, "real line") {
		t.Fatalf("log: %s", log)
	}
	c := h.r.starts[0]
	for k, v := range map[string]string{"TRANSFORMERS_VERBOSITY": "error", "TRANSFORMERS_NO_ADVISORY_WARNINGS": "1", "HF_HUB_DISABLE_TELEMETRY": "1"} {
		if envOf(c, k) != v {
			t.Errorf("%s = %q", k, envOf(c, k))
		}
	}
}

func TestSlimImageGetsItsOwnNoPythonKey(t *testing.T) {
	t.Setenv("TANULOM_IMAGE", "slim")
	h := newHarness(t, func(o *Options) {
		o.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	})
	if st := h.m.Status(MT); st.Available || st.UnavailableKey != KeyNoPythonSlim {
		t.Fatalf("%+v", st)
	}
	if k := errKey(h.m.Install(context.Background(), MT)); k != KeyNoPythonSlim {
		t.Fatalf("install key = %q", k)
	}
}
