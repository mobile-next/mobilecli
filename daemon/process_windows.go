//go:build windows

package daemon

import "os"

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	// on Windows FindProcess opens a handle and fails when the pid is gone
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	_ = p.Release()
	return true
}

func killProcess(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}
