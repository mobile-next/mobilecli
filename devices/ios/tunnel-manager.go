package ios

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"syscall"

	"github.com/mobile-next/mobilecli/utils"
)

type TunnelManager struct {
	udid          string
	tunnelProcess *exec.Cmd
	tunnelCancel  context.CancelFunc
	tunnelMutex   sync.Mutex
}

func NewTunnelManager(udid string) *TunnelManager {
	return &TunnelManager{
		udid: udid,
	}
}

func (tm *TunnelManager) StartTunnel() error {
	return tm.StartTunnelWithCallback(nil)
}

func (tm *TunnelManager) StartTunnelWithCallback(onProcessDied func(error)) error {
	tm.tunnelMutex.Lock()
	defer tm.tunnelMutex.Unlock()

	if tm.tunnelProcess != nil {
		return fmt.Errorf("tunnel is already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	tm.tunnelCancel = cancel

	cmdName, err := findGoIosPath()
	if err != nil {
		return fmt.Errorf("failed to find go-ios path: %w", err)
	}

	cmd := exec.CommandContext(ctx, cmdName, "tunnel", "start", "--userspace", "--udid", tm.udid)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start tunnel process: %w", err)
	}

	tm.tunnelProcess = cmd
	utils.Verbose("Tunnel started in background with PID: %d", cmd.Process.Pid)

	if onProcessDied != nil {
		go func() {
			waitErr := cmd.Wait()
			tm.tunnelMutex.Lock()
			tm.tunnelProcess = nil
			tm.tunnelCancel = nil
			tm.tunnelMutex.Unlock()
			onProcessDied(waitErr)
		}()
	} else {
		go func() {
			cmd.Wait()
			tm.tunnelMutex.Lock()
			tm.tunnelProcess = nil
			tm.tunnelCancel = nil
			tm.tunnelMutex.Unlock()
		}()
	}

	return nil
}

func (tm *TunnelManager) StopTunnel() error {
	tm.tunnelMutex.Lock()
	defer tm.tunnelMutex.Unlock()

	if tm.tunnelProcess == nil {
		return fmt.Errorf("no tunnel process running")
	}

	if tm.tunnelCancel != nil {
		tm.tunnelCancel()
	}

	utils.Verbose("Stopping tunnel process with PID: %d", tm.tunnelProcess.Process.Pid)
	tm.tunnelProcess = nil
	tm.tunnelCancel = nil

	return nil
}

func (tm *TunnelManager) GetTunnelPID() int {
	tm.tunnelMutex.Lock()
	defer tm.tunnelMutex.Unlock()

	if tm.tunnelProcess != nil && tm.tunnelProcess.Process != nil {
		return tm.tunnelProcess.Process.Pid
	}
	return 0
}

func findGoIosPath() (string, error) {
	if path, err := exec.LookPath("go-ios"); err == nil {
		return path, nil
	}

	if path, err := exec.LookPath("ios"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("neither go-ios nor ios found in PATH")
}