package devices

import (
	"fmt"

	goios "github.com/danielpaulus/go-ios/ios"
)

// iTunesLockdownDomain exposes the native screen resolution over plain usbmux
// lockdown, so no tunnel and no on-device agent are needed. MobileGestalt
// (main-screen-*) is deprecated on iOS 17.4+ and returns nothing.
const iTunesLockdownDomain = "com.apple.mobile.iTunes"

// readScreenSizeFromLockdown returns the screen size in points, like DeviceKit.
func readScreenSizeFromLockdown(udid string) (*ScreenSize, error) {
	device, err := goios.GetDevice(udid)
	if err != nil {
		return nil, fmt.Errorf("device not found: %s: %w", udid, err)
	}

	conn, err := goios.ConnectLockdownWithSession(device)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to lockdown: %w", err)
	}
	defer conn.Close()

	values := make(map[string]any, 3)
	for _, key := range []string{"ScreenWidth", "ScreenHeight", "ScreenScaleFactor"} {
		value, err := conn.GetValueForDomain(key, iTunesLockdownDomain)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s from lockdown: %w", key, err)
		}
		values[key] = value
	}

	return screenSizeFromLockdownValues(values["ScreenWidth"], values["ScreenHeight"], values["ScreenScaleFactor"])
}

// screenSizeFromLockdownValues converts lockdown's native pixel resolution to points.
func screenSizeFromLockdownValues(widthPixels, heightPixels, scaleFactor any) (*ScreenSize, error) {
	width, okWidth := lockdownNumber(widthPixels)
	height, okHeight := lockdownNumber(heightPixels)
	scale, okScale := lockdownNumber(scaleFactor)
	if !okWidth || !okHeight || !okScale || width <= 0 || height <= 0 || scale <= 0 {
		return nil, fmt.Errorf("unexpected screen values from lockdown: width=%v height=%v scale=%v", widthPixels, heightPixels, scaleFactor)
	}

	return &ScreenSize{
		Width:  int(width / scale),
		Height: int(height / scale),
		Scale:  int(scale),
	}, nil
}

// lockdownNumber accepts the integer and real plist types lockdown may return.
func lockdownNumber(value any) (float64, bool) {
	switch number := value.(type) {
	case uint64:
		return float64(number), true
	case int64:
		return float64(number), true
	case float64:
		return number, true
	default:
		return 0, false
	}
}
