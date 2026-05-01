package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/mobile-next/mobilecli/utils"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
)

func getRemoteToken() (string, error) {
	if token := os.Getenv("MOBILECLI_TOKEN"); token != "" {
		return token, nil
	}

	token, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", fmt.Errorf("not logged in, run 'mobilecli auth login' first")
		}
		return "", fmt.Errorf("failed to get token from keyring: %w", err)
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
			result, ok := response.Data.(commands.FleetAllocateResponse)
			if !ok {
				printJson(response)
				return fmt.Errorf("unexpected response format")
			}

			if result.IsAllocating() {
				utils.Verbose("waiting for device allocation, session %s (0 seconds elapsed)", result.SessionID)
				start := time.Now()
				deadline := start.Add(time.Duration(fleetTimeout) * time.Second)
				for {
					if time.Now().After(deadline) {
						err := fmt.Errorf("timed out waiting for device allocation after %d seconds (session %s)", fleetTimeout, result.SessionID)
						printJson(commands.NewErrorResponse(err))
						return err
					}
					time.Sleep(5 * time.Second)
					elapsed := int(time.Since(start).Seconds())
					utils.Verbose("waiting for device allocation, session %s (%d seconds elapsed)", result.SessionID, elapsed)
					device, err := commands.FleetGetDeviceBySession(token, result.SessionID)
					if err != nil {
						err = fmt.Errorf("failed to check device status (session %s): %w", result.SessionID, err)
						printJson(commands.NewErrorResponse(err))
						return err
					}
					if device.State != "allocating" {
						response = commands.NewSuccessResponse(commands.FleetAllocateResponse{
							SessionID:   result.SessionID,
							ProvisionID: result.ProvisionID,
							State:       device.State,
							Device: commands.FleetAllocateDevice{
								ID:       device.ID,
								Name:     device.Name,
								Platform: device.Platform,
								Status:   device.State,
								Model:    device.Model,
							},
						})
						break
					}
				}
			}
		}

		printJson(response)
		return nil
	},
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
