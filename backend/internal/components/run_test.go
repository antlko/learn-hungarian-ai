package components

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func lineSink() (func(string), func() string) {
	var mu sync.Mutex
	var b strings.Builder
	return func(l string) { mu.Lock(); b.WriteString(l + "\n"); mu.Unlock() },
		func() string { mu.Lock(); defer mu.Unlock(); return b.String() }
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func markInstalled(t *testing.T, h *harness, id string) {
	t.Helper()
	if err := os.MkdirAll(h.m.final(id), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker(h.m.final(id)), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRun(t *testing.T) {
	cases := []struct {
		id, host string
		port     int
		wantHost string
		wantPort int
		bad      bool
	}{
		{"mt", "", 0, "0.0.0.0", 5003, false},
		{"nlp", "", 0, "0.0.0.0", 5002, false},
		{"nlp", "127.0.0.1", 5099, "127.0.0.1", 5099, false},
		{"mt", "::1", 1024, "", 0, true},
		{"mt", "10.0.0.5", 5003, "", 0, true},
		{"mt", "0.0.0.0", 1024, "0.0.0.0", 1024, false},
		{"tts", "", 0, "", 0, true},
		{"mt", "localhost", 0, "", 0, true},
		{"mt", "http://x", 0, "", 0, true},
		{"mt", "", 80, "", 0, true},
		{"mt", "", 1023, "", 0, true},
		{"mt", "", 70000, "", 0, true},
	}
	for _, c := range cases {
		h, p, err := ValidateRun(c.id, c.host, c.port)
		if c.bad {
			if !errors.Is(err, ErrUsage) {
				t.Errorf("%+v: want usage error, got %v", c, err)
			}
			continue
		}
		if err != nil || h != c.wantHost || p != c.wantPort {
			t.Errorf("%+v: got %q %d %v", c, h, p, err)
		}
	}
}

func TestEnsureInstalledInstallsWhenMissingWithoutStarting(t *testing.T) {
	h := newHarness(t)
	if _, err := h.m.EnsureInstalled(context.Background(), NLP); err != nil {
		t.Fatal(err)
	}
	if !h.m.Installed(NLP) {
		t.Fatal("marker missing after install")
	}
	if n := h.r.startCount(); n != 0 {
		t.Fatalf("install-only started %d children", n)
	}
	if len(h.installs) != 0 {
		t.Fatalf("provider hand-over ran in foreground mode: %v", h.installs)
	}
	if st := h.m.Status(NLP); st.State != StateInstalled {
		t.Fatalf("%+v", st)
	}
}

func TestEnsureInstalledSkipsWhenInstalled(t *testing.T) {
	h := newHarness(t)
	markInstalled(t, h, MT)
	if _, err := h.m.EnsureInstalled(context.Background(), MT); err != nil {
		t.Fatal(err)
	}
	for _, c := range h.r.runs {
		if strings.Contains(strings.Join(c.Args, " "), "pip install") || strings.Contains(strings.Join(c.Args, " "), "-m venv") {
			t.Fatalf("re-installed: %v", c.Args)
		}
	}
}

func TestEnsureInstalledRefusesWithoutPython(t *testing.T) {
	for _, slim := range []bool{false, true} {
		h := newHarness(t, func(o *Options) {
			o.LookPath = func(string) (string, error) { return "", errors.New("nope") }
		})
		want := KeyNoPython
		if slim {
			t.Setenv("TANULOM_IMAGE", "slim")
			want = KeyNoPythonSlim
		}
		// Both for a fresh and for an already-installed volume.
		_, err := h.m.EnsureInstalled(context.Background(), MT)
		if errKey(err) != want {
			t.Fatalf("slim=%v fresh: %v", slim, err)
		}
		markInstalled(t, h, NLP)
		_, err = h.m.EnsureInstalled(context.Background(), NLP)
		if errKey(err) != want {
			t.Fatalf("slim=%v installed: %v", slim, err)
		}
	}
}

func TestRunForegroundBindsRequestedHostAndKeepsEnvAllowlisted(t *testing.T) {
	t.Setenv("TANULOM_SECRET_X", "s3cret")
	h := newHarness(t)
	markInstalled(t, h, MT)
	ctx, cancel := context.WithCancel(context.Background())
	out, got := lineSink()
	res := make(chan int, 1)
	go func() {
		code, err := h.m.RunForeground(ctx, MT, RunOptions{Out: out})
		if err != nil {
			t.Error(err)
		}
		res <- code
	}()
	waitFor(t, "start", func() bool { return h.r.startCount() == 1 })
	h.r.mu.Lock()
	cmd := h.r.starts[0]
	h.r.mu.Unlock()
	env := strings.Join(cmd.Env, "\n")
	for _, w := range []string{"MT_HOST=0.0.0.0", "MT_PORT=5003", "MT_MODEL_DIR="} {
		if !strings.Contains(env, w) {
			t.Errorf("env lacks %s:\n%s", w, env)
		}
	}
	if strings.Contains(env, "TANULOM_") || strings.Contains(env, "s3cret") {
		t.Errorf("env leaked: %s", env)
	}
	if cmd.Path != filepath.Join(h.m.final(MT), "venv", "bin", "python") {
		t.Errorf("path %q", cmd.Path)
	}
	if h.pr.warm.Load() != 0 {
		t.Error("mt must not be warmed up through /warmup")
	}
	cancel()
	if code := <-res; code != 0 {
		t.Fatalf("requested stop exit code %d", code)
	}
	_ = got
}

func TestRunForegroundForwardsSignalAndWarmsUpNLP(t *testing.T) {
	h := newHarness(t)
	markInstalled(t, h, NLP)
	ctx, cancel := context.WithCancel(context.Background())
	res := make(chan int, 1)
	go func() {
		code, _ := h.m.RunForeground(ctx, NLP, RunOptions{Host: "127.0.0.1", Port: 5099})
		res <- code
	}()
	waitFor(t, "warmup", func() bool { return h.pr.warm.Load() == 1 })
	h.r.mu.Lock()
	env := strings.Join(h.r.starts[0].Env, "\n")
	h.r.mu.Unlock()
	if !strings.Contains(env, "NLP_HOST=127.0.0.1") || !strings.Contains(env, "NLP_PORT=5099") {
		t.Fatalf("env:\n%s", env)
	}
	select {
	case <-res:
		t.Fatal("returned before the signal: it must keep running after warm-up")
	case <-time.After(30 * time.Millisecond):
	}
	cancel() // what SIGTERM does in main
	if code := <-res; code != 0 {
		t.Fatalf("code %d", code)
	}
	if h.r.lastProc().term.Load() != 1 {
		t.Fatal("SIGTERM was not forwarded to the child")
	}
}

func TestRunForegroundReturnsChildExitCode(t *testing.T) {
	h := newHarness(t)
	markInstalled(t, h, MT)
	h.pr.silent = true
	res := make(chan int, 1)
	go func() {
		code, _ := h.m.RunForeground(context.Background(), MT, RunOptions{})
		res <- code
	}()
	waitFor(t, "start", func() bool { return h.r.startCount() == 1 })
	p := h.r.lastProc()
	p.code.Store(3)
	p.exit()
	if code := <-res; code != 3 {
		t.Fatalf("code %d, want the child's 3", code)
	}
	if h.r.startCount() != 1 {
		t.Fatal("a crash must not be restarted in-process")
	}
}

func TestRunForegroundRefusesBadInputBeforeAnyWork(t *testing.T) {
	h := newHarness(t)
	code, err := h.m.RunForeground(context.Background(), "tts", RunOptions{})
	if code != 2 || !errors.Is(err, ErrUsage) {
		t.Fatalf("%d %v", code, err)
	}
	code, err = h.m.RunForeground(context.Background(), MT, RunOptions{Port: 22})
	if code != 2 || !errors.Is(err, ErrUsage) || len(h.r.runs) != 0 {
		t.Fatalf("%d %v runs=%d", code, err, len(h.r.runs))
	}
}

func TestManagedModeStillBindsLoopback(t *testing.T) {
	h := newHarness(t)
	h.install(t, MT)
	h.r.mu.Lock()
	env := strings.Join(h.r.starts[0].Env, "\n")
	h.r.mu.Unlock()
	if !strings.Contains(env, "MT_HOST=127.0.0.1") {
		t.Fatalf("managed child must stay on loopback:\n%s", env)
	}
}
