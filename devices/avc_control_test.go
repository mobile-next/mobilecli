package devices

import (
	"encoding/binary"
	"encoding/json"
	"net"
	"testing"
)

// readControlMessage reads one length-prefixed JSON-RPC message the way the
// broadcast extension's ScreenStreamer does: 4-byte big-endian length, then payload.
func readControlMessage(t *testing.T, conn net.Conn) map[string]any {
	t.Helper()

	header := make([]byte, 4)
	if _, err := conn.Read(header); err != nil {
		t.Fatalf("read length prefix: %v", err)
	}
	payload := make([]byte, binary.BigEndian.Uint32(header))
	if _, err := conn.Read(payload); err != nil {
		t.Fatalf("read payload: %v", err)
	}

	var msg map[string]any
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	return msg
}

func TestSetAvcBitrateSendsAndroidCompatiblePayloadOnStreamConn(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	dev := &IOSDevice{avcStreamConn: local}

	errCh := make(chan error, 1)
	go func() { errCh <- SetAvcBitrate(dev, 4_000_000) }()

	msg := readControlMessage(t, remote)
	if err := <-errCh; err != nil {
		t.Fatalf("SetAvcBitrate: %v", err)
	}

	if msg["method"] != "screencapture.setBitrate" {
		t.Errorf("method = %v, want screencapture.setBitrate", msg["method"])
	}
	params, ok := msg["params"].(map[string]any)
	if !ok {
		t.Fatalf("params missing or not an object: %v", msg["params"])
	}
	if params["bps"] != float64(4_000_000) {
		t.Errorf("params.bps = %v, want 4000000", params["bps"])
	}
}

func TestRequestAvcKeyFrameSendsMethodOnStreamConn(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = remote.Close() }()

	dev := &IOSDevice{avcStreamConn: local}

	errCh := make(chan error, 1)
	go func() { errCh <- RequestAvcKeyFrame(dev) }()

	msg := readControlMessage(t, remote)
	if err := <-errCh; err != nil {
		t.Fatalf("RequestAvcKeyFrame: %v", err)
	}

	if msg["method"] != "screencapture.requestKeyFrame" {
		t.Errorf("method = %v, want screencapture.requestKeyFrame", msg["method"])
	}
}

func TestSetAvcBitrateFailsWithoutActiveStream(t *testing.T) {
	if err := SetAvcBitrate(&IOSDevice{}, 4_000_000); err == nil {
		t.Fatal("expected error when no capture stream is active")
	}
}
