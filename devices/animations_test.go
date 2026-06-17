package devices

import "testing"

// The animations setting is fire-and-forget and platform-specific: Android
// applies it via adb, while iOS/simulator are expected to no-op. Callers rely
// on the AnimationConfigurable type assertion to decide which path to take, so
// this test pins down which device types satisfy the capability.
func TestAnimationConfigurableImplementers(t *testing.T) {
	var android any = (*AndroidDevice)(nil)
	if _, ok := android.(AnimationConfigurable); !ok {
		t.Error("AndroidDevice should implement AnimationConfigurable")
	}

	var ios any = IOSDevice{}
	if _, ok := ios.(AnimationConfigurable); ok {
		t.Error("IOSDevice should not implement AnimationConfigurable (no-op expected)")
	}

	var simulator any = SimulatorDevice{}
	if _, ok := simulator.(AnimationConfigurable); ok {
		t.Error("SimulatorDevice should not implement AnimationConfigurable (no-op expected)")
	}
}
