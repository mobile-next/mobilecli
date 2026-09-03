package cli

import (
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/mobile-next/mobilecli/daemon"
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

		raw, err := callDaemon("cli.dump.ui", req, daemon.NoTimeout)
		if err != nil {
			printJson(errorEnvelope(err))
			return err
		}

		// Printing text through the JSON envelope would escape every newline,
		// which defeats the point of the format.
		var dumpResponse commands.DumpUIResponse
		if err := decodeEnvelope(raw, &dumpResponse); err == nil && dumpResponse.Text != "" {
			fmt.Print(dumpResponse.Text)
			return nil
		}

		out, err := indentedJSON(raw)
		if err != nil {
			return err
		}
		fmt.Println(out)
		return envelopeError(raw)
	},
}

func init() {
	rootCmd.AddCommand(dumpCmd)

	// add dump subcommands
	dumpCmd.AddCommand(dumpUICmd)

	// dump ui command flags
	dumpUICmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to dump UI tree from")
	dumpUICmd.Flags().StringVar(&dumpUIFormat, "format", "", "Output format: 'text' for indented element lines, 'raw' for unprocessed tree from agent (Default: json)")
}
