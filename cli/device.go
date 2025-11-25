package cli

import (
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/mobile-next/mobilecli/devices"
	"github.com/spf13/cobra"
)

var deviceCmd = &cobra.Command{
	Use:   "device",
	Short: "Device management commands",
	Long:  `Commands for managing individual devices including rebooting and getting device information.`,
}

var deviceRebootCmd = &cobra.Command{
	Use:   "reboot",
	Short: "Reboot a connected device or simulator",
	Long:  `Reboots a specified device (using its ID). Supports iOS (real/simulator) and Android (real/emulator).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.RebootRequest{
			DeviceID: deviceId,
		}

		response := commands.RebootCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}

		return nil
	},
}

var deviceInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get device info",
	Long:  `Get detailed information about a connected device, such as OS, version, and screen size.`,
	RunE: func(cmd *cobra.Command, args []string) error {

		// Find the target device
		targetDevice, err := commands.FindDeviceOrAutoSelect(deviceId)
		if err != nil {
			response := commands.NewErrorResponse(fmt.Errorf("error finding device: %v", err))
			printJson(response)
			return fmt.Errorf("%s", response.Error)
		}

		// Start agent
		err = targetDevice.StartAgent(devices.StartAgentConfig{})
		if err != nil {
			response := commands.NewErrorResponse(fmt.Errorf("error starting agent: %v", err))
			printJson(response)
			return fmt.Errorf("%s", response.Error)
		}

		response := commands.InfoCommand(deviceId)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var orientationCmd = &cobra.Command{
	Use:   "orientation",
	Short: "Device orientation commands",
	Long:  `Commands for getting and setting device orientation.`,
}

var orientationGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get current device orientation",
	Long:  `Get the current orientation of the device (portrait or landscape).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.OrientationGetRequest{
			DeviceID: deviceId,
		}

		response := commands.OrientationGetCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}

		return nil
	},
}

var orientationSetCmd = &cobra.Command{
	Use:   "set [orientation]",
	Short: "Set device orientation",
	Long:  `Set the device orientation to portrait or landscape.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.OrientationSetRequest{
			DeviceID:    deviceId,
			Orientation: args[0],
		}

		response := commands.OrientationSetCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}

		return nil
	},
}

var deviceBootCmd = &cobra.Command{
	Use:   "boot",
	Short: "Boot a simulator or emulator",
	Long:  `Boots a specified offline simulator or emulator.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.BootRequest{
			DeviceID: deviceId,
		}

		response := commands.BootCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}

		return nil
	},
}

var deviceShutdownCmd = &cobra.Command{
	Use:   "shutdown",
	Short: "Shutdown a simulator or emulator",
	Long:  `Shuts down a specified simulator or emulator.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.ShutdownRequest{
			DeviceID: deviceId,
		}

		response := commands.ShutdownCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(deviceCmd)

	// add device subcommands
	deviceCmd.AddCommand(deviceRebootCmd)
	deviceCmd.AddCommand(deviceInfoCmd)
	deviceCmd.AddCommand(deviceBootCmd)
	deviceCmd.AddCommand(deviceShutdownCmd)
	deviceCmd.AddCommand(orientationCmd)

	// add orientation subcommands
	orientationCmd.AddCommand(orientationGetCmd)
	orientationCmd.AddCommand(orientationSetCmd)

	// device command flags
	deviceRebootCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to reboot")
	deviceInfoCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to get info from")
	deviceBootCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to boot")
	deviceShutdownCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to shutdown")
	orientationGetCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to get orientation from")
	orientationSetCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to set orientation on")
}
