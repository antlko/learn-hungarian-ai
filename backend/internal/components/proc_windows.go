//go:build windows

package components

import (
	"errors"
	"os"
	"os/exec"
)

// Managed components are not implemented on Windows: the app reports them
// unavailable and the Docker / own-server paths remain.
const pythonSupported = false

func lookPath(name string) (string, error) { return "", errors.New("unsupported platform") }
func setProcAttr(cmd *exec.Cmd)            {}
func termGroup(p *os.Process) {
	if p != nil {
		_ = p.Kill()
	}
}
func killGroup(p *os.Process) {
	if p != nil {
		_ = p.Kill()
	}
}

func signalPID(pid int, force bool)   {}
func procInfo(pid int) (string, bool) { return "", false }

type osDisk struct{}

func (osDisk) Free(path string) (uint64, error) { return 0, errors.New("unsupported platform") }
