package ios

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/mobile-next/mobilecli/utils"
)

type PortForwarder struct {
	udid           string
	forwardProcess *exec.Cmd
	forwardCancel  context.CancelFunc
	forwardMutex   sync.Mutex
	srcPort        int
	dstPort        int
}

func NewPortForwarder(udid string) *PortForwarder {
	return &PortForwarder{
		udid: udid,
	}
}

func (pf *PortForwarder) Forward(srcPort, dstPort int) error {
	pf.forwardMutex.Lock()
	defer pf.forwardMutex.Unlock()

	if pf.forwardProcess != nil {
		return fmt.Errorf("port forwarding is already running from %d to %d", pf.srcPort, pf.dstPort)
	}

	ctx, cancel := context.WithCancel(context.Background())
	pf.forwardCancel = cancel
	pf.srcPort = srcPort
	pf.dstPort = dstPort

	cmdName, err := findGoIosPath()
	if err != nil {
		return fmt.Errorf("failed to find go-ios path: %w", err)
	}

	srcPortStr := strconv.Itoa(srcPort)
	dstPortStr := strconv.Itoa(dstPort)

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, cmdName, "forward", srcPortStr, dstPortStr, "--udid", pf.udid)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	utils.Verbose("Starting port forwarding process: %s", cmd.String())

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start port forwarding process: %w", err)
	}

	pf.forwardProcess = cmd
	utils.Verbose("Port forwarding started from %d to %d with PID: %d", srcPort, dstPort, cmd.Process.Pid)

	go func() {
		err := cmd.Wait()
		utils.Verbose("Port forwarding process %d->%d terminated")

		pf.forwardMutex.Lock()
		pf.forwardProcess = nil
		pf.forwardCancel = nil
		pf.forwardMutex.Unlock()
		if err != nil {
			utils.Verbose("Port forwarding process %d->%d terminated with error: %v", srcPort, dstPort, err)
			utils.Verbose("Stdout: %s", stdout.String())
			utils.Verbose("Stderr: %s", stderr.String())

		} else {
			utils.Verbose("Port forwarding process %d->%d terminated normally", srcPort, dstPort)
		}
	}()

	time.Sleep(5 * time.Second)
	return nil
}

func (pf *PortForwarder) Stop() error {
	pf.forwardMutex.Lock()
	defer pf.forwardMutex.Unlock()

	if pf.forwardProcess == nil {
		return fmt.Errorf("no port forwarding process running")
	}

	if pf.forwardCancel != nil {
		pf.forwardCancel()
	}

	utils.Verbose("Stopping port forwarding process %d->%d with PID: %d", pf.srcPort, pf.dstPort, pf.forwardProcess.Process.Pid)
	pf.forwardProcess = nil
	pf.forwardCancel = nil
	pf.srcPort = 0
	pf.dstPort = 0

	return nil
}

func (pf *PortForwarder) GetForwardingPID() int {
	pf.forwardMutex.Lock()
	defer pf.forwardMutex.Unlock()

	if pf.forwardProcess != nil && pf.forwardProcess.Process != nil {
		return pf.forwardProcess.Process.Pid
	}
	return 0
}

func (pf *PortForwarder) IsRunning() bool {
	pf.forwardMutex.Lock()
	defer pf.forwardMutex.Unlock()

	return pf.forwardProcess != nil
}

func (pf *PortForwarder) GetPorts() (srcPort, dstPort int) {
	pf.forwardMutex.Lock()
	defer pf.forwardMutex.Unlock()

	return pf.srcPort, pf.dstPort
}
