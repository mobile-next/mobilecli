package cli

import (
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var logsLimit int
var logsProcess string
var logsPID int

var deviceLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Stream device logs",
	Long:  `Streams real-time logs from a device. Press Ctrl+C to stop.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.LogsCommand(commands.LogsRequest{
			DeviceID: deviceId,
			Limit:    logsLimit,
			Process:  logsProcess,
			PID:      logsPID,
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
	deviceLogsCmd.Flags().StringVar(&logsProcess, "process", "", "Filter by process name (substring match)")
	deviceLogsCmd.Flags().IntVar(&logsPID, "pid", -1, "Filter by process ID")
}
