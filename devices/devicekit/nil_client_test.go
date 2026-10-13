package devicekit

import "testing"

func TestNilClientFailsSafe(t *testing.T) {
	var c *DeviceKitClient
	if c.Port() != 0 {
		t.Fatal("nil client Port != 0")
	}
	if _, err := c.GetStatus(); err == nil {
		t.Fatal("nil client GetStatus succeeded")
	}
	if _, err := c.CallRPC("device.info", nil); err == nil {
		t.Fatal("nil client CallRPC succeeded")
	}
}
