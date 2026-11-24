package commands

import (
	"fmt"
	"sync"
)

// nativeRecorders tracks devices whose native recorder (adb screenrecord,
// simctl recordVideo) is in use. Unlike the shared H.264 stream, these
// recorders cannot serve two recordings of one device at a time.
var nativeRecorders = struct {
	mu      sync.Mutex
	devices map[string]bool
}{devices: map[string]bool{}}

// claimNativeRecorder reserves deviceID's native recorder. The returned
// release must be called once the recording has ended.
func claimNativeRecorder(deviceID string) (func(), error) {
	nativeRecorders.mu.Lock()
	defer nativeRecorders.mu.Unlock()

	if nativeRecorders.devices[deviceID] {
		return nil, fmt.Errorf("a screen recording is already in progress on device %s, and this device supports one recording at a time", deviceID)
	}

	nativeRecorders.devices[deviceID] = true
	release := func() {
		nativeRecorders.mu.Lock()
		defer nativeRecorders.mu.Unlock()
		delete(nativeRecorders.devices, deviceID)
	}
	return release, nil
}
