package devices

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestNativeAgentMapperBootsOverTunnel maps the embedded agent dylib
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

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", tunnelPort))
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

func TestAgentIsCachedPerAppProcess(t *testing.T) {
	first := deviceAgentKey{udid: "per-process-device", pid: 1}
	second := deviceAgentKey{udid: "per-process-device", pid: 2}

	rememberInjectedAgent(first, &injectedAgent{bundleID: "com.example.first", localPort: 1111})

	if agent, ok := cachedDeviceAgent(first); !ok || agent.localPort != 1111 {
		t.Errorf("first app agent = %v, %v; want local port 1111", agent, ok)
	}
	if agent, ok := cachedDeviceAgent(second); ok {
		t.Errorf("second app reached the first app's agent: %v", agent)
	}
}

func TestRelaunchedAppForgetsItsPreviousProcess(t *testing.T) {
	beforeRelaunch := deviceAgentKey{udid: "relaunch-device", pid: 1}
	afterRelaunch := deviceAgentKey{udid: "relaunch-device", pid: 2}

	rememberInjectedAgent(beforeRelaunch, &injectedAgent{bundleID: "com.example.app"})
	rememberInjectedAgent(afterRelaunch, &injectedAgent{bundleID: "com.example.app"})

	if _, ok := cachedDeviceAgent(beforeRelaunch); ok {
		t.Error("the agent of the dead process is still cached")
	}
}

func TestSwitchingAppsKeepsTheOtherAppsAgent(t *testing.T) {
	firstApp := deviceAgentKey{udid: "switching-device", pid: 1}
	secondApp := deviceAgentKey{udid: "switching-device", pid: 2}

	rememberInjectedAgent(firstApp, &injectedAgent{bundleID: "com.example.first"})
	rememberInjectedAgent(secondApp, &injectedAgent{bundleID: "com.example.second"})

	if _, ok := cachedDeviceAgent(firstApp); !ok {
		t.Error("the first app lost its agent when another app was injected")
	}
}

func TestEachDeviceHasItsOwnSetupLock(t *testing.T) {
	firstLookup := deviceAgentLock("device-a")
	secondLookup := deviceAgentLock("device-a")
	otherDevice := deviceAgentLock("device-b")

	if firstLookup != secondLookup {
		t.Error("one device must always get the same lock")
	}
	if firstLookup == otherDevice {
		t.Error("two devices must not wait for each other")
	}
}

func TestRefusedAttachExplainsThatTheAppMustBeDebuggable(t *testing.T) {
	err := attachError(803, "E96")

	if !strings.Contains(err.Error(), "only debuggable apps") {
		t.Errorf("error = %v, want one that says the app must be debuggable", err)
	}
}
