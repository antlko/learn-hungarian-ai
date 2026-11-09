//go:build !windows

package components

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// pythonSupported is false on platforms the manager does not implement.
const pythonSupported = true

func lookPath(name string) (string, error) { return exec.LookPath(name) }

// signalPID signals a whole process group by its leader's pid.
func signalPID(pid int, force bool) {
	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}
	_ = syscall.Kill(-pid, sig)
}

// procInfo reports whether pid is alive and its command line (via ps, which
// both Linux and macOS ship).
func procInfo(pid int) (string, bool) {
	if pid <= 0 || syscall.Kill(pid, 0) != nil {
		return "", false
	}
	out, err := exec.Command("ps", "-o", "command=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

func termGroup(p *os.Process) {
	if p != nil {
		_ = syscall.Kill(-p.Pid, syscall.SIGTERM)
	}
}

func killGroup(p *os.Process) {
	if p != nil {
		_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
	}
}

// osDisk answers free space for a path with statfs.
type osDisk struct{}

func (osDisk) Free(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
