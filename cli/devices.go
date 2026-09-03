package cli

import (
	"github.com/mobile-next/mobilecli/devices"
	"github.com/spf13/cobra"
)

var (
	includeOfflineDevices bool
)

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List connected devices",
	Long:  `List all connected iOS and Android devices, both real devices and simulators/emulators.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := devices.DeviceListOptions{
			IncludeOffline: includeOfflineDevices,
			Platform:       platform,
			DeviceType:     deviceType,
		}

		return runViaDaemon("cli.devices", opts)
	},
}

func init() {
	rootCmd.AddCommand(devicesCmd)

	// devices command flags
	devicesCmd.Flags().StringVar(&platform, "platform", "", "target platform (ios or android)")
	devicesCmd.Flags().StringVar(&deviceType, "type", "", "filter by device type (real or simulator/emulator)")
	devicesCmd.Flags().BoolVar(&includeOfflineDevices, "include-offline", false, "include offline emulators and simulators")
}
