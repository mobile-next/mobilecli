package devices

import (
	"strings"
	"testing"
)

func Test_LaunchApp_ActivityRejectedOnApple(t *testing.T) {
	opts := LaunchOptions{Activity: ".DebugActivity"}

	if err := (SimulatorDevice{}).LaunchApp("com.example.app", opts); err == nil {
		t.Fatal("SimulatorDevice.LaunchApp: expected error for activity, got nil")
	} else if !strings.Contains(err.Error(), "not supported on iOS") {
		t.Fatalf("SimulatorDevice.LaunchApp: unexpected error: %v", err)
	}

	if err := (IOSDevice{}).LaunchApp("com.example.app", opts); err == nil {
		t.Fatal("IOSDevice.LaunchApp: expected error for activity, got nil")
	} else if !strings.Contains(err.Error(), "not supported on iOS") {
		t.Fatalf("IOSDevice.LaunchApp: unexpected error: %v", err)
	}
}
