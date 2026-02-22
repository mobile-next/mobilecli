package devices

import (
	"time"

	"github.com/mobile-next/mobilecli/types"
)

// IOSControl is the common interface satisfied by devicekit.Client.
// Both IOSDevice and SimulatorDevice hold a single controlClient of this type
// and delegate all interaction methods to it.
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

	// Lifecycle methods
	HealthCheck() error
	WaitForReady(timeout time.Duration) error
	Close()

	// Streaming
	StartMjpegStream(fps int, onData func([]byte) bool) error
	StartH264Stream(fps, quality int, scale float64, onData func([]byte) bool) error
}
