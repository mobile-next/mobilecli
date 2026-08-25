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
	locale   string
	activity string

	// for agent install command
	agentForce               bool
	agentProvisioningProfile string

	// for remote allocate command
	fleetType     string
	fleetVersions []string
	fleetName     string
	fleetWait     bool
	fleetTimeout  int

	// for webview wait command
	webviewWaitState   string
	webviewWaitTimeout int
)
