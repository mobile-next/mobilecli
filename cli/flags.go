package cli

var (
	verbose bool

	// all commands
	deviceId string

	// for screenshot command
	screenshotOutputPath  string
	screenshotFormat      string
	screenshotJpegQuality int

	// for screencapture command
	screencaptureFormat string
	// for audiocapture command
	audiocaptureFormat string

	// for devices command
	platform   string
	deviceType string
)
