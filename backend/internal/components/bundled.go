package components

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// Enable / Disable of a bundled component (ADR-080). «Включить» never touches
// the network: it verifies the bundle, unpacks the interpreter, installs the
// wheels from the bundle with pip --no-index, writes runtime.stamp and starts
// the child, which loads every voice into memory. A current stamp skips straight
// to the start. Disable is Stop (the API layer also resets the provider).

var reHashMismatch = regexp.MustCompile(`(?i)do not match the hashes|hash mismatch|THESE PACKAGES DO NOT MATCH`)

// bundledPipEnv is the COMPLETE environment of the offline pip run: the base
// allowlist plus the variables that make a network or a config file impossible.
// Never the install allowlist (proxies, CA bundles): pip has nowhere to go.
func bundledPipEnv(home string) []string {
	return append(baseEnv(home),
		"PIP_NO_INDEX=1",
		"PIP_CONFIG_FILE=/dev/null",
		"PIP_DISABLE_PIP_VERSION_CHECK=1",
		"PIP_NO_INPUT=1",
		"PIP_NO_CACHE_DIR=1",
	)
}

// enable validates, claims and runs the enable in the background. handover
// selects whether OnInstalled (provider=managed) runs at the end: it does for
// the button, not for boot or a provider change that already says managed.
func (m *Manager) enable(id string, handover bool) error {
	c, err := m.get(id)
	if err != nil {
		return err
	}
	if err := m.claimEnable(c); err != nil {
		return err
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.runEnable(m.ctx, c, false, handover)
	}()
	return nil
}

// claimEnable is enable's synchronous half: preflight plus the single-flight
// claim. On success the component is `installing` and runEnable must follow.
func (m *Manager) claimEnable(c *comp) error {
	id := c.spec.id
	if _, key := m.locateBundle(); key != "" {
		return &Error{Key: key}
	}
	if len(m.Voices()) == 0 {
		return &Error{Key: KeyNoVoices}
	}
	if wkey := m.writable(); wkey != "" {
		return &Error{Key: wkey}
	}
	ready := m.runtimeReady(id)
	if !ready {
		free, err := m.opt.Disk.Free(m.dir)
		if err != nil {
			return &Error{Key: KeyReadOnly, Detail: m.redact(err.Error())}
		}
		if free < c.spec.diskNeedBytes {
			return &Error{Key: KeyDiskFull, Detail: fmt.Sprintf("need ≈%d GB, %d GB free", c.spec.diskNeedBytes/gb, free/gb)}
		}
	}
	m.mu.Lock()
	if m.installing != "" || c.busy || c.claim {
		m.mu.Unlock()
		return ErrBusy
	}
	if c.supDone != nil {
		m.mu.Unlock()
		return ErrInstalled // already on
	}
	c.busy = true
	m.installing = id
	c.state, c.step, c.progress, c.port = StateInstalling, StepVoices, 0, 0
	if !ready {
		c.step = StepVerify
	}
	c.errKey, c.errMsg = "", ""
	c.log.reset()
	m.mu.Unlock()
	m.changed(id)
	return nil
}

// runEnable is the pipeline. Every failure funnels through fail, which removes
// the staging directory; the live environment is only ever replaced by a rename
// of a fully built one.
//
// foreground (`component run`) stops before the child starts: the CLI starts it
// itself on the address it was given.
func (m *Manager) runEnable(ctx context.Context, c *comp, foreground, handover bool) {
	id := c.spec.id
	root := m.final(id)
	stage := filepath.Join(root, ".unpack")
	started := time.Now()
	slog.Info("components: enable started", "component", id)

	fail := func(key, detail string) {
		_ = os.RemoveAll(stage)
		m.stopSupervisor(c)
		m.set(c, func(c *comp) {
			c.busy = false
			m.installing = ""
			c.state, c.step, c.port = StateFailed, "", 0
			c.errKey, c.errMsg = key, m.redact(detail)
		})
		slog.Warn("components: enable failed", "component", id, "key", key)
	}
	failErr := func(err error, what string) {
		var key = KeyInstallFailed
		switch {
		case ctx.Err() != nil:
			fail(KeyInstallFailed, "cancelled")
			return
		default:
			if e, ok := asError(err); ok {
				fail(e.Key, e.Detail)
				return
			}
			if reDisk.MatchString(err.Error()) {
				key = KeyDiskFull
			}
			fail(key, fmt.Sprintf("%s: %v", what, err))
		}
	}

	if !m.runtimeReady(id) {
		b, bkey := m.locateBundle()
		if bkey != "" {
			fail(bkey, "")
			return
		}
		man, lock, err := b.readMeta()
		if err != nil {
			fail(KeyBundleCorrupt, "MANIFEST or lock unreadable")
			return
		}
		stamp := b.stampOf(man, lock)
		wheels, unpack := filepath.Join(stage, "wheelhouse"), filepath.Join(stage, "unpack")
		_ = os.RemoveAll(stage)
		_ = os.RemoveAll(filepath.Join(root, "python.old")) // a crash between the two renames
		if err := os.MkdirAll(wheels, 0o755); err != nil {
			fail(KeyReadOnly, err.Error())
			return
		}
		if err := os.MkdirAll(unpack, 0o755); err != nil {
			fail(KeyReadOnly, err.Error())
			return
		}
		home := filepath.Join(root, "home")
		if err := os.MkdirAll(home, 0o755); err != nil {
			fail(KeyReadOnly, err.Error())
			return
		}

		m.setStep(c, StepVerify, 0.02)
		tarball, err := verifyCopy(ctx, b, man, lock, wheels, func(f float64) { m.progress(c, 0.02+0.28*f) })
		if err != nil {
			failErr(err, "verify")
			return
		}

		m.setStep(c, StepUnpack, 0.3)
		if err := untarGz(ctx, tarball, unpack); err != nil {
			failErr(err, "unpack")
			return
		}
		_ = os.Remove(tarball) // the archive is no longer needed; frees ~25 MB
		py := filepath.Join(unpack, "python", "bin", "python3")
		if fi, err := os.Stat(py); err != nil || !fi.Mode().IsRegular() {
			fail(KeyBundleCorrupt, "the archive has no python/bin/python3")
			return
		}
		m.progress(c, 0.55)
		pip := []string{py, "-I", "-m", "pip", "install", "--no-index", "--no-deps", "--require-hashes",
			"--only-binary=:all:", "-r", filepath.Join(wheels, lockName), "--find-links", wheels}
		err = m.opt.Runner.Run(ctx, Cmd{Path: pip[0], Args: pip[1:], Dir: stage, Env: bundledPipEnv(home)},
			func(l string) { m.addLog(c, l) })
		if err != nil {
			lines := m.logLines(c)
			if ctx.Err() == nil && reHashMismatch.MatchString(tail(lines, 30)) {
				fail(KeyBundleCorrupt, "pip: "+relevantTail(lines, 6))
				return
			}
			failErr(fmt.Errorf("%w\n%s", err, relevantTail(lines, 8)), "pip install")
			return
		}
		m.progress(c, 0.75)

		// Commit. The stamp goes last, so a crash leaves "not unpacked", never "half".
		// Rename-swap: the live runtime is only discarded after the new one is in place.
		live, old := filepath.Join(root, "python"), filepath.Join(root, "python.old")
		_ = os.RemoveAll(old)
		_ = os.Remove(filepath.Join(root, stampName))
		hadOld := false
		if _, err := os.Lstat(live); err == nil {
			if err := m.rename(live, old); err != nil {
				fail(KeyReadOnly, err.Error())
				return
			}
			hadOld = true
		}
		restore := func() {
			_ = os.RemoveAll(live)
			if hadOld {
				_ = os.Rename(old, live)
			}
		}
		if err := m.rename(filepath.Join(unpack, "python"), live); err != nil {
			restore()
			fail(KeyReadOnly, err.Error())
			return
		}
		tmp := filepath.Join(root, stampName+".tmp")
		if err := os.WriteFile(tmp, []byte(stamp), 0o644); err != nil {
			restore()
			fail(KeyReadOnly, err.Error())
			return
		}
		if err := os.Rename(tmp, filepath.Join(root, stampName)); err != nil {
			restore()
			fail(KeyReadOnly, err.Error())
			return
		}
		_ = os.RemoveAll(old)
		_ = os.RemoveAll(stage)
	}

	m.setStep(c, StepVoices, 0.8)
	if err := m.extractVoices(ctx); err != nil {
		failErr(err, "voices")
		return
	}
	if ctx.Err() != nil {
		fail(KeyInstallFailed, "cancelled")
		return
	}
	if foreground {
		m.set(c, func(c *comp) {
			c.busy = false
			m.installing = ""
			c.state, c.step, c.progress = StateInstalled, "", 1
		})
		slog.Info("components: enable finished", "component", id, "seconds", int(time.Since(started).Seconds()))
		return
	}

	// The child loads every voice before /health says ready; Status keeps
	// showing the voices step for that time (busy + starting).
	if err := m.launch(c); err != nil {
		fail(KeyStartFailed, err.Error())
		return
	}
	if err := m.waitRunning(ctx, c); err != nil {
		// The supervisor's own account (a voice that would not load, from /health)
		// beats a generic line plus the log tail.
		m.mu.Lock()
		reason := c.errMsg
		m.mu.Unlock()
		if reason == "" {
			reason = err.Error() + "\n" + tail(m.logLines(c), 6)
		}
		fail(KeyStartFailed, reason)
		return
	}
	if handover && m.opt.OnInstalled != nil {
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
	slog.Info("components: enable finished", "component", id, "seconds", int(time.Since(started).Seconds()))
}

// uninstallBundled removes the unpacked environment (interpreter, stamp, the
// extracted voices, staging) and never the bundle itself.
func (m *Manager) uninstallBundled(id string) error {
	root := m.final(id)
	var first error
	for _, p := range []string{"python", "python.old", stampName, stampName + ".tmp", ".unpack", "voices"} {
		if err := os.RemoveAll(filepath.Join(root, p)); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// rename is os.Rename behind a seam so a test can fail the swap half-way.
func (m *Manager) rename(from, to string) error {
	if m.opt.Rename != nil {
		return m.opt.Rename(from, to)
	}
	return os.Rename(from, to)
}
