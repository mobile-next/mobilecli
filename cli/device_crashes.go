package cli

import (
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var deviceCrashesCmd = &cobra.Command{
	Use:   "crashes",
	Short: "Manage crash reports from a device",
}

var deviceCrashesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List crash reports from a device",
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.CrashesListCommand(deviceId)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var deviceCrashesGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get a crash report by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.CrashesGetCommand(deviceId, args[0])
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(deviceCrashesCmd)

	deviceCrashesCmd.AddCommand(deviceCrashesListCmd)
	deviceCrashesCmd.AddCommand(deviceCrashesGetCmd)

	deviceCrashesListCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to list crashes from")
	deviceCrashesGetCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to get crash from")
}
