package devices

import (
	"github.com/mobile-next/mobilecli/types"
)

// IOSControl is the common interface satisfied by both wda.WdaClient and devicekit.Client.
// IOSDevice holds a single controlClient of this type and delegates all
// interaction methods to it, so there is no per-method if/else branching.
type IOSControl interface {
	TakeScreenshot() ([]byte, error)
	Tap(x, y int) error
	LongPress(x, y, duration int) error
	Swipe(x1, y1, x2, y2 int) error
	Gesture(actions []types.TapAction) error
	SendKeys(text string) error
	PressButton(key string) error
	OpenURL(url string) error
	GetSourceElements() ([]types.ScreenElement, error)
	GetSourceRaw() (interface{}, error)
	GetOrientation() (string, error)
	SetOrientation(orientation string) error
	GetWindowSize() (*types.WindowSize, error)
	GetForegroundApp() (*types.ActiveAppInfo, error)
}
