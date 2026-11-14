package devices

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEmulatorHost is an emulatorProbe over a scripted set of running
// emulators, and counts the getprop calls refreshState makes.
type fakeEmulatorHost struct {
	running      []adbTransport
	avdBySerial  map[string]string
	offlineAVDs  map[string]*AndroidDevice
	adbDown      bool
	getpropCalls int
}

func newFakeEmulatorHost() *fakeEmulatorHost {
	return &fakeEmulatorHost{avdBySerial: map[string]string{}, offlineAVDs: map[string]*AndroidDevice{}}
}

func (h *fakeEmulatorHost) runEmulator(avdName, serial, connection string) {
	h.running = append(h.running, adbTransport{serial: serial, status: "device", connection: connection})
	h.avdBySerial[serial] = avdName
}

func (h *fakeEmulatorHost) killAllEmulators() {
	h.running = nil
	h.avdBySerial = map[string]string{}
}

func (h *fakeEmulatorHost) haveAVD(avdName, displayName, version string) {
	h.offlineAVDs[avdName] = &AndroidDevice{id: avdName, name: displayName, version: version, state: StateOffline}
}

func (h *fakeEmulatorHost) transports() ([]adbTransport, error) {
	if h.adbDown {
		return nil, errors.New("adb server not running")
	}
	return h.running, nil
}

func (h *fakeEmulatorHost) getprop(serial, property string) (string, error) {
	h.getpropCalls++
	avdName, ok := h.avdBySerial[serial]
	if !ok {
		return "", errors.New("device not found")
	}
	switch property {
	case "ro.boot.qemu.avd_name":
		return avdName, nil
	case "ro.product.model":
		return "sdk_gphone64_arm64", nil
	case "ro.build.version.release":
		return "16", nil
	}
	return "", nil
}

func (h *fakeEmulatorHost) offlineEmulator(avdName string) *AndroidDevice {
	return h.offlineAVDs[avdName]
}

func offlineEmulatorDevice(avdName string) *AndroidDevice {
	return &AndroidDevice{id: avdName, name: avdName, version: "16.0", state: StateOffline}
}

func onlineEmulatorDevice(avdName, serial string) *AndroidDevice {
	return &AndroidDevice{id: avdName, name: avdName, version: "16", model: "sdk_gphone64_arm64", state: StateOnline, transportID: serial}
}

func TestParseAdbDevicesLongReadsSerialStatusAndConnection(t *testing.T) {
	output := "List of devices attached\n" +
		"0B101FDD4001HU         device usb:1-1 product:redfin model:Pixel_5 device:redfin transport_id:7\n" +
		"emulator-5554          offline transport_id:9\n"

	assert.Equal(t, []adbTransport{
		{serial: "0B101FDD4001HU", status: "device", connection: "7"},
		{serial: "emulator-5554", status: "offline", connection: "9"},
	}, parseAdbDevicesLong(output))
}

func TestEmulatorStartedOutsideMobilecliBecomesOnline(t *testing.T) {
	host := newFakeEmulatorHost()
	device := offlineEmulatorDevice("my_avd")
	host.runEmulator("other_avd", "emulator-5554", "3")
	host.runEmulator("my_avd", "emulator-5640", "8")

	require.NoError(t, device.refreshStateWith(host))

	assert.Equal(t, StateOnline, device.State())
	assert.Equal(t, "emulator-5640", device.getAdbIdentifier())
	assert.Equal(t, "16", device.Version())
	assert.Equal(t, "sdk_gphone64_arm64", device.Model())
	assert.Equal(t, DeviceTypeEmulator, device.DeviceType())
}

func TestEmulatorKilledOutsideMobilecliBecomesOfflineLikeTheAVDListing(t *testing.T) {
	host := newFakeEmulatorHost()
	host.haveAVD("my_avd", "Pixel 9 Pro", "16.0")
	device := onlineEmulatorDevice("my_avd", "emulator-5640")

	require.NoError(t, device.refreshStateWith(host))

	assert.Equal(t, StateOffline, device.State())
	assert.Equal(t, "my_avd", device.getAdbIdentifier())
	assert.Equal(t, "Pixel 9 Pro", device.Name())
	assert.Equal(t, "16.0", device.Version())
	assert.Equal(t, "", device.Model())
	assert.Equal(t, DeviceTypeEmulator, device.DeviceType())
}

func TestAnotherAVDOnTheSamePortDoesNotKeepTheEmulatorOnline(t *testing.T) {
	host := newFakeEmulatorHost()
	device := onlineEmulatorDevice("my_avd", "emulator-5554")
	host.runEmulator("other_avd", "emulator-5554", "12")

	require.NoError(t, device.refreshStateWith(host))

	assert.Equal(t, StateOffline, device.State())
}

func TestEmulatorRestartedOnAnotherPortIsFollowed(t *testing.T) {
	host := newFakeEmulatorHost()
	device := onlineEmulatorDevice("my_avd", "emulator-5554")
	host.runEmulator("my_avd", "emulator-5556", "4")

	require.NoError(t, device.refreshStateWith(host))

	assert.Equal(t, StateOnline, device.State())
	assert.Equal(t, "emulator-5556", device.getAdbIdentifier())
}

func TestUnchangedAdbConnectionNeedsNoGetprop(t *testing.T) {
	host := newFakeEmulatorHost()
	device := offlineEmulatorDevice("my_avd")
	host.runEmulator("my_avd", "emulator-5640", "8")
	require.NoError(t, device.refreshStateWith(host))
	callsToFindIt := host.getpropCalls

	require.NoError(t, device.refreshStateWith(host))

	assert.Equal(t, callsToFindIt, host.getpropCalls, "a known, unchanged connection must be trusted without asking the device")
	assert.Equal(t, StateOnline, device.State())
}

func TestEmulatorRestartedOnTheSamePortIsRecognisedByItsNewConnection(t *testing.T) {
	host := newFakeEmulatorHost()
	device := offlineEmulatorDevice("my_avd")
	host.runEmulator("my_avd", "emulator-5640", "8")
	require.NoError(t, device.refreshStateWith(host))
	device.serverPort = 41000

	host.killAllEmulators()
	host.runEmulator("my_avd", "emulator-5640", "15")
	require.NoError(t, device.refreshStateWith(host))

	assert.Equal(t, StateOnline, device.State())
	assert.Equal(t, 0, device.serverPort, "the device server of the previous emulator process is gone")
}

func TestUnreachableAdbKeepsTheLastKnownState(t *testing.T) {
	host := newFakeEmulatorHost()
	host.adbDown = true
	device := onlineEmulatorDevice("my_avd", "emulator-5640")

	assert.Error(t, device.refreshStateWith(host))

	assert.Equal(t, StateOnline, device.State())
	assert.Equal(t, "emulator-5640", device.getAdbIdentifier())
}

func TestRealDeviceStateIsNotRefreshed(t *testing.T) {
	host := newFakeEmulatorHost()
	device := &AndroidDevice{id: "0B101FDD4001HU", transportID: "0B101FDD4001HU", state: StateOnline}

	require.NoError(t, device.refreshStateWith(host))

	assert.Equal(t, StateOnline, device.State())
	assert.Equal(t, 0, host.getpropCalls)
}
