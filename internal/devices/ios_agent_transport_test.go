package devices

import (
	"context"
	"net"
	"testing"

	goios "github.com/danielpaulus/go-ios/ios"
	"github.com/mobile-next/mobilecli/internal/devices/ios"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runningForwarder returns a forwarder that reports running, backed by a local listener
// and a tunnel endpoint nobody dials in these tests.
func runningForwarder(t *testing.T) *ios.PortForwarder {
	t.Helper()
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr, ok := l.Addr().(*net.TCPAddr)
	require.True(t, ok)
	port := addr.Port
	require.NoError(t, l.Close())

	pf := ios.NewPortForwarder("udid-1")
	require.NoError(t, pf.ForwardOverTunnel(port, goios.DeviceEntry{UserspaceTUN: true, UserspaceTUNHost: "127.0.0.1"}, "fd00::1", 12004))
	t.Cleanup(func() {
		if pf.IsRunning() {
			_ = pf.Stop()
		}
	})
	return pf
}

func TestAgentTunnelAddressSkipsTunnelWhenNotNeeded(t *testing.T) {
	addr, err := (&IOSDevice{Udid: "udid-1", OSVersion: "16.7"}).agentTunnelAddress(false)
	require.NoError(t, err)
	assert.Empty(t, addr, "iOS below 17 has no tunnel")

	addr, err = (&IOSDevice{Udid: "udid-1", OSVersion: "18.6"}).agentTunnelAddress(true)
	require.NoError(t, err)
	assert.Empty(t, addr, "a USB-connected device keeps the usbmuxd path")
}

func TestStopAgentAbandonsRunnerAndStopsForwarder(t *testing.T) {
	canceled := false
	forwarder := runningForwarder(t)
	dev := &IOSDevice{
		portForwarderDeviceKitAgent: forwarder,
		deviceKitCancel:             func() { canceled = true },
		deviceKitDone:               make(chan struct{}),
	}

	dev.stopAgent()

	assert.True(t, canceled, "the runner context should be canceled")
	assert.Nil(t, dev.deviceKitCancel, "ownership should be cleared so the next launch goes ahead")
	assert.Nil(t, dev.deviceKitDone)
	assert.False(t, forwarder.IsRunning())
}

func TestResetAgentIfTransportChanged(t *testing.T) {
	t.Run("same address keeps the agent", func(t *testing.T) {
		canceled := false
		dev := &IOSDevice{agentTunnelAddr: "fd00::1", deviceKitCancel: func() { canceled = true }}

		require.NoError(t, dev.resetAgentIfTransportChanged("fd00::1", 0))
		assert.False(t, canceled)
		assert.NotNil(t, dev.deviceKitCancel)
	})

	t.Run("new address restarts the agent", func(t *testing.T) {
		canceled := false
		dev := &IOSDevice{agentTunnelAddr: "", deviceKitCancel: func() { canceled = true }}

		require.NoError(t, dev.resetAgentIfTransportChanged("fd00::1", 0))
		assert.True(t, canceled)
		assert.Nil(t, dev.deviceKitCancel)
	})

	t.Run("same address on a new tunnel port restarts the agent", func(t *testing.T) {
		canceled := false
		dev := &IOSDevice{agentTunnelAddr: "fd00::1", agentTunnelPort: 60106, deviceKitCancel: func() { canceled = true }}

		require.NoError(t, dev.resetAgentIfTransportChanged("fd00::1", 60107))
		assert.True(t, canceled)
	})

	t.Run("nothing running is a no-op", func(t *testing.T) {
		dev := &IOSDevice{agentTunnelAddr: "fd00::1"}
		require.NoError(t, dev.resetAgentIfTransportChanged("", 0))
	})
}

func TestResetIfConnectionChanged(t *testing.T) {
	t.Run("first start only records the connection", func(t *testing.T) {
		dev := &IOSDevice{Udid: "udid-1", OSVersion: "18.6"}

		require.NoError(t, dev.resetIfConnectionChanged(true))
		require.NotNil(t, dev.agentOverUSB)
		assert.True(t, *dev.agentOverUSB)
	})

	t.Run("same connection keeps the agent", func(t *testing.T) {
		usb := true
		canceled := false
		dev := &IOSDevice{Udid: "udid-1", OSVersion: "18.6", agentOverUSB: &usb, deviceKitCancel: func() { canceled = true }}

		require.NoError(t, dev.resetIfConnectionChanged(true))
		assert.False(t, canceled)
	})

	t.Run("moving to network rebuilds the tunnel and restarts the agent", func(t *testing.T) {
		t.Setenv("TMPDIR", t.TempDir()) // own pair-record dir, other packages' tests create theirs in parallel
		tm, err := ios.NewTunnelManager("udid-1")
		require.NoError(t, err)
		before := tm.GetTunnelManager()

		usb := true
		canceled := false
		dev := &IOSDevice{
			Udid:            "udid-1",
			OSVersion:       "18.6",
			tunnelManager:   tm,
			agentOverUSB:    &usb,
			deviceKitCancel: func() { canceled = true },
		}

		require.NoError(t, dev.resetIfConnectionChanged(false))
		assert.True(t, canceled)
		assert.Nil(t, dev.deviceKitCancel)
		assert.NotSame(t, before, tm.GetTunnelManager(), "the tunnel built over USB should be dropped")
		require.NotNil(t, dev.agentOverUSB)
		assert.False(t, *dev.agentOverUSB)
	})

	t.Run("devices without a tunnel are left alone", func(t *testing.T) {
		usb := true
		canceled := false
		dev := &IOSDevice{Udid: "udid-1", OSVersion: "16.7", agentOverUSB: &usb, deviceKitCancel: func() { canceled = true }}

		require.NoError(t, dev.resetIfConnectionChanged(false))
		assert.False(t, canceled)
	})
}

func TestAbandonFailedStart(t *testing.T) {
	t.Run("over usb only abandons the runner", func(t *testing.T) {
		canceled := false
		dev := &IOSDevice{Udid: "udid-1", OSVersion: "18.6", deviceKitCancel: func() { canceled = true }}

		dev.abandonFailedStart()
		assert.True(t, canceled)
		assert.Nil(t, dev.deviceKitCancel, "a later start must not take the dead runner for a running one")
	})

	t.Run("over a tunnel also rebuilds the tunnel and forgets its address", func(t *testing.T) {
		t.Setenv("TMPDIR", t.TempDir())
		tm, err := ios.NewTunnelManager("udid-1")
		require.NoError(t, err)
		before := tm.GetTunnelManager()

		canceled := false
		dev := &IOSDevice{
			Udid:            "udid-1",
			OSVersion:       "18.6",
			tunnelManager:   tm,
			agentTunnelAddr: "fd00::1",
			deviceKitCancel: func() { canceled = true },
		}

		dev.abandonFailedStart()
		assert.True(t, canceled)
		assert.Nil(t, dev.deviceKitCancel)
		assert.Empty(t, dev.agentTunnelAddr)
		assert.NotSame(t, before, tm.GetTunnelManager())
	})
}

func TestUserspaceTunnelPortForMissingTunnel(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	tm, err := ios.NewTunnelManager("udid-1")
	require.NoError(t, err)
	dev := &IOSDevice{Udid: "udid-1", OSVersion: "18.6", tunnelManager: tm}

	// go-ios reports an unknown device's tunnel as a zero value without an error
	assert.Equal(t, 0, dev.userspaceTunnelPort("fd00::1"))
	assert.Equal(t, 0, dev.userspaceTunnelPort(""))
}
