package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/utils"
	"github.com/spf13/cobra"
)

const (
	agentVersionIOS     = "0.0.20"
	agentVersionTVOS    = "0.0.20"
	agentVersionAndroid = "1.2.4"
	iosRunnerBundleID   = "com.mobilenext.devicekit-iosUITests.xctrunner"
	tvosRunnerBundleID  = "com.mobilenext.devicekit-tvosUITests.xctrunner"
	androidPackageName  = "com.mobilenext.devicekit"
)

// pinned SHA-256 checksums for agent artifacts, keyed by download filename
var agentChecksums = map[string]string{
	"devicekit-ios-Sim-arm64.zip":  "8040f4918892f63d79713b5824184ac5f296c5ec9b23266c25af34777550f28c",
	"devicekit-ios-Sim-x86_64.zip": "78a8f2d208a22523efbaa5cb2a735557e807f877bb8ec1a1c31c886f2e425684",
	"devicekit-ios-runner.ipa":     "f5fe88d4169c39001ed012101651c5ac00e8ab54aefb72c74455e7037c2e8205",
	// tvOS simulator runner is published from the same devicekit-ios release.
	// Update this checksum whenever the tvOS runner artifact is rebuilt/republished.
	"devicekit-tvos-Sim-arm64.zip": "49061f17046055c7e89dbf27067a59ab0bffd9d7d14d63031d5411c5963ccb81",
	"devicekit.apk":                "63b1111fbd3b986c7452bc7c28150b1e9c0d611b2ecd7f6917a0f50a84d0836b",
}

type agentMessageResponse struct {
	Message string `json:"message"`
}

type agentInfo struct {
	Version  string `json:"version"`
	BundleID string `json:"bundleId"`
}

type agentStatusResponse struct {
	Message string    `json:"message"`
	Agent   agentInfo `json:"agent"`
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent management commands",
	Long:  `Commands for managing the on-device agent.`,
}

var agentStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check agent installation status on a device",
	RunE: func(cmd *cobra.Command, args []string) error {
		device, err := commands.FindDeviceOrAutoSelect(deviceId)
		if err != nil {
			return err
		}

		agent := findInstalledAgent(device)
		if agent == nil {
			printJson(&commands.CommandResponse{
				Status: "fail",
				Data: agentMessageResponse{
					Message: "Agent is not installed on the device",
				},
			})
			return nil
		}

		printJson(commands.NewSuccessResponse(agentStatusResponse{
			Message: fmt.Sprintf("Agent version %s is installed on device", agent.Version),
			Agent: agentInfo{
				Version:  agent.Version,
				BundleID: agent.PackageName,
			},
		}))
		return nil
	},
}

var agentInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the agent on a device",
	Long:  `Installs the on-device agent on the specified device.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		device, err := commands.FindDeviceOrAutoSelect(deviceId)
		if err != nil {
			return err
		}

		utils.Verbose("device: %s (%s)", device.Name(), device.ID())
		utils.Verbose("platform: %s", device.Platform())
		utils.Verbose("type: %s", device.DeviceType())

		if !agentForce {
			if agent := findInstalledAgent(device); agent != nil {
				expectedVersion := agentVersionForPlatform(device.Platform())
				if agent.Version == expectedVersion {
					utils.Verbose("agent already installed with version %s", agent.Version)
					printJson(commands.NewSuccessResponse(agentStatusResponse{
						Message: "Agent is already installed",
						Agent: agentInfo{
							Version:  agent.Version,
							BundleID: agent.PackageName,
						},
					}))
					return nil
				}

				utils.Verbose("installed agent version %s differs from expected %s, uninstalling before reinstall", agent.Version, expectedVersion)
				if _, err := device.UninstallApp(agent.PackageName); err != nil {
					return fmt.Errorf("failed to uninstall existing agent: %w", err)
				}
			}
		}

		var installErr error
		switch device.Platform() {
		case "ios":
			switch device.DeviceType() {
			case "simulator":
				installErr = installAgentOnSimulator(device)
			case "real":
				if agentProvisioningProfile == "" {
					return fmt.Errorf("--provisioning-profile is required for real iOS devices")
				}
				installErr = installAgentOnRealIOS(device)
			default:
				return fmt.Errorf("unsupported device type: %s", device.DeviceType())
			}
		case "tvos":
			switch device.DeviceType() {
			case "simulator":
				installErr = installAgentOnSimulator(device)
			default:
				return fmt.Errorf("unsupported tvOS device type: %s (only the tvOS Simulator is supported)", device.DeviceType())
			}
		case "android":
			installErr = installAgentOnAndroid(device)
		default:
			return fmt.Errorf("unsupported platform: %s", device.Platform())
		}

		if installErr != nil {
			return installErr
		}

		agent := findInstalledAgent(device)
		if agent == nil {
			return fmt.Errorf("agent was installed but could not be found")
		}

		printJson(commands.NewSuccessResponse(agentStatusResponse{
			Message: "Agent installed successfully",
			Agent: agentInfo{
				Version:  agent.Version,
				BundleID: agent.PackageName,
			},
		}))
		return nil
	},
}

var agentUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the agent from a device",
	Long:  `Removes the on-device agent from the specified device.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		device, err := commands.FindDeviceOrAutoSelect(deviceId)
		if err != nil {
			return err
		}

		agent := findInstalledAgent(device)
		if agent == nil {
			printJson(&commands.CommandResponse{
				Status: "fail",
				Data: agentMessageResponse{
					Message: "Agent is not installed on the device",
				},
			})
			return nil
		}

		utils.Verbose("uninstalling agent %s from device %s", agent.PackageName, device.ID())
		if _, err := device.UninstallApp(agent.PackageName); err != nil {
			return fmt.Errorf("failed to uninstall agent: %w", err)
		}

		printJson(commands.NewSuccessResponse(agentMessageResponse{
			Message: "Agent uninstalled successfully",
		}))
		return nil
	},
}

func agentPackageForPlatform(platform string) string {
	switch platform {
	case "android":
		return androidPackageName
	case "ios":
		return iosRunnerBundleID
	case "tvos":
		return tvosRunnerBundleID
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
	case "tvos":
		return agentVersionTVOS
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

	var filename, version string
	switch device.Platform() {
	case "tvos":
		// The tvOS runner is currently published for Apple Silicon (arm64) only.
		if arch != "arm64" {
			return fmt.Errorf("the tvOS simulator runner is only available for arm64 (Apple Silicon)")
		}
		filename = "devicekit-tvos-Sim-arm64.zip"
		version = agentVersionTVOS
	default:
		filename = fmt.Sprintf("devicekit-ios-Sim-%s.zip", arch)
		version = agentVersionIOS
	}

	agentURL := fmt.Sprintf("https://github.com/mobile-next/devicekit-ios/releases/download/%s/%s", version, filename)

	tmpDir, err := os.MkdirTemp("", "mobilecli-agent-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	return downloadAndInstallAgent(device, agentURL, filepath.Join(tmpDir, filename), nil)
}

func installAgentOnRealIOS(device devices.ControllableDevice) error {
	filename := "devicekit-ios-runner.ipa"
	agentURL := fmt.Sprintf("https://github.com/mobile-next/devicekit-ios/releases/download/%s/%s", agentVersionIOS, filename)

	tmpDir, err := os.MkdirTemp("", "mobilecli-agent-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	return downloadAndInstallAgent(device, agentURL, filepath.Join(tmpDir, filename), func(downloaded string) (string, error) {
		utils.Verbose("re-signing agent with provisioning profile %s", agentProvisioningProfile)
		resignedPath, err := utils.ResignIPA(downloaded, device.ID(), agentProvisioningProfile, "")
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

func init() {
	rootCmd.AddCommand(agentCmd)

	agentCmd.AddCommand(agentInstallCmd)
	agentCmd.AddCommand(agentStatusCmd)
	agentCmd.AddCommand(agentUninstallCmd)

	agentInstallCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to install the agent on")
	agentStatusCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to check")
	agentUninstallCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to uninstall the agent from")
	agentInstallCmd.Flags().BoolVar(&agentForce, "force", false, "force install even if agent is already installed")
	agentInstallCmd.Flags().StringVar(&agentProvisioningProfile, "provisioning-profile", "", "path to a .mobileprovision file to use for re-signing (required for real iOS devices)")
}
