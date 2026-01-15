package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var devicekitCmd = &cobra.Command{
	Use:   "devicekit",
	Short: "Manage DeviceKit on iOS devices",
	Long:  `Start and control DeviceKit on iOS devices. DeviceKit provides tap/dumpUI commands and screen streaming.`,
}

var devicekitStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start DeviceKit on an iOS device",
	Long: `Starts the devicekit-ios XCUITest which provides:
  - HTTP server for tap/dumpUI commands
  - Broadcast extension for H.264 screen streaming

The command returns the local ports that are forwarded to the device.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.DeviceKitStartRequest{
			DeviceID: deviceId,
		}

		response := commands.DeviceKitStartCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}

		// Keep the process running to maintain the XCUITest runner alive
		fmt.Fprintln(os.Stderr, "DeviceKit is running. Press Ctrl+C to stop.")

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		<-sigChan

		fmt.Fprintln(os.Stderr, "Shutting down DeviceKit...")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(devicekitCmd)

	devicekitCmd.AddCommand(devicekitStartCmd)

	devicekitStartCmd.Flags().StringVar(&deviceId, "device", "", "ID of the iOS device to start DeviceKit on")
}
