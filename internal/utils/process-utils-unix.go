//go:build unix

package utils

import (
	"os/exec"
	"syscall"
)

// ConfigureDetachedProcAttr configures the command to run in a separate process group
// on Unix systems, allowing for proper cleanup when the parent process is terminated.
func ConfigureDetachedProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}
}
