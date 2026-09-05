package devices

import (
	"fmt"
	"strconv"
	"strings"
)

// avcControlSocket is the localabstract name AvcServer binds for its live
// encoder control channel. Must match CONTROL_SOCKET in agents/android/java/AvcServer.java.
//
// The channel is a JSON-RPC socket in the capture process (shell uid), reached
// via `adb forward` + POST — the same transport as the webview agent. Stdin
// can't be used: AvcServer is launched with `adb exec-out`, which does not
// forward host stdin to the device process.
const avcControlSocket = "mobilecli-avc"

// SetAvcBitrate changes the bitrate of an in-flight AVC capture without
// restarting the stream. Both platforms accept the same payload
// ("screencapture.setBitrate" with a "bps" param); only the transport differs —
// Android over the localabstract control socket, iOS over the live stream conn.
func SetAvcBitrate(device ControllableDevice, bitrate int) error {
	switch dev := device.(type) {
	case *AndroidDevice:
		port, err := dev.ensureControlForward()
		if err != nil {
			return err
		}
		if _, err := agentRequest(port, "screencapture.setBitrate", map[string]any{"bps": bitrate}); err != nil {
			return fmt.Errorf("set AVC bitrate: %w", err)
		}
		return nil
	case *IOSDevice:
		return dev.sendAvcControl("screencapture.setBitrate", map[string]any{"bps": bitrate})
	default:
		return fmt.Errorf("live bitrate control is not supported for this device type")
	}
}

// RequestAvcKeyFrame asks the in-flight AVC encoder for an immediate sync
// frame (e.g. in response to a viewer PLI).
func RequestAvcKeyFrame(device ControllableDevice) error {
	switch dev := device.(type) {
	case *AndroidDevice:
		port, err := dev.ensureControlForward()
		if err != nil {
			return err
		}
		if _, err := agentRequest(port, "screencapture.requestKeyFrame", nil); err != nil {
			return fmt.Errorf("request AVC keyframe: %w", err)
		}
		return nil
	case *IOSDevice:
		return dev.sendAvcControl("screencapture.requestKeyFrame", nil)
	default:
		return fmt.Errorf("keyframe request is not supported for this device type")
	}
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
// device to target (e.g. "localabstract:mobilecli-avc"), or 0 if none.
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
