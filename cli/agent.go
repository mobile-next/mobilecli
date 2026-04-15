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

		switch device.Platform() {
		case "ios":
			switch device.DeviceType() {
			case "simulator":
				return installAgentOnSimulator(device)
			case "real":
				if agentProvisioningProfile == "" {
					return fmt.Errorf("--provisioning-profile is required for real iOS devices")
				}
				return installAgentOnRealIOS(device)
			default:
				return fmt.Errorf("unsupported device type: %s", device.DeviceType())
			}
		case "android":
			return installAgentOnAndroid(device)
		default:
			return fmt.Errorf("unsupported platform: %s", device.Platform())
		}
	},
}

func agentPackageForPlatform(platform string) string {
	if platform == "android" {
		return androidPackageName
	}
	return iosRunnerBundleID
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

	utils.Verbose("downloading agent from %s", agentURL)
	if err := utils.DownloadFile(agentURL, tmpPath); err != nil {
		return fmt.Errorf("failed to download agent: %w", err)
	}
	utils.Verbose("downloaded agent to %s", tmpPath)
	defer func() { _ = os.Remove(tmpPath) }()

	utils.Verbose("installing agent on simulator %s", device.ID())
	if err := device.InstallApp(tmpPath); err != nil {
		return fmt.Errorf("failed to install agent: %w", err)
	}

	return waitForAgentInstalled(device)
}

func installAgentOnRealIOS(device devices.ControllableDevice) error {
	agentURL := fmt.Sprintf("https://github.com/mobile-next/devicekit-ios/releases/download/%s/devicekit-ios-runner.ipa", agentVersionIOS)
	tmpPath := filepath.Join(os.TempDir(), "devicekit-ios-runner.ipa")

	utils.Verbose("downloading agent from %s", agentURL)
	if err := utils.DownloadFile(agentURL, tmpPath); err != nil {
		return fmt.Errorf("failed to download agent: %w", err)
	}
	utils.Verbose("downloaded agent to %s", tmpPath)
	defer func() { _ = os.Remove(tmpPath) }()

	utils.Verbose("re-signing agent with provisioning profile %s", agentProvisioningProfile)
	resignedPath, err := utils.ResignIPA(tmpPath, device.ID(), agentProvisioningProfile, "")
	if err != nil {
		return fmt.Errorf("failed to re-sign agent: %w", err)
	}
	defer func() { _ = os.Remove(resignedPath) }()

	utils.Verbose("installing agent on device %s", device.ID())
	if err := device.InstallApp(resignedPath); err != nil {
		return fmt.Errorf("failed to install agent: %w", err)
	}

	return waitForAgentInstalled(device)
}

func installAgentOnAndroid(device devices.ControllableDevice) error {
	agentURL := fmt.Sprintf("https://github.com/mobile-next/devicekit-android/releases/download/%s/mobilenext-devicekit.apk", agentVersionAndroid)
	tmpPath := filepath.Join(os.TempDir(), "mobilenext-devicekit.apk")

	utils.Verbose("downloading agent from %s", agentURL)
	if err := utils.DownloadFile(agentURL, tmpPath); err != nil {
		return fmt.Errorf("failed to download agent: %w", err)
	}
	utils.Verbose("downloaded agent to %s", tmpPath)
	defer func() { _ = os.Remove(tmpPath) }()

	utils.Verbose("installing agent on device %s", device.ID())
	if err := device.InstallApp(tmpPath); err != nil {
		return fmt.Errorf("failed to install agent: %w", err)
	}

	return waitForAgentInstalled(device)
}

func isAgentInstalled(device devices.ControllableDevice) bool {
	agentPackage := agentPackageForPlatform(device.Platform())

	if androidDevice, ok := device.(*devices.AndroidDevice); ok {
		packages, err := androidDevice.ListAllPackages()
		if err != nil {
			return false
		}
		for _, pkg := range packages {
			if pkg == agentPackage {
				return true
			}
		}
		return false
	}

	apps, err := device.ListApps()
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
			printJson(commands.NewSuccessResponse(map[string]any{
				"message": "agent installed successfully",
			}))
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
