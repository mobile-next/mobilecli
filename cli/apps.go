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
			return fmt.Errorf("%s", response.Error)
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
			return fmt.Errorf("%s", response.Error)
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
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var appsInstallCmd = &cobra.Command{
	Use:   "install [path]",
	Short: "Install an app on a device",
	Long:  `Installs an app on the specified device from the given path (.apk for Android, .zip for iOS Simulator, and .ipa for iOS).`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.InstallAppRequest{
			DeviceID: deviceId,
			Path:     args[0],
		}

		response := commands.InstallAppCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var appsUninstallCmd = &cobra.Command{
	Use:   "uninstall [bundle_id]",
	Short: "Uninstall an app from a device",
	Long:  `Uninstalls an app from the specified device using its bundle ID.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.UninstallAppRequest{
			DeviceID:    deviceId,
			PackageName: args[0],
		}

		response := commands.UninstallAppCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var appsForegroundCmd = &cobra.Command{
	Use:   "foreground",
	Short: "Get the currently foreground app on a device",
	Long:  `Returns information about the app currently in the foreground on the specified device.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.ForegroundAppRequest{
			DeviceID: deviceId,
		}

		response := commands.ForegroundAppCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(appsCmd)

	appsCmd.AddCommand(appsLaunchCmd)
	appsCmd.AddCommand(appsTerminateCmd)
	appsCmd.AddCommand(appsListCmd)
	appsCmd.AddCommand(appsInstallCmd)
	appsCmd.AddCommand(appsUninstallCmd)
	appsCmd.AddCommand(appsForegroundCmd)

	appsLaunchCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to launch app on")
	appsTerminateCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to terminate app on")
	appsListCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to list apps from")
	appsInstallCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to install app on")
	appsUninstallCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to uninstall app from")
	appsForegroundCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to get foreground app from")
}
