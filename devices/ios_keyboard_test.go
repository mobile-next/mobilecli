package devices

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mobile-next/mobilecli/devices/devicekit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deviceKitReportingNoKeyboard is a DeviceKit agent that reports no keyboard. Any
// attempt to inject the in-app agent would fail, because no such app exists.
func deviceKitReportingNoKeyboard(t *testing.T) *devicekit.DeviceKitClient {
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"visible":false}}`))
	}))
	t.Cleanup(agent.Close)
	return devicekit.NewDeviceKitClient(agent.URL)
}

func simulatorWithNoKeyboardShowing(t *testing.T) *SimulatorDevice {
	return &SimulatorDevice{deviceKitClient: deviceKitReportingNoKeyboard(t)}
}

func realIPhoneWithNoKeyboardShowing(t *testing.T) *IOSDevice {
	return &IOSDevice{deviceKitClient: deviceKitReportingNoKeyboard(t)}
}

func TestRealIPhoneHideWithNoKeyboardShowingDoesNothing(t *testing.T) {
	dismissed, err := realIPhoneWithNoKeyboardShowing(t).HideKeyboard()

	require.NoError(t, err)
	assert.False(t, dismissed)
}

func TestSimulatorHideWithNoKeyboardShowingDoesNothing(t *testing.T) {
	dismissed, err := simulatorWithNoKeyboardShowing(t).HideKeyboard()

	require.NoError(t, err)
	assert.False(t, dismissed)
}
