package cli

import (
	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var crashesCmd = &cobra.Command{
	Use:        "crashes",
	Short:      "Manage crash reports from devices",
	Deprecated: "use 'device crashes' instead",
}

var crashesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List crash reports from a device",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runViaDaemon("cli.device.crashes.list", commands.DeviceIDRequest{DeviceID: deviceId})
	},
}

var crashesGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get a crash report by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runViaDaemon("cli.device.crashes.get", commands.CrashesGetRequest{DeviceID: deviceId, ID: args[0]})
	},
}

func init() {
	rootCmd.AddCommand(crashesCmd)

	crashesCmd.AddCommand(crashesListCmd)
	crashesCmd.AddCommand(crashesGetCmd)

	crashesListCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to list crashes from")
	crashesGetCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to get crash from")
}
