package devices

import (
	"context"
	"os"
	"strconv"
	"strings"
)

const defaultADBServerPort = 5037

// adbTransport is one device line of `adb devices -l`.
type adbTransport struct {
	serial     string
	status     string // "device" once usable; "offline", "unauthorized", ... otherwise
	connection string // adb's transport_id, new on every (re)connection
}

// emulatorProbe is what refreshState asks about the host's emulators, so the
// decision can be tested without adb.
type emulatorProbe interface {
	transports() ([]adbTransport, error)
	getprop(serial, property string) (string, error)
	// offlineEmulator returns the AVD as listed when it is not running, or nil
	// if no such AVD exists.
	offlineEmulator(avdName string) *AndroidDevice
}

// parseAdbDevicesLong parses `adb devices -l` output. The header line, if
// present, is skipped because it has no status field.
func parseAdbDevicesLong(output string) []adbTransport {
	var transports []adbTransport
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || line == "List of devices attached" {
			continue
		}
		t := adbTransport{serial: fields[0], status: fields[1]}
		for _, field := range fields[2:] {
			if id, ok := strings.CutPrefix(field, "transport_id:"); ok {
				t.connection = id
			}
		}
		transports = append(transports, t)
	}
	return transports
}

// adbServerProbe answers emulatorProbe through the local adb server.
type adbServerProbe struct {
	adb *ADB
}

func newADBServerProbe() adbServerProbe {
	port := defaultADBServerPort
	if p, err := strconv.Atoi(os.Getenv("ANDROID_ADB_SERVER_PORT")); err == nil && p > 0 {
		port = p
	}
	return adbServerProbe{adb: NewADB("127.0.0.1", port).WithTimeout(androidDiscoveryGetpropTimeout)}
}

func (p adbServerProbe) transports() ([]adbTransport, error) {
	output, err := p.adb.DevicesLong(context.Background())
	if err != nil {
		return nil, err
	}
	return parseAdbDevicesLong(output), nil
}

func (p adbServerProbe) getprop(serial, property string) (string, error) {
	output, err := p.adb.Shell(context.Background(), serial, "getprop "+property)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func (p adbServerProbe) offlineEmulator(avdName string) *AndroidDevice {
	avds, err := getOfflineAndroidEmulators(map[string]bool{})
	if err != nil {
		return nil
	}
	for _, avd := range avds {
		if avd.ID() == avdName {
			if device, ok := avd.(*AndroidDevice); ok {
				return device
			}
		}
	}
	return nil
}

func (d *AndroidDevice) refreshState() error {
	return d.refreshStateWith(newADBServerProbe())
}

// refreshStateWith re-reads whether this emulator is running, and on which adb
// serial, instead of trusting what was true when the device was first listed.
// It is started and stopped outside mobilecli too (Android Studio, `kill`, a
// crash), and may come back on a different port. Real devices are left alone:
// their serial is their identity.
func (d *AndroidDevice) refreshStateWith(probe emulatorProbe) error {
	if d.DeviceType() != DeviceTypeEmulator {
		return nil
	}

	transports, err := probe.transports()
	if err != nil {
		return err
	}

	d.liveMu.RLock()
	knownSerial, knownConnection := d.transportID, d.connection
	d.liveMu.RUnlock()

	for _, t := range runningEmulatorsKnownFirst(transports, knownSerial) {
		if t.serial == knownSerial && knownConnection != "" && t.connection == knownConnection {
			// same adb connection as last time, so the same emulator process
			return nil
		}

		avdName, err := probe.getprop(t.serial, "ro.boot.qemu.avd_name")
		if err != nil || avdName != d.id {
			continue
		}

		model, err := probe.getprop(t.serial, "ro.product.model")
		if err != nil {
			return err
		}
		version, err := probe.getprop(t.serial, "ro.build.version.release")
		if err != nil {
			return err
		}

		d.becomeOnline(t, getAndroidDeviceName(t.serial, avdName, model), model, version)
		return nil
	}

	d.becomeOffline(probe.offlineEmulator(d.id))
	return nil
}

// runningEmulatorsKnownFirst keeps the emulators adb can talk to, with the
// serial this device last had first, since it most likely still has it.
func runningEmulatorsKnownFirst(transports []adbTransport, knownSerial string) []adbTransport {
	var known, others []adbTransport
	for _, t := range transports {
		if t.status != "device" || !strings.HasPrefix(t.serial, "emulator-") {
			continue
		}
		if t.serial == knownSerial {
			known = append(known, t)
		} else {
			others = append(others, t)
		}
	}
	return append(known, others...)
}

func (d *AndroidDevice) becomeOnline(t adbTransport, name, model, version string) {
	d.liveMu.Lock()
	restarted := t.serial != d.transportID || (d.connection != "" && t.connection != d.connection)
	d.state = StateOnline
	d.transportID = t.serial
	d.connection = t.connection
	d.name = name
	d.model = model
	d.version = version
	d.liveMu.Unlock()

	if restarted {
		// the DeviceServer and its port forward belonged to the old process
		d.forgetDeviceServer()
	}
}

// becomeOffline describes the device as the offline AVD listing does, so
// `device info` and `devices` agree.
func (d *AndroidDevice) becomeOffline(avd *AndroidDevice) {
	d.liveMu.Lock()
	wasOnline := d.state != StateOffline
	d.state = StateOffline
	d.transportID = ""
	d.connection = ""
	d.model = ""
	if avd != nil {
		d.name = avd.Name()
		d.version = avd.Version()
	}
	d.liveMu.Unlock()

	if wasOnline {
		d.forgetDeviceServer()
	}
}
