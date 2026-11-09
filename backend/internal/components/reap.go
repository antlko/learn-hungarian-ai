package components

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (m *Manager) pidFile(id string) string { return filepath.Join(m.final(id), "run.pid") }

func (m *Manager) writePID(id string, p Proc) {
	if pid := p.Pid(); pid > 0 {
		_ = os.WriteFile(m.pidFile(id), []byte(strconv.Itoa(pid)+"\n"), 0o644)
	}
}

func (m *Manager) removePID(id string) { _ = os.Remove(m.pidFile(id)) }

// reapStale kills a child left behind by a previous server that died without
// stopping it (SIGKILL, crash, OOM; on macOS also any exit the kernel does not
// clean up). It only acts on a pid whose command line still names THIS
// component's venv, so a recycled pid is never signalled.
func (m *Manager) reapStale() {
	for _, id := range IDs() {
		b, err := os.ReadFile(m.pidFile(id))
		if err != nil {
			continue
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
		if cmd, alive := m.opt.ProcInfo(pid); alive && strings.Contains(cmd, filepath.Join(m.final(id), "venv")) {
			m.opt.SignalPID(pid, false)
			deadline := time.Now().Add(m.opt.StopGrace)
			for time.Now().Before(deadline) {
				if _, alive := m.opt.ProcInfo(pid); !alive {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if _, alive := m.opt.ProcInfo(pid); alive {
				m.opt.SignalPID(pid, true)
			}
		}
		_ = os.Remove(m.pidFile(id))
	}
}
