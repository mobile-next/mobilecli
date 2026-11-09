package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var locationWait bool

var locationCmd = &cobra.Command{
	Use:   "location",
	Short: "Device location commands",
	Long:  `Commands for overriding the GPS location reported by a device.`,
}

var locationSetCmd = &cobra.Command{
	Use:   "set [latitude,longitude]",
	Short: "Override the device location",
	Long: `Overrides the GPS location reported by the device, for example 37.7749,-122.4194.

The override is held by the background mobilecli daemon, which stays alive
until the override is cleared. With --wait, mobilecli also keeps running in the
foreground and clears the override when interrupted with Ctrl-C.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		lat, lon, err := commands.ParseLatLon(args[0])
		if err != nil {
			return err
		}

		err = runViaDaemon("cli.device.location.set", commands.LocationSetRequest{
			DeviceID:  deviceId,
			Latitude:  lat,
			Longitude: lon,
		})
		if err != nil {
			return err
		}

		if locationWait {
			waitForInterrupt()
			clearLocationOnExit()
		}

		return nil
	},
}

var locationClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear a location override",
	Long:  `Removes a location override, restoring the location the device reports on its own.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runViaDaemon("cli.device.location.clear", commands.LocationClearRequest{
			DeviceID: deviceId,
		})
	},
}

// waitForInterrupt blocks until the user presses Ctrl-C
func waitForInterrupt() {
	fmt.Fprintln(os.Stderr, "Holding location override, press Ctrl-C to clear it")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	<-sigChan
}

// clearLocationOnExit clears the override we are holding, reporting a failure
// as a warning: we are on our way out either way.
func clearLocationOnExit() {
	if err := runViaDaemon("cli.device.location.clear", commands.LocationClearRequest{DeviceID: deviceId}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to clear location: %s\n", err)
	}
}

func init() {
	deviceCmd.AddCommand(locationCmd)

	locationCmd.AddCommand(locationSetCmd)
	locationCmd.AddCommand(locationClearCmd)

	locationSetCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to set the location on")
	locationSetCmd.Flags().BoolVar(&locationWait, "wait", false, "Keep the override until Ctrl-C, then clear it")
	locationClearCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to clear the location on")
}
