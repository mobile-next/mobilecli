package devices

import (
	"fmt"
	"strings"

	"github.com/mobile-next/mobilecli/devices/devicekit"
)

// springboardBundleID is the home screen. A real device lists it among its
// installed apps as "SpringBoard"; a simulator does not list it, and the agent names
// it with a single space.
const springboardBundleID = "com.apple.springboard"

// deviceKitForegroundApp asks the DeviceKit agent which app is in the foreground and enriches the
// answer with version details from the device's installed-app list. Shared by
// the physical-device and simulator implementations, which both talk to the DeviceKit agent.
func deviceKitForegroundApp(client *devicekit.DeviceKitClient, listApps func(onlyLaunchable bool) ([]InstalledAppInfo, error)) (*ForegroundAppInfo, error) {
	// get active app info from the agent
	activeApp, err := client.GetActiveAppInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get active app info: %w", err)
	}

	// get all installed apps to enrich with version information
	apps, err := listApps(true)
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	return foregroundAppFromAgent(activeApp, apps), nil
}

// foregroundAppFromAgent describes the app the agent reports in the foreground, using
// the installed-app entry when there is one.
func foregroundAppFromAgent(activeApp *devicekit.ActiveAppInfo, apps []InstalledAppInfo) *ForegroundAppInfo {
	for _, app := range apps {
		if app.PackageName == activeApp.BundleID {
			return &ForegroundAppInfo{
				PackageName: app.PackageName,
				AppName:     app.AppName,
				Version:     app.Version,
				Activity:    activeApp.ViewController,
			}
		}
	}

	// not in the list (e.g., a system app): describe it from the agent alone
	return &ForegroundAppInfo{
		PackageName: activeApp.BundleID,
		AppName:     agentAppName(activeApp),
		Version:     "",
		Activity:    activeApp.ViewController,
	}
}

// agentAppName is the name the agent reports for an app, or, when that name is blank,
// the name a real device lists for the home screen and the bundle ID for any
// other app, as the Android foreground app does for an app without a label.
func agentAppName(activeApp *devicekit.ActiveAppInfo) string {
	name := strings.TrimSpace(activeApp.Name)
	if name != "" {
		return name
	}
	if activeApp.BundleID == springboardBundleID {
		return "SpringBoard"
	}
	return activeApp.BundleID
}
