//go:build !windows && !linux

package components

import (
	"os/exec"
	"syscall"
)

// setProcAttr: own process group. macOS has no Pdeathsig, so a SIGKILLed or
// crashed server leaves its children running; the next start reaps them from
// their pid files (reapStale). Documented limitation, ADR-061.
func setProcAttr(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
