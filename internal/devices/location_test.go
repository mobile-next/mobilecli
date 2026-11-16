package devices

import "testing"

// Location override is opt-in per device type: the command layer decides
// whether to fake a location or report it unsupported purely from this type
// assertion, so this test pins down which device types satisfy it.
func TestLocationSettableImplementers(t *testing.T) {
	var android any = (*AndroidDevice)(nil)
	if _, ok := android.(LocationSettable); !ok {
		t.Error("AndroidDevice should implement LocationSettable")
	}

	var ios any = (*IOSDevice)(nil)
	if _, ok := ios.(LocationSettable); !ok {
		t.Error("IOSDevice should implement LocationSettable")
	}

	var simulator any = &SimulatorDevice{}
	if _, ok := simulator.(LocationSettable); !ok {
		t.Error("SimulatorDevice should implement LocationSettable")
	}

	var remote any = (*RemoteDevice)(nil)
	if _, ok := remote.(LocationSettable); ok {
		t.Error("RemoteDevice should not implement LocationSettable (unsupported expected)")
	}
}
