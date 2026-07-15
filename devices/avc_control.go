package devices

import (
	"fmt"
	"strconv"
	"strings"
)

// avcControlSocket is the localabstract name AvcServer binds for its live
// encoder control channel. Must match devicekit AvcServer's CONTROL_SOCKET.
//
// The channel is a JSON-RPC socket in the capture process (shell uid), reached
// via `adb forward` + POST — the same transport as the webview agent. Stdin
// can't be used: AvcServer is launched with `adb exec-out`, which does not
// forward host stdin to the device process.
const avcControlSocket = "devicekit-avc"

// SetAvcBitrate changes the bitrate of an in-flight Android AVC capture without
// restarting the stream. Returns an error for non-Android devices (iOS uses
// MJPEG, which has no live control channel).
func SetAvcBitrate(device ControllableDevice, bitrate int) error {
	android, ok := device.(*AndroidDevice)
	if !ok {
		return fmt.Errorf("live bitrate control is only supported for Android AVC captures")
	}
	port, err := android.ensureControlForward()
	if err != nil {
		return err
	}
	if _, err := agentRequest(port, "screencapture.setBitrate", map[string]any{"bps": bitrate}); err != nil {
		return fmt.Errorf("set AVC bitrate: %w", err)
	}
	return nil
}

// RequestAvcKeyFrame asks the in-flight Android AVC encoder for an immediate sync
// frame (e.g. in response to a viewer PLI).
func RequestAvcKeyFrame(device ControllableDevice) error {
	android, ok := device.(*AndroidDevice)
	if !ok {
		return fmt.Errorf("keyframe request is only supported for Android AVC captures")
	}
	port, err := android.ensureControlForward()
	if err != nil {
		return err
	}
	if _, err := agentRequest(port, "screencapture.requestKeyFrame", nil); err != nil {
		return fmt.Errorf("request AVC keyframe: %w", err)
	}
	return nil
}

// ensureControlForward returns a host TCP port forwarded to the AvcServer control
// socket, reusing an existing forward when present so the ~2/sec bitrate updates
// don't churn adb forwards.
func (d *AndroidDevice) ensureControlForward() (int, error) {
	target := "localabstract:" + avcControlSocket
	if port := d.findForward(target); port != 0 {
		return port, nil
	}
	out, err := d.runAdbCommand("forward", "tcp:0", target)
	if err != nil {
		return 0, fmt.Errorf("adb forward control socket: %s: %w", strings.TrimSpace(string(out)), err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("unexpected adb forward output %q: %w", strings.TrimSpace(string(out)), err)
	}
	return port, nil
}

// findForward returns the host TCP port of an existing adb forward for this
// device to target (e.g. "localabstract:devicekit-avc"), or 0 if none.
func (d *AndroidDevice) findForward(target string) int {
	out, err := d.runAdbCommand("forward", "--list")
	if err != nil {
		return 0
	}
	serial := d.getAdbIdentifier()
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		// `adb forward --list` prints: "<serial> <local> <remote>"
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != serial || fields[2] != target {
			continue
		}
		if port, err := strconv.Atoi(strings.TrimPrefix(fields[1], "tcp:")); err == nil && port > 0 {
			return port
		}
	}
	return 0
}
