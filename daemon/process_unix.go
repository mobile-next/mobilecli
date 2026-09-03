//go:build !windows

package daemon

import (
	"errors"
	"os"
	"syscall"
)

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	// EPERM means the process exists but belongs to another user
	return err == nil || errors.Is(err, syscall.EPERM)
}

func killProcess(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}
