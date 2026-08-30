package devices

import (
	"fmt"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/agents"
	"github.com/mobile-next/mobilecli/utils"
)

const (
	// mockLocationClass is the embedded agent that holds the test providers alive
	mockLocationClass = "com.mobilenext.mobilecli.MockLocation"
	mockLocationLog   = "/data/local/tmp/mobilecli-mocklocation.log"

	// shellPackage is the package app_process is attributed to, and therefore the
	// one the mock_location appop has to be granted to
	shellPackage = "com.android.shell"

	mockLocationStartTimeout = 10 * time.Second
)

// emulatorDefaultLatitude/Longitude are the coordinates an emulator hands its
// emulated gps at launch (the Googleplex). The console has no counterpart to
// `geo fix`, and an emulator has no real location to go back to, so clearing an
// override means putting those back.
const (
	emulatorDefaultLatitude  = 37.421998
	emulatorDefaultLongitude = -122.084000
)

// SetLocation overrides the device location. Emulators go through the emulator
// console; real devices need an on-device agent holding a test provider open.
func (d *AndroidDevice) SetLocation(lat, lon float64) error {
	if d.DeviceType() == "emulator" {
		// note: the emulator console takes longitude first
		out, err := d.runAdbCommand("emu", "geo", "fix", fmt.Sprintf("%f", lon), fmt.Sprintf("%f", lat))
		if err != nil {
			return fmt.Errorf("emu geo fix: %s: %w", strings.TrimSpace(string(out)), err)
		}
		if strings.Contains(string(out), "KO") {
			return fmt.Errorf("emu geo fix: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}

	return d.startMockLocation(lat, lon)
}

// ClearLocation restores the location the device reports on its own, which on
// an emulator means the default it booted with.
func (d *AndroidDevice) ClearLocation() error {
	if d.DeviceType() == "emulator" {
		return d.SetLocation(emulatorDefaultLatitude, emulatorDefaultLongitude)
	}

	d.stopMockLocation()

	if out, err := d.runDexClass(mockLocationClass, "--clear"); err != nil {
		return fmt.Errorf("remove test providers: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// revoke the appop we granted in startMockLocation
	if out, err := d.runAdbCommand("shell", "appops", "set", shellPackage, "android:mock_location", "default"); err != nil {
		return fmt.Errorf("revoke mock_location appop: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}

// startMockLocation launches the MockLocation agent detached, so the test
// providers it registers outlive this mobilecli invocation.
func (d *AndroidDevice) startMockLocation(lat, lon float64) error {
	if out, err := d.runAdbCommand("shell", "appops", "set", shellPackage, "android:mock_location", "allow"); err != nil {
		return fmt.Errorf("grant mock_location appop: %s: %w", strings.TrimSpace(string(out)), err)
	}

	if err := d.pushTempFile(agents.AndroidMobilecliDEX, androidDexPath); err != nil {
		return fmt.Errorf("push .dex: %w", err)
	}

	d.stopMockLocation()

	launchCmd := fmt.Sprintf("rm -f %s; CLASSPATH=%s nohup app_process / %s %f %f >%s 2>&1 &",
		mockLocationLog, androidDexPath, mockLocationClass, lat, lon, mockLocationLog)
	if out, err := d.runAdbCommand("shell", launchCmd); err != nil {
		return fmt.Errorf("launch mock location agent: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return d.waitForMockLocation()
}

// waitForMockLocation blocks until the agent reports it registered a test
// provider, and surfaces its log when it died on startup instead.
func (d *AndroidDevice) waitForMockLocation() error {
	deadline := time.Now().Add(mockLocationStartTimeout)
	var log string

	for time.Now().Before(deadline) {
		out, _ := d.runAdbCommand("shell", "cat", mockLocationLog)
		log = strings.TrimSpace(string(out))
		if strings.HasPrefix(log, "ok") {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	if log == "" {
		log = "no output from the agent"
	}
	return fmt.Errorf("mock location agent did not start: %s", log)
}

// stopMockLocation kills a running agent, if any
func (d *AndroidDevice) stopMockLocation() {
	if out, err := d.runAdbCommand("shell", "pkill", "-f", mockLocationClass); err != nil {
		// pkill exits non-zero when nothing matched, which is the common case
		utils.Verbose("pkill mock location agent: %s: %v", strings.TrimSpace(string(out)), err)
	}
}
