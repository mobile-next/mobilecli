package devices

import "testing"

// Before StartAgent sets up this device's own forwarder there must be no
// usable agent client: calls fail instead of reaching another device's
// agent on localhost:8100.
func TestIOSDeviceWithoutOwnForwarderFailsSafe(t *testing.T) {
	d := &IOSDevice{Udid: "test-device"}
	if c := d.ownAgentClient(); c != nil {
		t.Fatalf("ownAgentClient = %v, want nil", c)
	}
	if err := d.Tap(1, 2); err == nil {
		t.Fatal("Tap succeeded without own agent forwarder")
	}
	if _, err := d.TakeScreenshot(ScreenshotOptions{}); err == nil {
		t.Fatal("TakeScreenshot succeeded without own agent forwarder")
	}
	if _, err := d.Info(); err == nil {
		t.Fatal("Info succeeded without own agent forwarder")
	}
	if err := d.StartScreenCapture(ScreenCaptureConfig{Format: FormatMJPEG}); err == nil {
		t.Fatal("StartScreenCapture succeeded without own agent forwarder")
	}
}
