package types

// TapAction represents a single action in a gesture sequence.
// Used by DeviceKit gesture API (press/move/release).
type TapAction struct {
	Type     string `json:"type"`
	Duration int    `json:"duration"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Button   int    `json:"button"`
}

// Size represents width and height dimensions.
type Size struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// WindowSize represents the device screen size and scale factor.
type WindowSize struct {
	Scale      int  `json:"scale"`
	ScreenSize Size `json:"screenSize"`
}

// ActiveAppInfo represents information about the currently active/foreground application.
type ActiveAppInfo struct {
	BundleID  string `json:"bundleId"`
	Name      string `json:"name"`
	ProcessID int    `json:"pid"`
}
