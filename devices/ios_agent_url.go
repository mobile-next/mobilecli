package devices

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/mobile-next/mobilecli/devices/devicekit"
)

// iosAgentURLEnv points real iOS devices at a DeviceKit agent that is already
// running and reachable at that URL (e.g. "http://localhost:8100", or the Xcode
// CoreDevice tunnel address of a Wi-Fi paired device such as
// "http://[fd34:13d9:5468::1]:12004"). When set,
// mobilecli neither launches the agent nor starts a tunnel or port forward to
// reach it. Calls that do not go through the agent (apps, logs, crashes) are
// unchanged.
const iosAgentURLEnv = "MOBILECLI_IOS_AGENT_URL"

// iosAgentURLOverride returns the agent URL from iosAgentURLEnv, or "" when it
// is not set.
func iosAgentURLOverride() (string, error) {
	value := strings.TrimSpace(os.Getenv(iosAgentURLEnv))
	if value == "" {
		return "", nil
	}

	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("%s=%q is not a valid agent URL, expected something like http://localhost:8100", iosAgentURLEnv, value)
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
