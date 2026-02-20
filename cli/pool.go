package cli

import (
	"fmt"
	"os"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
)

func getPoolToken() (string, error) {
	if token := os.Getenv("MOBILECLI_TOKEN"); token != "" {
		return token, nil
	}

	token, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		return "", fmt.Errorf("not logged in, run 'mobilecli auth login' first")
	}

	return token, nil
}

var poolCmd = &cobra.Command{
	Use:   "pool",
	Short: "Device pool management commands",
	Long:  `Commands for managing device pool including allocating, listing, and releasing devices.`,
}

var poolAllocateCmd = &cobra.Command{
	Use:   "allocate",
	Short: "Allocate a device from the pool",
	Long:  `Allocates a device from the pool for the specified platform (ios or android).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if platform != "ios" && platform != "android" {
			return fmt.Errorf("platform must be 'ios' or 'android'")
		}

		token, err := getPoolToken()
		if err != nil {
			return err
		}

		req := commands.PoolAllocateRequest{
			Platform: platform,
			Token:    token,
		}

		response := commands.PoolAllocateCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}

		return nil
	},
}

var poolListCmd = &cobra.Command{
	Use:   "list",
	Short: "List allocated devices",
	Long:  `Lists all devices currently allocated from the pool.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getPoolToken()
		if err != nil {
			return err
		}

		req := commands.PoolListRequest{
			Token: token,
		}

		response := commands.PoolListCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}

		return nil
	},
}

var poolReleaseDeviceID string

var poolReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Release a device back to the pool",
	Long:  `Releases an allocated device back to the pool.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := getPoolToken()
		if err != nil {
			return err
		}

		req := commands.PoolReleaseRequest{
			DeviceID: poolReleaseDeviceID,
			Token:    token,
		}

		response := commands.PoolReleaseCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(poolCmd)
	poolCmd.AddCommand(poolAllocateCmd, poolListCmd, poolReleaseCmd)

	poolAllocateCmd.Flags().StringVar(&platform, "platform", "", "device platform (ios or android)")
	_ = poolAllocateCmd.MarkFlagRequired("platform")

	poolReleaseCmd.Flags().StringVar(&poolReleaseDeviceID, "device", "", "device ID to release")
	_ = poolReleaseCmd.MarkFlagRequired("device")
}
