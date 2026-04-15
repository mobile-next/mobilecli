package cli

import (
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var logsLimit int
var logsFilters []string

var deviceLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Stream device logs",
	Long: `Streams real-time logs from a device. Press Ctrl+C to stop.

Filters use key=value (include) or key!=value (exclude) syntax.
Multiple --filter flags are ANDed together.

Supported keys: pid, process, tag, level, subsystem, category, message

Examples:
  mobilecli device logs --filter tag=ActivityManager
  mobilecli device logs --filter process!=SpringBoard
  mobilecli device logs --filter level=Error --filter process=backboardd`,
	RunE: func(cmd *cobra.Command, args []string) error {
		filters, err := commands.ParseLogFilters(logsFilters)
		if err != nil {
			return err
		}
		response := commands.LogsCommand(commands.LogsRequest{
			DeviceID: deviceId,
			Limit:    logsLimit,
			Filters:  filters,
		})
		if response.Status == "error" {
			printJson(response)
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(deviceLogsCmd)
	deviceLogsCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to stream logs from")
	deviceLogsCmd.Flags().IntVar(&logsLimit, "limit", 0, "Stop after N log entries (0 = unlimited)")
	deviceLogsCmd.Flags().StringArrayVar(&logsFilters, "filter", nil, "Filter logs (key=value or key!=value, repeatable)")
}
