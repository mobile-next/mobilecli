package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/utils"
	"github.com/spf13/cobra"
)

const (
	agentVersionIOS     = "0.0.12"
	agentVersionAndroid = "1.1.5"
	iosRunnerBundleID   = "com.mobilenext.devicekit-iosUITests.xctrunner"
	androidPackageName  = "com.mobilenext.devicekit"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent management commands",
	Long:  `Commands for managing the on-device agent.`,
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

		if !agentReinstall && isAgentInstalled(device) {
			utils.Verbose("agent already installed")
			printJson(commands.NewSuccessResponse(map[string]any{
				"message": "agent is already installed",
			}))
			return nil
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
		case "android":
			installErr = installAgentOnAndroid(device)
		default:
			return fmt.Errorf("unsupported platform: %s", device.Platform())
		}

		if installErr != nil {
			return installErr
		}

		printJson(commands.NewSuccessResponse(map[string]any{
			"message": "agent installed successfully",
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
	default:
		return ""
	}
}

// downloadAndInstallAgent downloads the agent, optionally transforms the downloaded file,
// installs it on the device, and waits for installation to complete.
// the transform function, if non-nil, receives the downloaded path and returns a new path to install.
func downloadAndInstallAgent(device devices.ControllableDevice, agentURL, tmpPath string, transform func(string) (string, error)) error {
	utils.Verbose("downloading agent from %s", agentURL)
	if err := utils.DownloadFile(agentURL, tmpPath); err != nil {
		return fmt.Errorf("failed to download agent: %w", err)
	}
	utils.Verbose("downloaded agent to %s", tmpPath)
	defer func() { _ = os.Remove(tmpPath) }()

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

	agentURL := fmt.Sprintf("https://github.com/mobile-next/devicekit-ios/releases/download/%s/devicekit-ios-Sim-%s.zip", agentVersionIOS, arch)
	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("devicekit-ios-Sim-%s.zip", arch))

	return downloadAndInstallAgent(device, agentURL, tmpPath, nil)
}

func installAgentOnRealIOS(device devices.ControllableDevice) error {
	agentURL := fmt.Sprintf("https://github.com/mobile-next/devicekit-ios/releases/download/%s/devicekit-ios-runner.ipa", agentVersionIOS)
	tmpPath := filepath.Join(os.TempDir(), "devicekit-ios-runner.ipa")

	return downloadAndInstallAgent(device, agentURL, tmpPath, func(downloaded string) (string, error) {
		utils.Verbose("re-signing agent with provisioning profile %s", agentProvisioningProfile)
		resignedPath, err := utils.ResignIPA(downloaded, device.ID(), agentProvisioningProfile, "")
		if err != nil {
			return "", fmt.Errorf("failed to re-sign agent: %w", err)
		}
		return resignedPath, nil
	})
}

func installAgentOnAndroid(device devices.ControllableDevice) error {
	agentURL := fmt.Sprintf("https://github.com/mobile-next/devicekit-android/releases/download/%s/mobilenext-devicekit.apk", agentVersionAndroid)
	tmpPath := filepath.Join(os.TempDir(), "mobilenext-devicekit.apk")

	return downloadAndInstallAgent(device, agentURL, tmpPath, nil)
}

func isAgentInstalled(device devices.ControllableDevice) bool {
	agentPackage := agentPackageForPlatform(device.Platform())

	apps, err := device.ListApps(false)
	if err != nil {
		return false
	}
	for _, app := range apps {
		if app.PackageName == agentPackage {
			return true
		}
	}
	return false
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

	agentInstallCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to install the agent on")
	agentInstallCmd.Flags().BoolVar(&agentReinstall, "reinstall", false, "reinstall even if agent is already installed")
	agentInstallCmd.Flags().StringVar(&agentProvisioningProfile, "provisioning-profile", "", "path to a .mobileprovision file to use for re-signing (required for real iOS devices)")
}
