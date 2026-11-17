//go:build !windows

package daemon

import (
	"os/exec"
	"syscall"
)

// detach puts the daemon in its own session so it survives the CLI's terminal.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
