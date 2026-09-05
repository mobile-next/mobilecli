//go:build !windows

package daemon

import (
	"errors"
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
