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

With --wait, mobilecli keeps running and clears the override when interrupted
with Ctrl-C. This is required on iOS 17+ physical devices, where the simulated
location only lasts as long as the mobilecli process that set it.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		lat, lon, err := commands.ParseLatLon(args[0])
		if err != nil {
			return err
		}

		response := commands.LocationSetCommand(commands.LocationSetRequest{
			DeviceID:  deviceId,
			Latitude:  lat,
			Longitude: lon,
		})

		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
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
		response := commands.LocationClearCommand(commands.LocationClearRequest{
			DeviceID: deviceId,
		})

		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}

		return nil
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
	response := commands.LocationClearCommand(commands.LocationClearRequest{
		DeviceID: deviceId,
	})

	if response.Status == "error" {
		fmt.Fprintf(os.Stderr, "Warning: failed to clear location: %s\n", response.Error)
		return
	}

	printJson(response)
}

func init() {
	deviceCmd.AddCommand(locationCmd)

	locationCmd.AddCommand(locationSetCmd)
	locationCmd.AddCommand(locationClearCmd)

	locationSetCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to set the location on")
	locationSetCmd.Flags().BoolVar(&locationWait, "wait", false, "Keep the override until Ctrl-C, then clear it")
	locationClearCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to clear the location on")
}
