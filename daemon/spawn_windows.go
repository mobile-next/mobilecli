//go:build windows

package daemon

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// detach starts the daemon without a console and outside our process group.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow,
	}
}
