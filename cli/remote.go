package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/mobile-next/mobilecli/utils"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
)

func getRemoteToken() (string, error) {
	token, err := loadToken()
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", fmt.Errorf("not logged in, run 'mobilecli auth login' first")
		}
		return "", err
	}

	return token, nil
}

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Remote device management commands",
	Long:  `Commands for managing remote devices including allocating, listing, and releasing devices.`,
}

var remoteAllocateCmd = &cobra.Command{
	Use:   "allocate",
	Short: "Allocate a remote device",
	Long: `Allocates a device from the remote fleet matching the given filters.

Flags --version and --name can be specified multiple times (all are ANDed).

Version supports comparison operators:
  --version ">=18"    (greater than or equal)
  --version "<20"     (less than)
  --version 18.6.2    (exact match)

Name supports wildcard prefix matching:
  --name "iPhone*"    (starts with)
  --name "iPhone 16"  (exact match)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if platform != "ios" && platform != "android" {
			return fmt.Errorf("platform must be 'ios' or 'android'")
		}

		token, err := getRemoteToken()
		if err != nil {
			return err
		}

		filters, err := buildAllocateFilters(platform, fleetType, fleetVersions, fleetNames)
		if err != nil {
			return err
		}

		req := commands.FleetAllocateRequest{
			Filters: filters,
			Token:   token,
		}

		response := commands.FleetAllocateCommand(req)
		if response.Status == "error" {
			printJson(response)
			return fmt.Errorf("%s", response.Error)
		}

		if fleetWait {
			response, err = waitForAllocation(token, response, fleetTimeout)
			if err != nil {
				printJson(commands.NewErrorResponse(err))
				return err
			}
		}

		printJson(response)
		return nil
	},
}

// polls devices.list until the allocation leaves the "allocating" state, and returns
// a response with the allocated device filled in
func waitForAllocation(token string, response *commands.CommandResponse, timeoutSeconds int) (*commands.CommandResponse, error) {
	result, ok := response.Data.(commands.FleetAllocateResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	if !result.IsAllocating() {
		return response, nil
	}

	start := time.Now()
	deadline := start.Add(time.Duration(timeoutSeconds) * time.Second)
	utils.Verbose("waiting for device allocation %s (0 seconds elapsed)", result.AllocationID)

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for device allocation after %d seconds (allocation %s)", timeoutSeconds, result.AllocationID)
		}

		time.Sleep(5 * time.Second)
		elapsed := int(time.Since(start).Seconds())
		utils.Verbose("waiting for device allocation %s (%d seconds elapsed)", result.AllocationID, elapsed)

		device, err := commands.FleetGetDeviceByAllocation(token, result.AllocationID)
		if err != nil {
			return nil, fmt.Errorf("failed to check device status (allocation %s): %w", result.AllocationID, err)
		}

		if device.State != "allocating" {
			return commands.NewSuccessResponse(commands.FleetAllocateResponse{
				AllocationID: result.AllocationID,
				State:        device.State,
				Device:       device,
			}), nil
		}
	}
}

var remoteListDevicesCmd = &cobra.Command{
	Use:   "list-devices",
	Short: "List available remote devices",
	Long:  `Lists available devices in the remote fleet.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getRemoteToken()
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

var remoteReleaseDeviceID string

var remoteReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Release a remote device",
	Long:  `Releases an allocated device back to the remote fleet.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getRemoteToken()
		if err != nil {
			return err
		}

		req := commands.FleetReleaseRequest{
			DeviceID: remoteReleaseDeviceID,
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
	rootCmd.AddCommand(remoteCmd)
	remoteCmd.AddCommand(remoteAllocateCmd, remoteListDevicesCmd, remoteReleaseCmd)

	remoteAllocateCmd.Flags().StringVar(&platform, "platform", "", "device platform (ios or android)")
	_ = remoteAllocateCmd.MarkFlagRequired("platform")
	remoteAllocateCmd.Flags().StringVar(&fleetType, "type", "", "device type (real)")
	remoteAllocateCmd.Flags().StringArrayVar(&fleetVersions, "version", nil, "OS version filter (supports >=, >, <=, < prefixes)")
	remoteAllocateCmd.Flags().StringArrayVar(&fleetNames, "name", nil, "device name filter (supports trailing * for prefix match)")
	remoteAllocateCmd.Flags().BoolVar(&fleetWait, "wait", false, "wait for device to finish allocating before returning")
	remoteAllocateCmd.Flags().IntVar(&fleetTimeout, "timeout", 900, "seconds to wait for allocation (only used with --wait)")

	remoteReleaseCmd.Flags().StringVar(&remoteReleaseDeviceID, "device", "", "device ID to release")
	_ = remoteReleaseCmd.MarkFlagRequired("device")
}
