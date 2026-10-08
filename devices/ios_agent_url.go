package devices

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	goios "github.com/danielpaulus/go-ios/ios"
	"github.com/mobile-next/mobilecli/devices/devicekit"
)

// iosAgentURLEnv points real iOS devices at a DeviceKit agent that is already
// running and reachable at that URL (e.g. "http://localhost:8100", or the Xcode
// CoreDevice tunnel address of a Wi-Fi paired device such as
// "http://[fd34:13d9:5468::1]:12004"). When set, mobilecli neither launches the
// agent nor starts a tunnel or port forward to reach it. Calls that do not go
// through the agent (apps, logs, crashes) are unchanged.
//
// The value is either a comma-separated list of <udid>=<url> pairs, where each
// device uses only its own entry and a device without one takes the normal
// tunnel path, or a bare URL, which is accepted only while exactly one real iOS
// device is connected. The agent cannot confirm which device it runs on
// (device.info returns only the screen size and scale), so the device is bound
// to the agent by this mapping alone.
const iosAgentURLEnv = "MOBILECLI_IOS_AGENT_URL"

// countConnectedIOSDevices returns how many distinct real iOS devices usbmuxd
// reports. It only asks usbmuxd for its list, without opening a lockdown
// session per device as ListIOSDevices does. Tests replace it.
var countConnectedIOSDevices = func() (int, error) {
	deviceList, err := goios.ListDevices()
	if err != nil {
		return 0, fmt.Errorf("failed getting device list: %w", err)
	}

	// go-ios returns one entry per connection (usb, network), count by udid
	seen := make(map[string]bool)
	for _, deviceEntry := range deviceList.DeviceList {
		seen[deviceEntry.Properties.SerialNumber] = true
	}
	return len(seen), nil
}

// iosAgentURLOverride returns the agent URL that iosAgentURLEnv sets for the
// device with this udid, or "" when the device should take the normal path.
func iosAgentURLOverride(udid string) (string, error) {
	value := strings.TrimSpace(os.Getenv(iosAgentURLEnv))
	if value == "" {
		return "", nil
	}

	// agent URLs cannot carry a query, so '=' and ',' only appear in a mapping
	if !strings.ContainsAny(value, "=,") {
		return bareAgentURL(value)
	}

	agentURLs, err := parseAgentURLMapping(value)
	if err != nil {
		return "", err
	}
	return agentURLs[strings.ToUpper(udid)], nil
}

// bareAgentURL accepts a URL without a udid only when it cannot reach the
// wrong device, that is with a single real iOS device connected.
func bareAgentURL(value string) (string, error) {
	agentURL, err := parseAgentURL(value)
	if err != nil {
		return "", err
	}

	count, err := countConnectedIOSDevices()
	if err != nil {
		return "", err
	}
	if count > 1 {
		return "", fmt.Errorf("%s holds a single URL but %d iOS devices are connected, use <udid>=<url> to say which device runs the agent", iosAgentURLEnv, count)
	}

	return agentURL, nil
}

// parseAgentURLMapping parses "<udid>=<url>,<udid>=<url>" into URLs keyed by
// upper-case udid.
func parseAgentURLMapping(value string) (map[string]string, error) {
	agentURLs := make(map[string]string)
	for _, entry := range strings.Split(value, ",") {
		udid, rawURL, found := strings.Cut(strings.TrimSpace(entry), "=")
		udid = strings.TrimSpace(udid)
		rawURL = strings.TrimSpace(rawURL)
		if !found || udid == "" || rawURL == "" || strings.ContainsAny(udid, ":/") {
			return nil, fmt.Errorf("%s has a malformed entry %q, expected <udid>=<url> such as 00008101-00161CEC3CDB001E=http://localhost:8100 (agent URLs cannot have a query)", iosAgentURLEnv, entry)
		}

		key := strings.ToUpper(udid)
		if _, duplicate := agentURLs[key]; duplicate {
			return nil, fmt.Errorf("%s lists device %s more than once", iosAgentURLEnv, udid)
		}

		agentURL, err := parseAgentURL(rawURL)
		if err != nil {
			return nil, err
		}
		agentURLs[key] = agentURL
	}
	return agentURLs, nil
}

// parseAgentURL checks that value can serve as the base of agent requests,
// which append /health, /rpc and /mjpeg to it.
func parseAgentURL(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("%s=%q is not a valid agent URL, expected something like http://localhost:8100", iosAgentURLEnv, value)
	}
	if parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || strings.Contains(value, "#") {
		return "", fmt.Errorf("%s=%q is not a valid agent URL, it cannot have a query or fragment", iosAgentURLEnv, value)
	}
	// the agent has no authentication, so credentials would only travel in the clear
	if parsed.User != nil {
		return "", fmt.Errorf("%s=%q is not a valid agent URL, it cannot carry a user name or password", iosAgentURLEnv, value)
	}

	return strings.TrimSuffix(value, "/"), nil
}

// useExternalAgent switches the device to the agent at agentURL and checks that
// it answers. It never launches the agent or sets up a tunnel or forward.
func (d *IOSDevice) useExternalAgent(agentURL string) error {
	client := devicekit.NewDeviceKitClient(agentURL)
	if _, err := client.GetStatus(); err != nil {
		return fmt.Errorf("no agent answering at %s (from %s): %w", agentURL, iosAgentURLEnv, err)
	}

	d.mu.Lock()
	d.deviceKitClient = client
	d.mu.Unlock()

	return nil
}
