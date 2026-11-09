//go:build !windows

package components

import "syscall"

func processAlive(pid int) bool { return syscall.Kill(pid, 0) == nil }
