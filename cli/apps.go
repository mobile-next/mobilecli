package cli

import (
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var appsCmd = &cobra.Command{
	Use:   "apps",
	Short: "Manage applications on devices",
	Long:  `Launch, terminate, and manage applications on connected devices.`,
}

var appsLaunchCmd = &cobra.Command{
	Use:   "launch [bundle_id]",
	Short: "Launch an app on a device",
	Long:  `Launches an app on the specified device using its bundle ID (e.g., "com.example.app").`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.AppRequest{
			DeviceID: deviceId,
			BundleID: args[0],
		}

		response := commands.LaunchAppCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

var appsTerminateCmd = &cobra.Command{
	Use:   "terminate [bundle_id]",
	Short: "Terminate an app on a device",
	Long:  `Terminates an app on the specified device using its bundle ID (e.g., "com.example.app").`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.AppRequest{
			DeviceID: deviceId,
			BundleID: args[0],
		}

		response := commands.TerminateAppCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

var appsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed apps on a device",
	Long:  `Lists all applications installed on the specified device.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.ListAppsRequest{
			DeviceID: deviceId,
		}

		response := commands.ListAppsCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(appsCmd)

	// add apps subcommands
	appsCmd.AddCommand(appsLaunchCmd)
	appsCmd.AddCommand(appsTerminateCmd)
	appsCmd.AddCommand(appsListCmd)

	// apps command flags
	appsLaunchCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to launch app on")
	appsTerminateCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to terminate app on")
	appsListCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to list apps from")
}
