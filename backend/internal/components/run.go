package components

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"time"
)

// `tanulom component run <id>` — ADR-061 amendment. The same install code and the
// same embedded sources as the in-app install, but the service runs as THIS
// process's foreground child on an address the operator chose, so a second
// compose service built from the one published image can host it. The only two
// inputs besides the id are a host (an IP literal) and a port; no URL, path or
// argument reaches a command line.

// Run-mode refusals, distinct from *Error (those are i18n keys for the UI).
var (
	// ErrUsage: the id, host or port is not acceptable (the CLI exits 2).
	ErrUsage = fmt.Errorf("usage")
)

// RunOptions are the CLI's only inputs besides the component id.
type RunOptions struct {
	Host string // IP literal; default 0.0.0.0
	Port int    // 1024-65535; default is the service's own (mt 5003)
	// Out receives the child's output lines (the CLI prints them to stdout).
	Out func(line string)
}

// DefaultRunHost is the bind address of `component run`: a compose-network
// sidecar must be reachable by service name. The managed mode stays 127.0.0.1.
const DefaultRunHost = "0.0.0.0"

// DefaultPort is the documented port of a component (0 for an unknown id).
func DefaultPort(id string) int { return specs[id].defPort }

// ValidateRun checks the three inputs and returns the resolved host and port.
func ValidateRun(id, host string, port int) (string, int, error) {
	sp, ok := specs[id]
	if !ok {
		return "", 0, fmt.Errorf("%w: unknown component %q (want mt)", ErrUsage, id)
	}
	if host == "" {
		host = DefaultRunHost
	}
	// Only the two addresses the readiness probe can reach: it always asks
	// 127.0.0.1, so a service bound to any other address would never look ready,
	// be killed after ReadyTimeout and restart-loop under restart: unless-stopped.
	if ip := net.ParseIP(host); ip == nil || (ip.String() != "0.0.0.0" && ip.String() != "127.0.0.1") {
		return "", 0, fmt.Errorf("%w: --host must be 0.0.0.0 (all interfaces, the default) or 127.0.0.1", ErrUsage)
	}
	if port == 0 {
		port = sp.defPort
	}
	if port < 1024 || port > 65535 {
		return "", 0, fmt.Errorf("%w: --port must be between 1024 and 65535", ErrUsage)
	}
	return host, port, nil
}

// EnsureInstalled makes sure the component is installed, SYNCHRONOUSLY, and
// returns once it is. A complete install (marker present) is left alone; the
// preflight (writable data dir, usable Python) runs either way, because the
// venv's interpreter is a link to the system one and means nothing without it.
func (m *Manager) EnsureInstalled(ctx context.Context, id string) (string, error) {
	if m.Installed(id) {
		if wkey := m.writable(); wkey != "" {
			return "", &Error{Key: wkey}
		}
		py := m.python()
		if !py.ok {
			return "", &Error{Key: py.key}
		}
		return py.path, nil
	}
	c, py, err := m.claimInstall(id)
	if err != nil {
		return "", err
	}
	m.runInstall(ctx, c, py.path, true)
	m.mu.Lock()
	st, key, msg := c.state, c.errKey, c.errMsg
	m.mu.Unlock()
	if st == StateFailed {
		return "", &Error{Key: key, Detail: msg}
	}
	return py.path, nil
}

// RunForeground installs when needed, starts the service in the foreground on
// host:port and blocks until it exits or ctx is cancelled (the caller cancels
// it on SIGTERM/SIGINT). Returns the child's exit code: 0 after a requested
// stop. There is no restart loop — the container's restart policy is the
// supervisor here, and a crash must be visible to it.
func (m *Manager) RunForeground(ctx context.Context, id string, ro RunOptions) (int, error) {
	host, port, err := ValidateRun(id, ro.Host, ro.Port)
	if err != nil {
		return 2, err
	}
	c, err := m.get(id)
	if err != nil {
		return 2, err
	}
	out := ro.Out
	if out == nil {
		out = func(string) {}
	}
	if _, err := m.EnsureInstalled(ctx, id); err != nil {
		return 1, err
	}
	// Refresh the sources from the binary, so an upgraded image runs its own.
	if err := writeSources(id, filepath.Join(m.final(id), "src")); err != nil {
		return 1, fmt.Errorf("embedded sources: %w", err)
	}

	proc, err := m.opt.Runner.Start(m.childCmdAt(c, host, port), func(l string) { out(m.redact(l)) })
	if err != nil {
		return 1, fmt.Errorf("start: %w", err)
	}
	m.mu.Lock()
	c.proc, c.port, c.state = proc, port, StateStarting
	m.mu.Unlock()
	slog.Info("components: running in the foreground", "component", id, "host", host, "port", port)

	// stopped is set when WE asked the child to end, so its signal death is a
	// clean exit rather than a failure.
	stopped := false
	exitCode := func() int {
		if ec, ok := proc.(interface{ ExitCode() int }); ok {
			if code := ec.ExitCode(); code >= 0 {
				return code
			}
		}
		if stopped {
			return 0
		}
		return 1
	}

	ready := make(chan error, 1)
	go func() { ready <- m.foregroundReady(ctx, c, proc, port) }()

	for {
		select {
		case <-proc.Done():
			code := exitCode()
			slog.Info("components: service exited", "component", id, "code", code)
			return code, nil
		case <-ctx.Done():
			stopped = true
			slog.Info("components: stopping", "component", id)
			m.terminate(proc)
			return exitCode(), nil
		case err := <-ready:
			ready = nil
			if err != nil {
				slog.Error("components: service did not become ready", "component", id, "error", err.Error())
				stopped = true
				m.terminate(proc)
				return 1, err
			}
			slog.Info("components: ready", "component", id, "port", port)
		}
	}
}

// foregroundReady waits for the service to answer /health. A nil `ready` channel in the caller's select
// blocks forever, which is what we want once it has fired.
func (m *Manager) foregroundReady(ctx context.Context, c *comp, p Proc, port int) error {
	deadline := time.Now().Add(m.opt.ReadyTimeout)
	t := time.NewTicker(m.opt.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-p.Done():
			return nil
		case <-t.C:
		}
		if m.opt.Prober.Alive(ctx, port) {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the service did not answer /health within %s", m.opt.ReadyTimeout)
		}
	}
	m.set(c, func(c *comp) { c.state = StateRunning })
	return nil
}
