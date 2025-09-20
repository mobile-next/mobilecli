package cli

import (
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Dump operations with devices",
	Long:  `Perform dump operations like source tree extraction from devices.`,
}

var dumpSourceCmd = &cobra.Command{
	Use:   "source",
	Short: "Dump source tree from a device",
	Long:  `Starts an agent and dumps the source tree from the specified device.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.DumpSourceRequest{
			DeviceID: deviceId,
		}

		response := commands.DumpSourceCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(dumpCmd)

	// add dump subcommands
	dumpCmd.AddCommand(dumpSourceCmd)

	// dump source command flags
	dumpSourceCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to dump source tree from")
}
