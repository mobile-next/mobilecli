package commands

import (
	"testing"

	"github.com/mobile-next/mobilecli/devices"
)

func TestShakerImplementers(t *testing.T) {
	implementers := map[string]struct {
		device any
		shakes bool
	}{
		"AndroidDevice":   {(*devices.AndroidDevice)(nil), true},
		"SimulatorDevice": {devices.SimulatorDevice{}, true},
		"RemoteDevice":    {(*devices.RemoteDevice)(nil), true},
		"IOSDevice":       {(*devices.IOSDevice)(nil), false},
	}

	for name, tc := range implementers {
		if _, ok := tc.device.(shaker); ok != tc.shakes {
			t.Errorf("%s implements shaker = %v, want %v", name, ok, tc.shakes)
		}
	}
}
