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

const agentVersion = "0.0.10"
const agentBundleID = "com.mobilenext.devicekit-ios"
const agentRunnerBundleID = "com.mobilenext.devicekit-iosUITests.xctrunner"

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

		if device.Platform() != "ios" {
			return fmt.Errorf("agent install is currently only supported on iOS devices")
		}

		apps, err := device.ListApps()
		if err != nil {
			return fmt.Errorf("failed to list apps: %w", err)
		}

		for _, app := range apps {
			if app.PackageName == agentBundleID {
				utils.Verbose("agent already installed: %s %s", app.AppName, app.Version)
				printJson(commands.NewSuccessResponse(map[string]any{
					"message": "agent is already installed",
					"version": app.Version,
				}))
				return nil
			}
		}

		switch device.DeviceType() {
		case "simulator":
			return installAgentOnSimulator(device)
		case "real":
			return installAgentOnRealIOS(device)
		default:
			return fmt.Errorf("unsupported device type: %s", device.DeviceType())
		}
	},
}

func installAgentOnSimulator(device devices.ControllableDevice) error {
	var arch string
	if runtime.GOARCH == "amd64" {
		arch = "x86_64"
	} else {
		arch = "arm64"
	}

	agentURL := fmt.Sprintf("https://github.com/mobile-next/devicekit-ios/releases/download/%s/devicekit-ios-Sim-%s.zip", agentVersion, arch)
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
	agentURL := fmt.Sprintf("https://github.com/mobile-next/devicekit-ios/releases/download/%s/devicekit-ios-unsigned.ipa", agentVersion)
	tmpPath := filepath.Join(os.TempDir(), "devicekit-ios-unsigned.ipa")

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

func waitForAgentInstalled(device devices.ControllableDevice) error {
	startTime := time.Now()
	for {
		apps, err := device.ListApps()
		if err == nil {
			for _, app := range apps {
				if app.PackageName == agentRunnerBundleID {
					printJson(commands.NewSuccessResponse(map[string]any{
						"message": "agent installed successfully",
						"version": app.Version,
					}))
					return nil
				}
			}
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
}
