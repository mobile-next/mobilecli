package commands

import (
	"encoding/json"
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
)

// CLIHandler runs one CLI command from JSON params and returns the full
// response envelope, exactly as the CLI printed it before the daemon existed.
type CLIHandler func(params json.RawMessage) *CommandResponse

// DeviceIDRequest is the params shape for commands that only take a device id.
type DeviceIDRequest struct {
	DeviceID string `json:"deviceId"`
}

// CrashesGetRequest is the params shape for cli.device.crashes.get.
type CrashesGetRequest struct {
	DeviceID string `json:"deviceId"`
	ID       string `json:"id"`
}

// adapt turns a typed command function into a CLIHandler.
func adapt[R any](fn func(R) *CommandResponse) CLIHandler {
	return func(params json.RawMessage) *CommandResponse {
		var req R
		if len(params) > 0 {
			if err := json.Unmarshal(params, &req); err != nil {
				return NewErrorResponse(fmt.Errorf("invalid parameters: %w", err))
			}
		}
		return fn(req)
	}
}

// CLIRegistry maps CLI method names (mirroring the cobra command path) to handlers.
func CLIRegistry() map[string]CLIHandler {
	return map[string]CLIHandler{
		"cli.devices": adapt(func(opts devices.DeviceListOptions) *CommandResponse {
			return DevicesCommand(opts, GetFleetToken())
		}),
		"cli.device.info":            adapt(func(r DeviceIDRequest) *CommandResponse { return InfoCommand(r.DeviceID) }),
		"cli.device.reboot":          adapt(RebootCommand),
		"cli.device.boot":            adapt(BootCommand),
		"cli.device.shutdown":        adapt(ShutdownCommand),
		"cli.device.orientation.get": adapt(OrientationGetCommand),
		"cli.device.orientation.set": adapt(OrientationSetCommand),
		"cli.device.settings.apply":  adapt(ApplySettingsCommand),
		"cli.device.crashes.list":    adapt(func(r DeviceIDRequest) *CommandResponse { return CrashesListCommand(r.DeviceID) }),
		"cli.device.crashes.get":     adapt(func(r CrashesGetRequest) *CommandResponse { return CrashesGetCommand(r.DeviceID, r.ID) }),
		"cli.device.location.set":    adapt(LocationSetCommand),
		"cli.device.location.clear":  adapt(LocationClearCommand),
		"cli.apps.launch":            adapt(LaunchAppCommand),
		"cli.apps.terminate":         adapt(TerminateAppCommand),
		"cli.apps.list":              adapt(ListAppsCommand),
		"cli.apps.install":           adapt(InstallAppCommand),
		"cli.apps.uninstall":         adapt(UninstallAppCommand),
		"cli.apps.clear":             adapt(ClearAppCommand),
		"cli.apps.foreground":        adapt(ForegroundAppCommand),
		"cli.apps.path":              adapt(AppPathCommand),
		"cli.io.tap":                 adapt(TapCommand),
		"cli.io.longpress":           adapt(LongPressCommand),
		"cli.io.button":              adapt(ButtonCommand),
		"cli.io.text":                adapt(TextCommand),
		"cli.io.keys":                adapt(KeysCommand),
		"cli.io.swipe":               adapt(SwipeCommand),
		"cli.io.gesture":             adapt(GestureCommand),
		"cli.io.clipboard.get":       adapt(ClipboardGetCommand),
		"cli.io.clipboard.set":       adapt(ClipboardSetCommand),
		"cli.fs.push":                adapt(FsPushCommand),
		"cli.fs.pull":                adapt(FsPullCommand),
		"cli.fs.ls":                  adapt(FsListCommand),
		"cli.fs.mkdir":               adapt(FsMkdirCommand),
		"cli.fs.rm":                  adapt(FsRmCommand),
		"cli.webview.list":           adapt(WebViewListCommand),
		"cli.webview.goto":           adapt(WebViewGotoCommand),
		"cli.webview.reload":         adapt(WebViewReloadCommand),
		"cli.webview.back":           adapt(WebViewGoBackCommand),
		"cli.webview.forward":        adapt(WebViewGoForwardCommand),
		"cli.webview.eval":           adapt(WebViewEvaluateCommand),
		"cli.webview.wait":           adapt(WebViewWaitForLoadStateCommand),
		"cli.webview.content":        adapt(WebViewContentCommand),
		"cli.webview.query":          adapt(WebViewQueryCommand),
		"cli.screenshot":             adapt(ScreenshotCommand),
		"cli.dump.ui":                adapt(DumpUICommand),
		"cli.url":                    adapt(URLCommand),
		"cli.agent.status":           adapt(AgentStatusCommand),
		"cli.agent.install":          adapt(AgentInstallCommand),
		"cli.agent.uninstall":        adapt(AgentUninstallCommand),
	}
}
