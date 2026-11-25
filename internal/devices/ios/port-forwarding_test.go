package ios

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	goios "github.com/danielpaulus/go-ios/ios"
)

// fakeUserspaceTUN stands in for go-ios's userspace tunnel: each connection starts with the
// 16-byte destination IP and a little-endian uint32 port, then carries the payload.
type fakeUserspaceTUN struct {
	listener net.Listener
	headers  chan []byte
}

func startFakeUserspaceTUN(t *testing.T, echo bool) *fakeUserspaceTUN {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeUserspaceTUN{listener: l, headers: make(chan []byte, 4)}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				header := make([]byte, 20)
				if _, err := io.ReadFull(c, header); err != nil {
					return
				}
				f.headers <- header
				if echo {
					_, _ = io.Copy(c, c)
				}
			}(conn)
		}
	}()
	return f
}

func (f *fakeUserspaceTUN) device() goios.DeviceEntry {
	return goios.DeviceEntry{
		UserspaceTUN:     true,
		UserspaceTUNHost: "127.0.0.1",
		UserspaceTUNPort: f.listener.Addr().(*net.TCPAddr).Port,
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestForwardOverTunnelProxiesThroughTunnel(t *testing.T) {
	tun := startFakeUserspaceTUN(t, true)
	pf := NewPortForwarder("udid-1")
	port := freePort(t)
	const tunnelAddr = "fd35:8002:295a::1"

	if err := pf.ForwardOverTunnel(port, tun.device(), tunnelAddr, 12004); err != nil {
		t.Fatal(err)
	}
	defer pf.Stop()

	if !pf.IsRunning() {
		t.Fatal("forwarder should report running")
	}
	if src, dst := pf.GetPorts(); src != port || dst != 12004 {
		t.Fatalf("GetPorts = (%d, %d), want (%d, 12004)", src, dst, port)
	}

	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 4)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatal(err)
	}
	if string(reply) != "ping" {
		t.Fatalf("reply = %q, want %q", reply, "ping")
	}

	header := <-tun.headers
	if !bytes.Equal(header[:16], net.ParseIP(tunnelAddr).To16()) {
		t.Fatalf("tunnel destination = %v, want %s", net.IP(header[:16]), tunnelAddr)
	}
	if got := binary.LittleEndian.Uint32(header[16:]); got != 12004 {
		t.Fatalf("tunnel destination port = %d, want 12004", got)
	}
}

func TestForwardOverTunnelRejectsSecondForward(t *testing.T) {
	tun := startFakeUserspaceTUN(t, true)
	pf := NewPortForwarder("udid-1")
	if err := pf.ForwardOverTunnel(freePort(t), tun.device(), "fd00::1", 12004); err != nil {
		t.Fatal(err)
	}
	defer pf.Stop()

	if err := pf.ForwardOverTunnel(freePort(t), tun.device(), "fd00::1", 12004); err == nil {
		t.Fatal("expected an error for a second tunnel forward")
	}
	if err := pf.Forward(freePort(t), 12004); err == nil {
		t.Fatal("expected an error for a usbmuxd forward while the tunnel forward runs")
	}
}

func TestForwardOverTunnelStop(t *testing.T) {
	tun := startFakeUserspaceTUN(t, true)
	pf := NewPortForwarder("udid-1")
	port := freePort(t)
	if err := pf.ForwardOverTunnel(port, tun.device(), "fd00::1", 12004); err != nil {
		t.Fatal(err)
	}

	if err := pf.Stop(); err != nil {
		t.Fatal(err)
	}
	if pf.IsRunning() {
		t.Fatal("forwarder should not report running after Stop")
	}
	if _, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), time.Second); err == nil {
		t.Fatal("local port should be closed after Stop")
	}
	if err := pf.Stop(); err == nil {
		t.Fatal("expected an error stopping a forwarder that is not running")
	}
}

func TestForwardOverTunnelClosesClientWhenTunnelUnreachable(t *testing.T) {
	tun := startFakeUserspaceTUN(t, false)
	device := tun.device()
	_ = tun.listener.Close() // nothing answers on the tunnel port any more

	pf := NewPortForwarder("udid-1")
	port := freePort(t)
	if err := pf.ForwardOverTunnel(port, device, "fd00::1", 12004); err != nil {
		t.Fatal(err)
	}
	defer pf.Stop()

	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("expected the client connection to be closed")
	}
}
