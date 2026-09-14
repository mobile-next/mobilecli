package devices

import "testing"

// Appearance is applied via adb on Android and via DeviceKit on iOS and the
// simulator, and forwarded over RPC for remote devices. Callers rely on the
// AppearanceConfigurable type assertion, so pin down who satisfies it.
func TestAppearanceConfigurableImplementers(t *testing.T) {
	implementers := map[string]any{
		"AndroidDevice":   (*AndroidDevice)(nil),
		"IOSDevice":       (*IOSDevice)(nil),
		"SimulatorDevice": SimulatorDevice{},
		"RemoteDevice":    (*RemoteDevice)(nil),
	}

	for name, device := range implementers {
		if _, ok := device.(AppearanceConfigurable); !ok {
			t.Errorf("%s should implement AppearanceConfigurable", name)
		}
	}
}
