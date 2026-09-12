package devices

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices/devicekit"
)

// wdaForegroundApp asks WDA which app is in the foreground and enriches the
// answer with version details from the device's installed-app list. Shared by
// the physical-device and simulator implementations, which both talk to WDA.
func wdaForegroundApp(client *devicekit.DeviceKitClient, listApps func(onlyLaunchable bool) ([]InstalledAppInfo, error)) (*ForegroundAppInfo, error) {
	// get active app info from WDA
	activeApp, err := client.GetActiveAppInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get active app info: %w", err)
	}

	// get all installed apps to enrich with version information
	apps, err := listApps(true)
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	// find the matching app to get full details
	for _, app := range apps {
		if app.PackageName == activeApp.BundleID {
			return &ForegroundAppInfo{
				PackageName: app.PackageName,
				AppName:     app.AppName,
				Version:     app.Version,
				Activity:    activeApp.ViewController,
			}, nil
		}
	}

	// if app not found in list (e.g., system app), return info from WDA only
	return &ForegroundAppInfo{
		PackageName: activeApp.BundleID,
		AppName:     activeApp.Name,
		Version:     "",
		Activity:    activeApp.ViewController,
	}, nil
}
