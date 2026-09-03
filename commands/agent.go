package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/utils"
)

// AgentInstallRequest are the params for cli.agent.install.
type AgentInstallRequest struct {
	DeviceID            string `json:"deviceId"`
	Force               bool   `json:"force,omitempty"`
	ProvisioningProfile string `json:"provisioningProfile,omitempty"`
}

const (
	agentVersionIOS     = "0.0.26"
	agentVersionAndroid = "1.2.6"
	iosRunnerBundleID   = "com.mobilenext.devicekit-iosUITests.xctrunner"
	androidPackageName  = "com.mobilenext.devicekit"
)

// pinned SHA-256 checksums for agent artifacts, keyed by download filename
var agentChecksums = map[string]string{
	"devicekit-ios-Sim-arm64.zip":  "c20c890e811e4f019a60807bdaefb724dba417cf1ac97197119cac738e32f24f",
	"devicekit-ios-Sim-x86_64.zip": "fc803e3518c6478091773161103fe8170abfe2a900764d10591fb7bb90c8fddc",
	"devicekit-ios-runner.ipa":     "a5d4a2a9a353fb9c0d25c6e52a64cc6729cdc9f3a8f80c874b25d4599731ecb9",
	"devicekit.apk":                "01d933a311dac113bb89f2cb3256482467c1e02b287a2fd5e412863b8f907c51",
}

// AgentMessageResponse is the data of agent status/uninstall responses.
type AgentMessageResponse struct {
	Message string `json:"message"`
}

// AgentInfo describes the installed on-device agent.
type AgentInfo struct {
	Version  string `json:"version"`
	BundleID string `json:"bundleId"`
}

// AgentStatusResponse is the data of agent install/status responses.
type AgentStatusResponse struct {
	Message string    `json:"message"`
	Agent   AgentInfo `json:"agent"`
}

func agentNotInstalledResponse() *CommandResponse {
	return &CommandResponse{
		Status: "fail",
		Data:   AgentMessageResponse{Message: "Agent is not installed on the device"},
	}
}

func agentStatusResponse(message string, agent *devices.InstalledAppInfo) *CommandResponse {
	return NewSuccessResponse(AgentStatusResponse{
		Message: message,
		Agent:   AgentInfo{Version: agent.Version, BundleID: agent.PackageName},
	})
}

// AgentStatusCommand reports whether the on-device agent is installed.
func AgentStatusCommand(req DeviceIDRequest) *CommandResponse {
	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	agent := findInstalledAgent(device)
	if agent == nil {
		return agentNotInstalledResponse()
	}
	return agentStatusResponse(fmt.Sprintf("Agent version %s is installed on device", agent.Version), agent)
}

// AgentInstallCommand downloads, verifies and installs the on-device agent.
func AgentInstallCommand(req AgentInstallRequest) *CommandResponse {
	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	utils.Verbose("device: %s (%s)", device.Name(), device.ID())
	utils.Verbose("platform: %s", device.Platform())
	utils.Verbose("type: %s", device.DeviceType())

	if !req.Force {
		if agent := findInstalledAgent(device); agent != nil {
			expectedVersion := agentVersionForPlatform(device.Platform())
			if agent.Version == expectedVersion {
				utils.Verbose("agent already installed with version %s", agent.Version)
				return agentStatusResponse("Agent is already installed", agent)
			}

			utils.Verbose("installed agent version %s differs from expected %s, uninstalling before reinstall", agent.Version, expectedVersion)
			if _, err := device.UninstallApp(agent.PackageName); err != nil {
				return NewErrorResponse(fmt.Errorf("failed to uninstall existing agent: %w", err))
			}
		}
	}

	if err := installAgent(device, req.ProvisioningProfile); err != nil {
		return NewErrorResponse(err)
	}

	agent := findInstalledAgent(device)
	if agent == nil {
		return NewErrorResponse(fmt.Errorf("agent was installed but could not be found"))
	}
	return agentStatusResponse("Agent installed successfully", agent)
}

func installAgent(device devices.ControllableDevice, provisioningProfile string) error {
	switch device.Platform() {
	case "ios":
		switch device.DeviceType() {
		case "simulator":
			return installAgentOnSimulator(device)
		case "real":
			if provisioningProfile == "" {
				return fmt.Errorf("--provisioning-profile is required for real iOS devices")
			}
			return installAgentOnRealIOS(device, provisioningProfile)
		default:
			return fmt.Errorf("unsupported device type: %s", device.DeviceType())
		}
	case "android":
		return installAgentOnAndroid(device)
	default:
		return fmt.Errorf("unsupported platform: %s", device.Platform())
	}
}

// AgentUninstallCommand removes the on-device agent.
func AgentUninstallCommand(req DeviceIDRequest) *CommandResponse {
	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	agent := findInstalledAgent(device)
	if agent == nil {
		return agentNotInstalledResponse()
	}

	utils.Verbose("uninstalling agent %s from device %s", agent.PackageName, device.ID())
	if _, err := device.UninstallApp(agent.PackageName); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to uninstall agent: %w", err))
	}
	return NewSuccessResponse(AgentMessageResponse{Message: "Agent uninstalled successfully"})
}

func agentPackageForPlatform(platform string) string {
	switch platform {
	case "android":
		return androidPackageName
	case "ios":
		return iosRunnerBundleID
	default:
		return ""
	}
}

func agentVersionForPlatform(platform string) string {
	switch platform {
	case "android":
		return agentVersionAndroid
	case "ios":
		return agentVersionIOS
	default:
		return ""
	}
}

func downloadAndInstallAgent(device devices.ControllableDevice, agentURL, tmpPath string, transform func(string) (string, error)) error {
	utils.Verbose("downloading agent from %s", agentURL)
	if err := utils.DownloadFile(agentURL, tmpPath); err != nil {
		return fmt.Errorf("failed to download agent: %w", err)
	}
	utils.Verbose("downloaded agent to %s", tmpPath)
	defer func() { _ = os.Remove(tmpPath) }()

	filename := filepath.Base(tmpPath)
	expectedHash, ok := agentChecksums[filename]
	if !ok {
		return fmt.Errorf("no pinned checksum for %s", filename)
	}
	actualHash, err := utils.SHA256File(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", filename, expectedHash, actualHash)
	}
	utils.Verbose("checksum verified for %s", filename)

	installPath := tmpPath
	if transform != nil {
		var err error
		installPath, err = transform(tmpPath)
		if err != nil {
			return err
		}
		defer func() { _ = os.Remove(installPath) }()
	}

	utils.Verbose("installing agent on device %s", device.ID())
	if err := device.InstallApp(installPath); err != nil {
		return fmt.Errorf("failed to install agent: %w", err)
	}

	return waitForAgentInstalled(device)
}

func installAgentOnSimulator(device devices.ControllableDevice) error {
	var arch string
	if runtime.GOARCH == "amd64" {
		arch = "x86_64"
	} else {
		arch = "arm64"
	}

	filename := fmt.Sprintf("devicekit-ios-Sim-%s.zip", arch)
	agentURL := fmt.Sprintf("https://github.com/mobile-next/devicekit-ios/releases/download/%s/%s", agentVersionIOS, filename)

	tmpDir, err := os.MkdirTemp("", "mobilecli-agent-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	return downloadAndInstallAgent(device, agentURL, filepath.Join(tmpDir, filename), nil)
}

func installAgentOnRealIOS(device devices.ControllableDevice, provisioningProfile string) error {
	filename := "devicekit-ios-runner.ipa"
	agentURL := fmt.Sprintf("https://github.com/mobile-next/devicekit-ios/releases/download/%s/%s", agentVersionIOS, filename)

	tmpDir, err := os.MkdirTemp("", "mobilecli-agent-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	return downloadAndInstallAgent(device, agentURL, filepath.Join(tmpDir, filename), func(downloaded string) (string, error) {
		utils.Verbose("re-signing agent with provisioning profile %s", provisioningProfile)
		resignedPath, err := utils.ResignIPA(downloaded, device.ID(), provisioningProfile, "")
		if err != nil {
			return "", fmt.Errorf("failed to re-sign agent: %w", err)
		}
		return resignedPath, nil
	})
}

func installAgentOnAndroid(device devices.ControllableDevice) error {
	filename := "devicekit.apk"
	agentURL := fmt.Sprintf("https://github.com/mobile-next/devicekit-android/releases/download/%s/%s", agentVersionAndroid, filename)

	tmpDir, err := os.MkdirTemp("", "mobilecli-agent-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	return downloadAndInstallAgent(device, agentURL, filepath.Join(tmpDir, filename), nil)
}

func findInstalledAgent(device devices.ControllableDevice) *devices.InstalledAppInfo {
	agentPackage := agentPackageForPlatform(device.Platform())

	apps, err := device.ListApps(false)
	if err != nil {
		return nil
	}
	for _, app := range apps {
		if agentMatchesApp(device.Platform(), app.PackageName, agentPackage) {
			if app.Version == "" {
				if androidDevice, ok := device.(*devices.AndroidDevice); ok {
					if v, err := androidDevice.GetAppVersion(agentPackage); err == nil {
						app.Version = v
					}
				}
			}
			return &app
		}
	}
	return nil
}

// agentMatchesApp reports whether an installed app's bundle id identifies the agent.
// On iOS the runner bundle id can carry a signing/team prefix when re-signed, so a
// suffix match is used; other platforms require an exact match.
func agentMatchesApp(platform, installedPackage, agentPackage string) bool {
	if platform == "ios" {
		return strings.HasSuffix(installedPackage, agentPackage)
	}
	return installedPackage == agentPackage
}

func isAgentInstalled(device devices.ControllableDevice) bool {
	return findInstalledAgent(device) != nil
}

func waitForAgentInstalled(device devices.ControllableDevice) error {
	startTime := time.Now()
	for {
		if isAgentInstalled(device) {
			return nil
		}

		if time.Since(startTime) > 30*time.Second {
			return fmt.Errorf("agent not found after 30 seconds")
		}

		utils.Verbose("waiting for agent to appear in installed apps...")
		time.Sleep(1 * time.Second)
	}
}
