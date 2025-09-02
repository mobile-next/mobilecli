package ios

import (
	"fmt"
	"sync"

	goios "github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/forward"
	"github.com/mobile-next/mobilecli/utils"
)

type PortForwarder struct {
	udid         string
	connListener *forward.ConnListener
	forwardMutex sync.Mutex
	srcPort      int
	dstPort      int
}

func NewPortForwarder(udid string) *PortForwarder {
	return &PortForwarder{
		udid: udid,
	}
}

func (pf *PortForwarder) Forward(srcPort, dstPort int) error {
	pf.forwardMutex.Lock()
	defer pf.forwardMutex.Unlock()

	if pf.connListener != nil {
		return fmt.Errorf("port forwarding is already running from %d to %d", pf.srcPort, pf.dstPort)
	}

	pf.srcPort = srcPort
	pf.dstPort = dstPort

	device, err := goios.GetDevice(pf.udid)
	if err != nil {
		return fmt.Errorf("failed to get device %s: %w", pf.udid, err)
	}

	connListener, err := forward.Forward(device, uint16(srcPort), uint16(dstPort))
	if err != nil {
		return fmt.Errorf("failed to create port forwarder: %w", err)
	}

	pf.connListener = connListener
	utils.Verbose("Port forwarding started from %d to %d", srcPort, dstPort)

	return nil
}

func (pf *PortForwarder) Stop() error {
	pf.forwardMutex.Lock()
	defer pf.forwardMutex.Unlock()

	if pf.connListener == nil {
		return fmt.Errorf("no port forwarding running")
	}

	err := pf.connListener.Close()
	if err != nil {
		utils.Verbose("Error stopping port forwarding %d->%d: %v", pf.srcPort, pf.dstPort, err)
	}

	utils.Verbose("Stopping port forwarding %d->%d", pf.srcPort, pf.dstPort)
	pf.connListener = nil
	pf.srcPort = 0
	pf.dstPort = 0

	return err
}

func (pf *PortForwarder) IsRunning() bool {
	pf.forwardMutex.Lock()
	defer pf.forwardMutex.Unlock()

	return pf.connListener != nil
}

func (pf *PortForwarder) GetPorts() (srcPort, dstPort int) {
	pf.forwardMutex.Lock()
	defer pf.forwardMutex.Unlock()

	return pf.srcPort, pf.dstPort
}
