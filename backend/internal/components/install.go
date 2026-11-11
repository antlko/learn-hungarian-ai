package components

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Install validates, runs the preflight synchronously (so a refusal reaches the
// button that was pressed) and then installs in the background. It returns
// ErrBusy while one runs, ErrInstalled when there is nothing to install, and
// keyed *Errors for the preflight.
func (m *Manager) Install(ctx context.Context, id string) error {
	c, py, err := m.claimInstall(id)
	if err != nil {
		return err
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.runInstall(m.ctx, c, py.path, false)
	}()
	return nil
}

// claimInstall is Install's synchronous half: the preflight and the single-flight
// claim. On success the component is `installing` and the caller must run
// runInstall, which releases the claim.
func (m *Manager) claimInstall(id string) (*comp, pyInfo, error) {
	c, err := m.get(id)
	if err != nil {
		return nil, pyInfo{}, err
	}
	py := m.python()
	if wkey := m.writable(); wkey != "" {
		return nil, pyInfo{}, &Error{Key: wkey}
	}
	if !py.ok {
		return nil, pyInfo{}, &Error{Key: py.key}
	}
	free, err := m.opt.Disk.Free(m.dir)
	if err != nil {
		return nil, pyInfo{}, &Error{Key: KeyReadOnly, Detail: m.redact(err.Error())}
	}
	if free < c.spec.diskNeedBytes {
		return nil, pyInfo{}, &Error{Key: KeyDiskFull, Detail: fmt.Sprintf("need ≈%d GB, %d GB free", c.spec.diskNeedBytes/gb, free/gb)}
	}

	m.mu.Lock()
	if m.installing != "" || c.busy || c.claim || c.supDone != nil {
		m.mu.Unlock()
		return nil, pyInfo{}, ErrBusy
	}
	if m.installedOnDisk(id) {
		m.mu.Unlock()
		return nil, pyInfo{}, ErrInstalled
	}
	c.busy = true
	m.installing = id
	c.state, c.step, c.progress, c.port = StateInstalling, StepVenv, 0, 0
	c.errKey, c.errMsg = "", ""
	c.log.reset()
	m.mu.Unlock()
	m.changed(id)
	return c, py, nil
}

// runInstall is the whole pipeline. Every failure funnels through fail, which
// removes everything this run created.
//
// foreground (the `component run` CLI) stops after the atomic rename: the CLI
// starts the service itself, on the address it was asked for, so there is no
// ephemeral child, no warm-up here and no provider=managed hand-over.
func (m *Manager) runInstall(ctx context.Context, c *comp, python string, foreground bool) {
	id := c.spec.id
	sp := c.spec
	stage, final := m.stage(id), m.final(id)
	started := time.Now()
	slog.Info("components: install started", "component", id)

	renamed := false
	fail := func(key, detail string) {
		if renamed {
			m.stopSupervisor(c)
			_ = os.RemoveAll(final)
		}
		_ = os.RemoveAll(stage)
		m.set(c, func(c *comp) {
			c.busy = false
			m.installing = ""
			c.state, c.step, c.port = StateFailed, "", 0
			c.errKey, c.errMsg = key, m.redact(detail)
		})
		slog.Warn("components: install failed", "component", id, "key", key)
	}
	cmdFail := func(err error, what string) {
		switch {
		case ctx.Err() != nil:
			fail(KeyInstallFailed, "cancelled")
		default:
			key := classify(m.logLines(c))
			fail(key, fmt.Sprintf("%s: %v\n%s", what, err, relevantTail(m.logLines(c), 8)))
		}
	}

	_ = os.RemoveAll(stage)
	if err := os.MkdirAll(filepath.Join(stage, "home"), 0o755); err != nil {
		fail(KeyReadOnly, err.Error())
		return
	}
	if err := writeSources(id, filepath.Join(stage, "src")); err != nil {
		fail(KeyInstallFailed, err.Error())
		return
	}
	logTo := func(l string) { m.addLog(c, l) }
	home := filepath.Join(stage, "home")
	venvPy := filepath.Join(stage, "venv", "bin", "python")
	srcDir := filepath.Join(stage, "src")
	modelDir := filepath.Join(stage, "models", sp.modelSub)
	// pip's download cache survives a failed attempt (it lives in the final
	// directory, which a failure does not remove) so a retry does not fetch torch
	// again; the successful rename below replaces that directory, freeing it.
	pipCache := "PIP_CACHE_DIR=" + filepath.Join(final, "cache", "pip")
	run := func(args []string, dir string, extra ...string) error {
		return m.opt.Runner.Run(ctx, Cmd{Path: args[0], Args: args[1:], Dir: dir,
			Env: installEnv(home, append([]string{pipCache}, extra...)...)}, logTo)
	}

	// 1. venv
	m.setStep(c, StepVenv, 0.02)
	if err := run([]string{python, "-I", "-m", "venv", filepath.Join(stage, "venv")}, stage); err != nil {
		cmdFail(err, "python -m venv")
		return
	}
	// Fail fast, with a key of its own, if the new environment cannot run pip.
	if err := run([]string{venvPy, "-I", "-c", "import sys; print(sys.version)"}, stage); err != nil {
		fail(KeyPipBroken, "venv python: "+err.Error())
		return
	}
	if err := run([]string{venvPy, "-I", "-m", "pip", "--version"}, stage); err != nil {
		fail(KeyPipBroken, "pip --version: "+err.Error()+"\n"+tail(m.logLines(c), 6))
		return
	}
	m.progress(c, 0.06)

	// 2. pip
	m.setStep(c, StepPip, 0.06)
	pipBase := []string{venvPy, "-I", "-m", "pip", "install", "--no-input", "--disable-pip-version-check", "--only-binary=:all:"}
	hi := 0.55
	if !sp.torch {
		hi = 0.75
	}
	counted := 0
	pipRun := func(args []string, lo, hi float64) error {
		counted = 0
		return m.opt.Runner.Run(ctx, Cmd{Path: args[0], Args: args[1:], Dir: srcDir, Env: installEnv(home, pipCache)},
			func(l string) {
				logTo(l)
				if strings.HasPrefix(strings.TrimSpace(l), "Collecting ") {
					counted++
					// Asymptotic: never claims 100% before pip has finished.
					frac := 1 - 1/(1+float64(counted)/float64(max(sp.pipExpected, 1))*2)
					m.progress(c, lo+(hi-lo)*frac)
				}
			})
	}
	if sp.torch {
		// The one-time conversion imports torch; the service never does, and it is
		// removed again below. torch's dependencies are NEVER resolved against the
		// PyTorch-only index (it lacks usable wheels for them and pip would fall
		// back to building sdists there) — see torchSteps.
		for _, step := range torchSteps(m.opt.GOOS) {
			if err := pipRun(append(append([]string{}, pipBase...), step...), 0.06, 0.2); err != nil {
				cmdFail(err, "pip install torch")
				return
			}
		}
	}
	if err := pipRun(append(append([]string{}, pipBase...), "-r", "requirements.txt"), 0.2, hi); err != nil {
		cmdFail(err, "pip install -r requirements.txt")
		return
	}
	m.progress(c, hi)

	// 3. model preparation (mt: convert once, then reclaim torch and the HF cache)
	if sp.prepare != "" {
		m.setStep(c, StepModel, hi)
		if err := os.MkdirAll(filepath.Dir(modelDir), 0o755); err != nil {
			fail(KeyReadOnly, err.Error())
			return
		}
		env := []string{sp.modelEnv + "=" + modelDir}
		if err := run([]string{venvPy, "-c", sp.prepare}, srcDir, env...); err != nil {
			cmdFail(err, "model preparation")
			return
		}
		m.progress(c, 0.85)
		if sp.torch {
			if err := run([]string{venvPy, "-I", "-m", "pip", "uninstall", "-y", "--disable-pip-version-check", "torch"}, srcDir); err != nil {
				// Not fatal: the service runs fine with torch present, it only
				// costs disk. Say so in the log and carry on.
				logTo("could not remove torch; continuing")
			}
			_ = os.RemoveAll(filepath.Join(home, ".cache"))
		}
	}
	_ = os.MkdirAll(modelDir, 0o755)
	if ctx.Err() != nil {
		fail(KeyInstallFailed, "cancelled")
		return
	}

	// 4. commit: marker, then the atomic rename.
	_ = os.WriteFile(marker(stage), []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644)
	if err := os.RemoveAll(final); err != nil {
		fail(KeyReadOnly, err.Error())
		return
	}
	if err := os.Rename(stage, final); err != nil {
		fail(KeyReadOnly, err.Error())
		return
	}
	renamed = true
	if foreground {
		m.set(c, func(c *comp) {
			c.busy = false
			m.installing = ""
			c.state, c.step, c.progress = StateInstalled, "", 1
		})
		slog.Info("components: install finished", "component", id, "seconds", int(time.Since(started).Seconds()))
		return
	}

	// 5. start.
	m.setStep(c, StepStart, 0.88)
	if err := m.launch(c); err != nil {
		fail(KeyStartFailed, err.Error())
		return
	}
	if err := m.waitRunning(ctx, c); err != nil {
		fail(KeyStartFailed, err.Error()+"\n"+tail(m.logLines(c), 6))
		return
	}

	// 6. hand over: provider=managed.
	if m.opt.OnInstalled != nil {
		if err := m.opt.OnInstalled(id); err != nil {
			fail(KeyInstallFailed, err.Error())
			return
		}
	}
	m.set(c, func(c *comp) {
		c.busy = false
		m.installing = ""
		c.step, c.progress = "", 1
	})
	slog.Info("components: install finished", "component", id, "seconds", int(time.Since(started).Seconds()))
}

func (m *Manager) setStep(c *comp, step string, p float64) {
	if m.opt.OnLog != nil {
		m.opt.OnLog(c.spec.id, "step: "+step)
	}
	m.set(c, func(c *comp) {
		c.step = step
		if p > c.progress {
			c.progress = p
		}
	})
}

func (m *Manager) logLines(c *comp) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return c.log.last(ringCap)
}

// waitRunning blocks until the supervisor reports the child up, or it fails.
func (m *Manager) waitRunning(ctx context.Context, c *comp) error {
	deadline := time.Now().Add(m.opt.ReadyTimeout)
	t := time.NewTicker(m.opt.PollInterval)
	defer t.Stop()
	for {
		m.mu.Lock()
		st, key := c.state, c.errKey
		m.mu.Unlock()
		switch st {
		case StateRunning:
			return nil
		case StateFailed:
			return fmt.Errorf("service did not start (%s)", key)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("service did not answer within %s", m.opt.ReadyTimeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}

var (
	// Network: only genuine connection failures. A dependency that cannot be
	// resolved ("Could not find a version", "No matching distribution",
	// ResolutionImpossible) is NOT a network problem — the owner's first real
	// install said "no network" over a healthy connection.
	reNetwork  = regexp.MustCompile(`(?i)temporary failure in name resolution|name or service not known|nodename nor servname|connection (refused|reset|aborted|timed out)|connectionerror|connecterror|failed to establish a new connection|max retries exceeded|network is unreachable|sslerror|ssl: |read ?timed? ?out|readtimeout|proxyerror`)
	reDisk     = regexp.MustCompile(`(?i)no space left on device|disk quota exceeded`)
	reRelevant = regexp.MustCompile(`(?i)error|could not|no matching|resolution|requires|failed|not found|conflict`)
)

func classifyOne(s string) string {
	switch {
	case reDisk.MatchString(s):
		return KeyDiskFull
	case reNetwork.MatchString(s):
		return KeyNetwork
	}
	return KeyInstallFailed
}

// classify maps the tail of a failed command's output to an error key.
func classify(lines []string) string {
	return classifyOne(tail(lines, 30))
}

// relevantTail is errorDetail: the last few lines that look like the cause,
// or the plain tail when none do.
func relevantTail(lines []string, n int) string {
	var picked []string
	for _, l := range lines {
		if reRelevant.MatchString(l) {
			picked = append(picked, l)
		}
	}
	if len(picked) == 0 {
		picked = lines
	}
	return tail(picked, n)
}

// Uninstall stops the child and deletes `<data>/components/<id>/`. The caller
// resets the provider.
func (m *Manager) Uninstall(id string) error {
	c, err := m.get(id)
	if err != nil {
		return err
	}
	m.mu.Lock()
	if c.busy || c.claim {
		m.mu.Unlock()
		return ErrBusy
	}
	c.busy = true
	m.mu.Unlock()

	m.stopSupervisor(c)
	err = os.RemoveAll(m.final(id))
	if e2 := os.RemoveAll(m.stage(id)); err == nil {
		err = e2
	}
	m.set(c, func(c *comp) {
		c.busy = false
		c.state, c.step, c.progress, c.port = "", "", 0, 0
		c.errKey, c.errMsg = "", ""
		c.log.reset()
	})
	return err
}

// torchSteps are the pip argument lists (after the common `pip install` flags)
// that install torch 2.5.1 for the one-time model conversion.
//
//   - darwin and anything else: PyPI's own wheel (CPU/MPS only on macOS, small).
//   - linux: the PyPI wheel is the 2 GB CUDA build, so the wheel alone comes from
//     the PyTorch CPU index with --no-deps, and the runtime dependencies from the
//     default index. Binary wheels only: a missing wheel fails fast and clearly.
func torchSteps(goos string) [][]string {
	if goos != "linux" {
		return [][]string{{"torch==2.5.1"}}
	}
	return [][]string{
		{"--no-deps", "--index-url", "https://download.pytorch.org/whl/cpu", "torch==2.5.1"},
		// torch 2.5.1 Requires-Dist (CPU, no nvidia/triton): filelock,
		// typing-extensions>=4.8.0, networkx, jinja2, fsspec, setuptools (py>=3.12),
		// sympy==1.13.1.
		{"filelock", "typing-extensions>=4.8.0", "sympy==1.13.1", "networkx", "jinja2", "fsspec", "setuptools"},
	}
}
