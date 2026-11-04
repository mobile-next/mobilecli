package devices

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"
)

// theNativeAgentMapperBootsTheAgentOverTheTunnel maps the embedded agent dylib
// into a live foreground app and confirms its server bound a port — the full
// injectAgentNative path minus the go-ios transport, exercised over the userspace
// tunnel forwarder so it needs no WDA/devicekit setup.
//
// Gated on NATIVE_INJECT_TEST; skipped otherwise so CI without a device stays
// green. Env: NATIVE_INJECT_HOST (tunnel IPv6), NATIVE_INJECT_TUNNEL_PORT (local
// forwarder), NATIVE_INJECT_PORT (device debugproxy), NATIVE_INJECT_PID (a FRESH
// foreground-app pid — the agent must not already be injected).
func TestNativeAgentMapperBootsOverTunnel(t *testing.T) {
	if os.Getenv("NATIVE_INJECT_TEST") == "" {
		t.Skip("set NATIVE_INJECT_TEST=1 with a connected device to run")
	}
	host := os.Getenv("NATIVE_INJECT_HOST")
	tunnelPort := mustAtoi(t, "NATIVE_INJECT_TUNNEL_PORT")
	proxyPort := mustAtoi(t, "NATIVE_INJECT_PORT")
	pid := mustAtoi(t, "NATIVE_INJECT_PID")

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", tunnelPort), 8*time.Second)
	if err != nil {
		t.Fatalf("dial forwarder: %v", err)
	}
	defer func() { _ = conn.Close() }()
	// go-ios userspace-tunnel forwarder header: 16-byte IPv6 + LE uint32 port
	ip := net.ParseIP(host).To16()
	if ip == nil {
		t.Fatalf("bad NATIVE_INJECT_HOST %q", host)
	}
	hdr := make([]byte, 20)
	copy(hdr[:16], ip)
	binary.LittleEndian.PutUint32(hdr[16:], uint32(proxyPort))
	if _, err := conn.Write(hdr); err != nil {
		t.Fatalf("write forwarder header: %v", err)
	}

	port, err := mapAgentOverConn(conn, pid)
	if err != nil {
		t.Fatalf("mapAgentOverConn: %v", err)
	}
	if port == 0 {
		t.Fatal("agent mapped but server bound no port")
	}
	t.Logf("agent booted; server bound device port %d", port)
}

func mustAtoi(t *testing.T, env string) int {
	t.Helper()
	v, err := strconv.Atoi(os.Getenv(env))
	if err != nil {
		t.Fatalf("%s: %v", env, err)
	}
	return v
}
