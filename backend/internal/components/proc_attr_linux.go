//go:build linux

package components

import (
	"os/exec"
	"syscall"
)

// setProcAttr gives the child its own process group (one signal reaches
// everything it spawned) and, on Linux, asks the kernel to SIGKILL it if this
// process dies — however it dies. The supervisor goroutine is locked to its
// thread while a child runs, because Pdeathsig is tied to the forking thread.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}
