package cli

import (
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
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
			return fmt.Errorf(response.Error)
		}

		return nil
	},
}

var deviceInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get device info",
	Long:  `Get detailed information about a connected device, such as OS, version, and screen size.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.InfoCommand(deviceId)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deviceCmd)
	
	// add device subcommands
	deviceCmd.AddCommand(deviceRebootCmd)
	deviceCmd.AddCommand(deviceInfoCmd)

	// device command flags
	deviceRebootCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to reboot")
	deviceInfoCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to get info from")
}