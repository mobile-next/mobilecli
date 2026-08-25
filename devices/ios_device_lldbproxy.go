package devices

import (
	"fmt"
	"io"
	"net"
	"strings"

	goios "github.com/danielpaulus/go-ios/ios"
	"github.com/mobile-next/mobilecli/devices/ios/debugserver"
	"github.com/mobile-next/mobilecli/utils"
)

// This file is the low-level GDB Remote Serial Protocol bridge that lets the
// system `lldb` binary talk to a process on the device. The device's debugserver
// is reachable only through the CoreDevice debug proxy (go-ios); lldb expects to
// connect to a local debugserver. startLLDBProxy/lldbProxyConn bridge the two.
//
// It is feature-agnostic: any injected-agent feature (webview today; view-tree,
// network capture, etc. later) relies on it to evaluate code in the target.

// startLLDBProxy pre-attaches to pid on the device via the debug proxy, then
// starts a local TCP listener for LLDB. Pre-attaching before listening ensures
// the proxy can respond to LLDB's handshake immediately upon connection.
// Returns the local port and a stop function.
func startLLDBProxy(device goios.DeviceEntry, proxyPort, pid int) (int, func(), error) {
	utils.Verbose("lldb-proxy: connecting to device debug proxy port %d", proxyPort)
	devConn, err := goios.ConnectTUNDevice(device.Address, proxyPort, device)
	if err != nil {
		return 0, nil, fmt.Errorf("lldb-proxy: connect to device: %w", err)
	}

	devGDB := debugserver.NewGDBServer(devConn)
	utils.Verbose("lldb-proxy: pre-attaching to pid %d", pid)
	stopReply, err := devGDB.Request(fmt.Sprintf("vAttach;%x", pid))
	if err != nil || !strings.HasPrefix(stopReply, "T") {
		devConn.Close()
		return 0, nil, fmt.Errorf("lldb-proxy: vAttach pid %d: err=%v resp=%q", pid, err, stopReply)
	}
	utils.Verbose("lldb-proxy: pre-attached, stop=%q", stopReply)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		devConn.Close()
		return 0, nil, fmt.Errorf("listen for lldb proxy: %w", err)
	}
	localPort := ln.Addr().(*net.TCPAddr).Port
	go func() {
		defer ln.Close()
		defer devConn.Close()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		lldbProxyConn(conn, devGDB, pid)
	}()
	return localPort, func() { ln.Close() }, nil
}

// lldbProxyConn is a GDB RSP bridge between LLDB and an already-attached
// device debugserver. Handles negotiation packets locally, forwards all others
// packet-by-packet with ack-mode translation (LLDB: no-ack; device: ack).
func lldbProxyConn(c net.Conn, devGDB *debugserver.GDBServer, pid int) {
	defer c.Close()

	// debugserver sends '+' immediately upon accepting a connection;
	// LLDB waits for this before sending the first packet.
	c.Write([]byte("+")) //nolint:errcheck

	noAck := false

	gdbChecksum := func(pkt string) byte {
		var sum byte
		for i := 0; i < len(pkt); i++ {
			sum += pkt[i]
		}
		return sum
	}

	sendToLLDB := func(pkt string) {
		ck := gdbChecksum(pkt)
		var s string
		if !noAck {
			s = "+"
		}
		s += fmt.Sprintf("$%s#%02x", pkt, ck)
		c.Write([]byte(s)) //nolint:errcheck
	}

	recvFromLLDB := func() (string, error) {
		buf := make([]byte, 1)
		for {
			if _, err := io.ReadFull(c, buf); err != nil {
				return "", err
			}
			if buf[0] == '$' {
				break
			}
		}
		var pkt strings.Builder
		for {
			if _, err := io.ReadFull(c, buf); err != nil {
				return "", err
			}
			if buf[0] == '#' {
				break
			}
			pkt.WriteByte(buf[0])
		}
		cksumBuf := make([]byte, 2)
		if _, err := io.ReadFull(c, cksumBuf); err != nil {
			return "", err
		}
		return pkt.String(), nil
	}

	for {
		pkt, err := recvFromLLDB()
		if err != nil {
			return
		}
		utils.Verbose("lldb-proxy ← LLDB: %.300s", pkt)

		// switchToNoAck is set by QStartNoAckMode and applied AFTER sendToLLDB
		// so the OK response goes out in ack mode (with '+') as LLDB expects.
		switchToNoAck := false
		var reply string
		switch {
		case pkt == "QStartNoAckMode":
			reply = "OK"
			switchToNoAck = true

		case strings.HasPrefix(pkt, "qSupported"):
			reply = "PacketSize=65536;vContSupported+"

		case pkt == "QThreadSuffixSupported",
			pkt == "QListThreadsInStopReply",
			pkt == "qVAttachOrWaitSupported",
			pkt == "QEnableErrorStrings":
			reply = "OK"

		case strings.HasPrefix(pkt, "vCont?"):
			reply = "vCont;c;C;s;S"

		case pkt == "k":
			// LLDB wants to kill — detach instead so the app keeps running
			devGDB.Request(fmt.Sprintf("D;%x", pid)) //nolint:errcheck
			return

		case strings.HasPrefix(pkt, "D"):
			devReply, _ := devGDB.Request(pkt)
			utils.Verbose("lldb-proxy → LLDB (detach): %d bytes", len(devReply))
			sendToLLDB(devReply)
			return

		default:
			// forward to device (devGDB uses ack mode: sends "+$pkt#XX")
			devReply, err := devGDB.Request(pkt)
			if err != nil {
				utils.Verbose("lldb-proxy: device error for %q: %v", pkt[:min(len(pkt), 40)], err)
				return
			}
			reply = devReply
		}

		utils.Verbose("lldb-proxy → LLDB: %d bytes", len(reply))
		sendToLLDB(reply)
		if switchToNoAck {
			noAck = true
		}
	}
}
