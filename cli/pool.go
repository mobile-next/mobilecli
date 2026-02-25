package cli

import (
	"fmt"
	"os"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
)

func getFleetToken() (string, error) {
	if token := os.Getenv("MOBILECLI_TOKEN"); token != "" {
		return token, nil
	}

	token, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		return "", fmt.Errorf("not logged in, run 'mobilecli auth login' first")
	}

	return token, nil
}

var fleetCmd = &cobra.Command{
	Use:   "fleet",
	Short: "Device fleet management commands",
	Long:  `Commands for managing device fleet including allocating, listing, and releasing devices.`,
}

var fleetAllocateCmd = &cobra.Command{
	Use:   "allocate",
	Short: "Allocate a device from the fleet",
	Long:  `Allocates a device from the fleet for the specified platform (ios or android).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if platform != "ios" && platform != "android" {
			return fmt.Errorf("platform must be 'ios' or 'android'")
		}

		token, err := getFleetToken()
		if err != nil {
			return err
		}

		req := commands.FleetAllocateRequest{
			Platform: platform,
			Token:    token,
		}

		response := commands.FleetAllocateCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}

		return nil
	},
}

var fleetListCmd = &cobra.Command{
	Use:   "list-devices",
	Short: "List available fleet devices",
	Long:  `Lists available devices in the device fleet.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getFleetToken()
		if err != nil {
			return err
		}

		req := commands.FleetListDevicesRequest{
			Token: token,
		}

		response := commands.FleetListDevicesCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}

		return nil
	},
}

var fleetReleaseDeviceID string

var fleetReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Release a device back to the fleet",
	Long:  `Releases an allocated device back to the fleet.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getFleetToken()
		if err != nil {
			return err
		}

		req := commands.FleetReleaseRequest{
			DeviceID: fleetReleaseDeviceID,
			Token:    token,
		}

		response := commands.FleetReleaseCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(fleetCmd)
	fleetCmd.AddCommand(fleetAllocateCmd, fleetListCmd, fleetReleaseCmd)

	fleetAllocateCmd.Flags().StringVar(&platform, "platform", "", "device platform (ios or android)")
	_ = fleetAllocateCmd.MarkFlagRequired("platform")

	fleetReleaseCmd.Flags().StringVar(&fleetReleaseDeviceID, "device", "", "device ID to release")
	_ = fleetReleaseCmd.MarkFlagRequired("device")
}
