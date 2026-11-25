package ios

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	goios "github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/forward"
	"github.com/mobile-next/mobilecli/internal/utils"
)

type PortForwarder struct {
	udid         string
	connListener *forward.ConnListener
	tunListener  net.Listener // set instead of connListener by ForwardOverTunnel
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

	if pf.connListener != nil || pf.tunListener != nil {
		return fmt.Errorf("port forwarding is already running from %d to %d", pf.srcPort, pf.dstPort)
	}

	if srcPort < 0 || srcPort > 65535 {
		return fmt.Errorf("invalid source port %d: must be between 0 and 65535", srcPort)
	}

	if dstPort < 0 || dstPort > 65535 {
		return fmt.Errorf("invalid destination port %d: must be between 0 and 65535", dstPort)
	}

	pf.srcPort = srcPort
	pf.dstPort = dstPort

	device, err := GetDevice(pf.udid)
	if err != nil {
		return fmt.Errorf("failed to get device %s: %w", pf.udid, err)
	}

	connListener, err := forward.Forward(device, uint16(srcPort), uint16(dstPort)) //nolint:gosec // ports validated to be 0-65535 above
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

	if pf.connListener == nil && pf.tunListener == nil {
		return fmt.Errorf("no port forwarding running")
	}

	var err error
	if pf.tunListener != nil {
		err = pf.tunListener.Close()
		pf.tunListener = nil
	} else {
		err = pf.connListener.Close()
	}
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

	return pf.connListener != nil || pf.tunListener != nil
}

func (pf *PortForwarder) GetPorts() (srcPort, dstPort int) {
	pf.forwardMutex.Lock()
	defer pf.forwardMutex.Unlock()

	return pf.srcPort, pf.dstPort
}

// ForwardOverTunnel forwards localhost:srcPort to dstAddr:dstPort inside the device's
// iOS 17+ tunnel instead of through usbmuxd. usbmuxd reaches a Wi-Fi-only device on its
// network interface, where a loopback-bound agent never answers; the tunnel address is
// reachable only from this Mac, so the agent can listen there without being exposed.
func (pf *PortForwarder) ForwardOverTunnel(srcPort int, device goios.DeviceEntry, dstAddr string, dstPort int) error {
	pf.forwardMutex.Lock()
	defer pf.forwardMutex.Unlock()

	if pf.connListener != nil || pf.tunListener != nil {
		return fmt.Errorf("port forwarding is already running from %d to %d", pf.srcPort, pf.dstPort)
	}

	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", srcPort))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", srcPort, err)
	}

	pf.tunListener = listener
	pf.srcPort = srcPort
	pf.dstPort = dstPort

	go func() {
		for {
			clientConn, err := listener.Accept()
			if err != nil {
				return
			}
			go proxyOverTunnel(clientConn, device, dstAddr, dstPort)
		}
	}()

	utils.Verbose("Port forwarding started from %d to [%s]:%d over the device tunnel", srcPort, dstAddr, dstPort)
	return nil
}

func proxyOverTunnel(clientConn net.Conn, device goios.DeviceEntry, dstAddr string, dstPort int) {
	defer func() { _ = clientConn.Close() }()

	deviceConn, err := goios.ConnectTUNDevice(dstAddr, dstPort, device)
	if err != nil {
		// go-ios returns the connection along with the error when writing the destination fails
		if deviceConn != nil {
			_ = deviceConn.Close()
		}
		utils.Verbose("tunnel forward: could not reach [%s]:%d: %v", dstAddr, dstPort, err)
		return
	}
	defer func() { _ = deviceConn.Close() }()

	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(deviceConn, clientConn); done <- struct{}{} }()
	go func() { _, _ = io.Copy(clientConn, deviceConn); done <- struct{}{} }()
	<-done
}
