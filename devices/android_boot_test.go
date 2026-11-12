package devices

import (
	"slices"
	"testing"
)

func TestEmulatorBootsHiddenByDefault(t *testing.T) {
	args := emulatorBootArgs("Pixel_9", BootOptions{})

	if !slices.Contains(args, "-qt-hide-window") {
		t.Fatalf("expected a hidden window by default, got %v", args)
	}
}

func TestEmulatorBootsWithWindowWhenHeaded(t *testing.T) {
	args := emulatorBootArgs("Pixel_9", BootOptions{Headed: true})

	if slices.Contains(args, "-qt-hide-window") {
		t.Fatalf("expected a visible window when headed, got %v", args)
	}
}

func TestEmulatorBootTargetsTheRequestedAvd(t *testing.T) {
	for _, opts := range []BootOptions{{}, {Headed: true}} {
		args := emulatorBootArgs("Pixel_9", opts)

		avdFlag := slices.Index(args, "-avd")
		if avdFlag == -1 || avdFlag+1 >= len(args) || args[avdFlag+1] != "Pixel_9" {
			t.Fatalf("expected -avd Pixel_9, got %v", args)
		}
	}
}
