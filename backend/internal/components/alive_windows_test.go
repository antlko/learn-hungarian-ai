//go:build windows

package components

func processAlive(pid int) bool { return false }
