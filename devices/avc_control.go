package devices

import (
	"fmt"
	"io"
	"sync"
)

// avcControls maps a device ID to the stdin writer of its running AvcServer
// process, so a separate JSON-RPC call (device.screencapture.setConfiguration)
// can push live encoder commands into an in-flight capture without restarting it.
//
// ponytail: one writer per device — a device streams at most one AVC capture at
// a time. Revisit if concurrent captures per device ever become a thing.
var (
	avcControlsMu sync.Mutex
	avcControls   = map[string]io.Writer{}
)

func registerAvcControl(deviceID string, w io.Writer) {
	avcControlsMu.Lock()
	defer avcControlsMu.Unlock()
	avcControls[deviceID] = w
}

func unregisterAvcControl(deviceID string) {
	avcControlsMu.Lock()
	defer avcControlsMu.Unlock()
	delete(avcControls, deviceID)
}

// SetAvcBitrate pushes a live bitrate command to the running AvcServer for the
// given device. Returns an error if no AVC capture is currently active.
func SetAvcBitrate(deviceID string, bitrate int) error {
	avcControlsMu.Lock()
	w := avcControls[deviceID]
	avcControlsMu.Unlock()

	if w == nil {
		return fmt.Errorf("no active AVC capture for device %s", deviceID)
	}

	// AvcServer reads newline-delimited commands from stdin.
	if _, err := fmt.Fprintf(w, "bitrate %d\n", bitrate); err != nil {
		return fmt.Errorf("failed to send bitrate command: %w", err)
	}
	return nil
}
