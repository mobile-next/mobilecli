package devices

import (
	"testing"
)

const avcOnSimulatorError = "avc is not supported on iOS simulators, use mjpeg"

func expectFormatAccepted(t *testing.T, platform, deviceType, format string) {
	t.Helper()
	if err := ValidateScreenCaptureFormat(platform, deviceType, format); err != nil {
		t.Fatalf("%s %s %s: expected accepted, got %v", platform, deviceType, format, err)
	}
}

func expectFormatRejected(t *testing.T, platform, deviceType, format, wantErr string) {
	t.Helper()
	err := ValidateScreenCaptureFormat(platform, deviceType, format)
	if err == nil || err.Error() != wantErr {
		t.Fatalf("%s %s %s: expected %q, got %v", platform, deviceType, format, wantErr, err)
	}
}

func TestValidateScreenCaptureFormat_MjpegWorksEverywhere(t *testing.T) {
	expectFormatAccepted(t, PlatformIOS, DeviceTypeSimulator, FormatMJPEG)
	expectFormatAccepted(t, PlatformIOS, DeviceTypeReal, FormatMJPEG)
	expectFormatAccepted(t, PlatformAndroid, DeviceTypeEmulator, FormatMJPEG)
	expectFormatAccepted(t, PlatformAndroid, DeviceTypeReal, FormatMJPEG)
}

func TestValidateScreenCaptureFormat_AvcWorksOnAndroidAndRealIOS(t *testing.T) {
	expectFormatAccepted(t, PlatformIOS, DeviceTypeReal, FormatAVC)
	expectFormatAccepted(t, PlatformAndroid, DeviceTypeEmulator, FormatAVC)
	expectFormatAccepted(t, PlatformAndroid, DeviceTypeReal, FormatAVC)
}

func TestValidateScreenCaptureFormat_AvcOnIOSSimulatorIsRejected(t *testing.T) {
	expectFormatRejected(t, PlatformIOS, DeviceTypeSimulator, FormatAVC, avcOnSimulatorError)
}

func TestValidateScreenCaptureFormat_UnknownFormatIsRejected(t *testing.T) {
	expectFormatRejected(t, PlatformAndroid, DeviceTypeReal, "h265", "format must be 'mjpeg' or 'avc' for screen capture")
	expectFormatRejected(t, PlatformIOS, DeviceTypeSimulator, "", "format must be 'mjpeg' or 'avc' for screen capture")
}

func TestSimulatorStartScreenCapture_RejectsAvcInsteadOfStreamingMjpeg(t *testing.T) {
	simulator := &SimulatorDevice{}
	framesReceived := 0
	err := simulator.StartScreenCapture(ScreenCaptureConfig{
		Format: FormatAVC,
		OnData: func([]byte) bool { framesReceived++; return true },
	})
	if err == nil || err.Error() != avcOnSimulatorError {
		t.Fatalf("expected %q, got %v", avcOnSimulatorError, err)
	}
	if framesReceived != 0 {
		t.Fatalf("expected no frames, got %d", framesReceived)
	}
}
