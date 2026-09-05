package devices

import (
	"fmt"
	"strings"

	"github.com/mobile-next/mobilecli/utils"
)

// shellPackage is the package the DeviceServer (app_process) is attributed
// to, and therefore the one the mock_location appop has to be granted to
const shellPackage = "com.android.shell"

// emulatorDefaultLatitude/Longitude are the coordinates an emulator hands its
// emulated gps at launch (the Googleplex). The console has no counterpart to
// `geo fix`, and an emulator has no real location to go back to, so clearing an
// override means putting those back.
const (
	emulatorDefaultLatitude  = 37.421998
	emulatorDefaultLongitude = -122.084000
)

// SetLocation overrides the device location. Emulators go through the emulator
// console; real devices need the on-device server holding a test provider open.
func (d *AndroidDevice) SetLocation(lat, lon float64) error {
	if d.DeviceType() == "emulator" {
		return d.setEmulatorLocation(lat, lon)
	}

	return d.startMockLocation(lat, lon)
}

// setEmulatorLocation sends a fix to the emulator console, which reports its
// own failures as a KO line rather than a non-zero exit code.
func (d *AndroidDevice) setEmulatorLocation(lat, lon float64) error {
	// note: the emulator console takes longitude first
	out, err := d.runAdbCommand("emu", "geo", "fix", fmt.Sprintf("%f", lon), fmt.Sprintf("%f", lat))
	if err != nil {
		return fmt.Errorf("emu geo fix: %s: %w", strings.TrimSpace(string(out)), err)
	}

	if strings.HasPrefix(strings.TrimSpace(string(out)), "KO") {
		return fmt.Errorf("emu geo fix: %s", strings.TrimSpace(string(out)))
	}

	return nil
}

// ClearLocation restores the location the device reports on its own, which on
// an emulator means the default it booted with.
func (d *AndroidDevice) ClearLocation() error {
	if d.DeviceType() == "emulator" {
		return d.SetLocation(emulatorDefaultLatitude, emulatorDefaultLongitude)
	}

	var removeErr error
	if _, err := d.serverRequest("device.location.clear", nil); err != nil {
		removeErr = fmt.Errorf("remove test providers: %w", err)
	}

	// the appop we granted in startMockLocation goes back either way, a failed
	// removal is no reason to leave it granted on the device
	if err := d.revokeMockLocationAppop(); err != nil && removeErr == nil {
		removeErr = err
	}

	return removeErr
}

// startMockLocation asks the DeviceServer to register test
// providers and keep publishing the location, so it outlives this mobilecli
// invocation. Nothing is left behind when it fails: the appop is revoked, so a
// failed call never leaves the device with a fake location.
func (d *AndroidDevice) startMockLocation(lat, lon float64) error {
	if out, err := d.runAdbCommand("shell", "appops", "set", shellPackage, "android:mock_location", "allow"); err != nil {
		return fmt.Errorf("grant mock_location appop: %s: %w", strings.TrimSpace(string(out)), err)
	}

	if _, err := d.serverRequest("device.location.set", map[string]any{"lat": lat, "lon": lon}); err != nil {
		if revokeErr := d.revokeMockLocationAppop(); revokeErr != nil {
			utils.Verbose("failed to revoke mock_location appop after a failed start: %v", revokeErr)
		}
		return fmt.Errorf("set mock location: %w", err)
	}

	return nil
}

// revokeMockLocationAppop puts the mock_location appop back the way it was
func (d *AndroidDevice) revokeMockLocationAppop() error {
	if out, err := d.runAdbCommand("shell", "appops", "set", shellPackage, "android:mock_location", "default"); err != nil {
		return fmt.Errorf("revoke mock_location appop: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}
