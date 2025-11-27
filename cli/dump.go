package cli

import (
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Dump operations with devices",
	Long:  `Perform dump operations like UI tree extraction from devices.`,
}

var dumpUIFormat string

var dumpUICmd = &cobra.Command{
	Use:   "ui",
	Short: "Dump UI tree from a device",
	Long:  `Starts an agent and dumps the UI tree from the specified device.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.DumpUIRequest{
			DeviceID: deviceId,
			Format:   dumpUIFormat,
		}

		response := commands.DumpUICommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(dumpCmd)

	// add dump subcommands
	dumpCmd.AddCommand(dumpUICmd)

	// dump ui command flags
	dumpUICmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to dump UI tree from")
	dumpUICmd.Flags().StringVar(&dumpUIFormat, "format", "", "Output format: 'raw' for unprocessed tree from agent (Default: json)")
}
