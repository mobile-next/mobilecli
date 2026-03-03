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

	// for devices command
	platform   string
	deviceType string

	// for apps launch command
	locale string

	// for fleet allocate command
	fleetType     string
	fleetVersions []string
	fleetNames    []string
)
