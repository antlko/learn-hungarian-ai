package components

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Start runs an installed component. It returns at once; the state moves
// starting -> running when the child answers /health. Idempotent while it runs.
func (m *Manager) Start(id string) error {
	c, err := m.get(id)
	if err != nil {
		return err
	}
	m.mu.Lock()
	if c.busy || c.claim {
		m.mu.Unlock()
		return ErrBusy
	}
	if c.supDone != nil {
		m.mu.Unlock()
		return nil // already supervised
	}
	// The claim keeps Uninstall (which checks it) from deleting the directory
	// between these checks and the moment the supervisor owns it.
	c.claim = true
	m.mu.Unlock()
	if !m.installedOnDisk(id) {
		m.mu.Lock()
		c.claim = false
		m.mu.Unlock()
		return ErrNotInstalled
	}
	err = m.launch(c)
	m.mu.Lock()
	c.claim = false
	m.mu.Unlock()
	return err
}

// Stop ends the child (SIGTERM, then SIGKILL after StopGrace) and waits for it.
func (m *Manager) Stop(id string) error {
	c, err := m.get(id)
	if err != nil {
		return err
	}
	m.mu.Lock()
	busy := c.busy
	m.mu.Unlock()
	if busy {
		return ErrBusy
	}
	if m.stopSupervisor(c) {
		m.set(c, func(c *comp) {
			c.state, c.port = StateStopped, 0
			c.errKey, c.errMsg = "", ""
		})
	}
	return nil
}

// stopSupervisor cancels the supervisor, terminates the child and waits until
// both are gone. Reports whether there was one.
func (m *Manager) stopSupervisor(c *comp) bool {
	m.mu.Lock()
	cancel, done := c.supCancel, c.supDone
	m.mu.Unlock()
	if done == nil {
		return false
	}
	cancel()
	<-done
	return true
}

// launch starts the supervisor goroutine for c.
func (m *Manager) launch(c *comp) error {
	if err := writeSources(c.spec.id, filepath.Join(m.final(c.spec.id), "src")); err != nil {
		return fmt.Errorf("embedded sources: %w", err)
	}
	ctx, cancel := context.WithCancel(m.ctx)
	done := make(chan struct{})
	m.mu.Lock()
	if c.supDone != nil {
		m.mu.Unlock()
		cancel()
		return nil
	}
	c.supCancel, c.supDone = cancel, done
	c.state, c.step, c.errKey, c.errMsg = StateStarting, "", "", ""
	m.mu.Unlock()
	m.changed(c.spec.id)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer func() {
			m.mu.Lock()
			c.supCancel, c.supDone, c.proc, c.port = nil, nil, nil, 0
			m.mu.Unlock()
			close(done)
		}()
		m.supervise(ctx, c)
	}()
	return nil
}

// childEnv is the COMPLETE environment of a running service: an allowlist,
// never the server's (no TANULOM_*, no secret, no proxy credentials).
func (m *Manager) childEnv(c *comp, port int) []string { return m.childEnvAt(c, "127.0.0.1", port) }

// childEnvAt is childEnv with the bind host chosen by the caller. The managed
// mode always passes 127.0.0.1; only `component run` passes anything else.
func (m *Manager) childEnvAt(c *comp, host string, port int) []string {
	sp := c.spec
	root := m.final(sp.id)
	env := baseEnv(filepath.Join(root, "home"))
	return append(env,
		sp.hostEnv+"="+host,
		sp.portEnv+"="+strconv.Itoa(port),
		sp.modelEnv+"="+filepath.Join(root, "models", sp.modelSub),
		// After conversion torch is uninstalled on purpose; transformers then
		// logs a scary-sounding advisory about it. Keep the log about real problems.
		"TRANSFORMERS_VERBOSITY=error",
		"TRANSFORMERS_NO_ADVISORY_WARNINGS=1",
		"HF_HUB_DISABLE_TELEMETRY=1",
	)
}

func (m *Manager) childCmd(c *comp, port int) Cmd { return m.childCmdAt(c, "127.0.0.1", port) }

func (m *Manager) childCmdAt(c *comp, host string, port int) Cmd {
	root := m.final(c.spec.id)
	return Cmd{
		Path: filepath.Join(root, "venv", "bin", "python"),
		Args: []string{filepath.Join(root, "src", "app.py")},
		Dir:  filepath.Join(root, "src"),
		Env:  m.childEnvAt(c, host, port),
	}
}

// supervise runs the child until ctx is cancelled, restarting it with
// exponential backoff, and gives up (state failed) after MaxRestarts crashes
// inside RestartWindow.
func (m *Manager) supervise(ctx context.Context, c *comp) {
	id := c.spec.id
	var crashes []time.Time
	addrRetries := 0
	// Pdeathsig (Linux) is tied to the forking thread: keep this goroutine on one.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	logf, closeLog := m.openRunLog(id)
	defer closeLog()

	fail := func(key, detail string) {
		m.set(c, func(c *comp) {
			c.state, c.port, c.errKey, c.errMsg = StateFailed, 0, key, m.redact(detail)
		})
		slog.Warn("components: giving up", "component", id, "key", key)
	}

	for ctx.Err() == nil {
		port, err := m.opt.PickPort()
		if err != nil {
			fail(KeyStartFailed, err.Error())
			return
		}
		var recent []string
		var rmu sync.Mutex
		onLine := func(l string) {
			logf(l)
			m.addLog(c, l)
			rmu.Lock()
			recent = append(recent, l)
			if len(recent) > 20 {
				recent = recent[1:]
			}
			rmu.Unlock()
		}
		proc, err := m.opt.Runner.Start(m.childCmd(c, port), onLine)
		if err == nil {
			// The port exists in memory only — never in config, never on disk.
			m.writePID(id, proc)
			m.mu.Lock()
			c.gen++
			gen := c.gen
			c.proc, c.port, c.state = proc, port, StateStarting
			m.mu.Unlock()
			m.changed(id)
			m.wg.Add(1)
			go func() {
				defer m.wg.Done()
				m.awaitReady(ctx, c, proc, gen, port)
			}()

			select {
			case <-proc.Done():
			case <-ctx.Done():
				m.terminate(proc)
				m.removePID(id)
				return
			}
			m.removePID(id)
			m.mu.Lock()
			c.port = 0
			timedOut := c.timedOut
			c.timedOut = false
			m.mu.Unlock()
			if timedOut && ctx.Err() == nil {
				fail(KeyStartFailed, fmt.Sprintf("the service did not answer /health within %s\n%s",
					m.opt.ReadyTimeout, tail(tailOf(&rmu, recent), 6)))
				return
			}
		} else {
			onLine("start failed: " + err.Error())
		}
		if ctx.Err() != nil {
			return
		}

		tailLines := tailOf(&rmu, recent)
		// Another process took the port between choosing it and binding it: pick
		// again, and do not count it as the service crashing.
		if err == nil && addrInUse(tailLines) && addrRetries < 5 {
			addrRetries++
			continue
		}

		now := time.Now()
		crashes = append(crashes, now)
		kept := crashes[:0]
		for _, t := range crashes {
			if now.Sub(t) <= m.opt.RestartWindow {
				kept = append(kept, t)
			}
		}
		crashes = kept
		if len(crashes) > m.opt.MaxRestarts {
			fail(KeyCrashed, m.redact(tail(tailLines, 6)))
			return
		}
		delay := m.opt.Backoff(len(crashes))
		slog.Warn("components: child exited; restarting", "component", id, "in", delay, "crashes", len(crashes))
		m.set(c, func(c *comp) { c.state = StateStarting })
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

func tailOf(mu *sync.Mutex, lines []string) []string {
	mu.Lock()
	defer mu.Unlock()
	return append([]string(nil), lines...)
}

func addrInUse(lines []string) bool {
	for _, l := range lines {
		l = strings.ToLower(l)
		if strings.Contains(l, "address already in use") || strings.Contains(l, "eaddrinuse") {
			return true
		}
	}
	return false
}

// terminate: SIGTERM, then SIGKILL after StopGrace; returns when it is gone.
func (m *Manager) terminate(p Proc) {
	p.Terminate()
	select {
	case <-p.Done():
	case <-time.After(m.opt.StopGrace):
		p.Kill()
		<-p.Done()
	}
}

// awaitReady flips starting -> running once the child answers /health, unless
// it has already been replaced or has exited.
func (m *Manager) awaitReady(ctx context.Context, c *comp, p Proc, gen, port int) {
	deadline := time.Now().Add(m.opt.ReadyTimeout)
	t := time.NewTicker(m.opt.PollInterval)
	defer t.Stop()
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		case <-p.Done():
			return
		case <-t.C:
		}
		if m.opt.Prober.Alive(ctx, port) {
			m.mu.Lock()
			if c.gen == gen && c.state == StateStarting {
				c.state, c.errMsg = StateRunning, ""
			}
			m.mu.Unlock()
			m.changed(c.spec.id)
			return
		}
	}
	// Alive but silent past the deadline: a hung child is worse than a dead
	// one. Mark the generation and kill it; the supervisor then fails the
	// component (errors.componentStartFailed) instead of restarting it.
	if ctx.Err() == nil {
		select {
		case <-p.Done():
		default:
			m.mu.Lock()
			if c.gen == gen {
				c.timedOut = true
			}
			m.mu.Unlock()
			m.terminate(p)
		}
	}
}

// openRunLog appends to `<id>/logs/run.log`, rotating at 1 MiB (one old file
// kept), so a crash-looping service cannot fill the disk. Failure to open is
// not an error: the ring buffer still has the lines.
func (m *Manager) openRunLog(id string) (func(string), func()) {
	dir := filepath.Join(m.final(id), "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return func(string) {}, func() {}
	}
	path := filepath.Join(dir, "run.log")
	var mu sync.Mutex
	var f *os.File
	var size int64
	open := func() {
		if st, err := os.Stat(path); err == nil && st.Size() > 1<<20 {
			_ = os.Rename(path, path+".1")
		}
		f, _ = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		size = 0
		if f != nil {
			if st, err := f.Stat(); err == nil {
				size = st.Size()
			}
		}
	}
	open()
	write := func(line string) {
		mu.Lock()
		defer mu.Unlock()
		if f == nil {
			return
		}
		line = m.redact(line)
		n, _ := f.WriteString(line + "\n")
		size += int64(n)
		if size > 1<<20 {
			_ = f.Close()
			open()
		}
	}
	closeFn := func() {
		mu.Lock()
		defer mu.Unlock()
		if f != nil {
			_ = f.Close()
			f = nil
		}
	}
	return write, closeFn
}
