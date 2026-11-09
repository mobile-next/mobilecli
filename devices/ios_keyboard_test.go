package devices

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mobile-next/mobilecli/devices/devicekit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// simulatorWithNoKeyboardShowing has a DeviceKit agent that reports no keyboard.
// Any attempt to inject the in-app agent would fail, because no such app exists.
func simulatorWithNoKeyboardShowing(t *testing.T) *SimulatorDevice {
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"visible":false}}`))
	}))
	t.Cleanup(agent.Close)
	return &SimulatorDevice{deviceKitClient: devicekit.NewDeviceKitClient(agent.URL)}
}

func TestRealIOSDevicesRefuseToHideTheKeyboard(t *testing.T) {
	_, err := (&IOSDevice{}).HideKeyboard()

	assert.ErrorIs(t, err, ErrHideKeyboardNotSupportedOnRealIOS)
}

func TestSimulatorHideWithNoKeyboardShowingDoesNothing(t *testing.T) {
	dismissed, err := simulatorWithNoKeyboardShowing(t).HideKeyboard()

	require.NoError(t, err)
	assert.False(t, dismissed)
}
