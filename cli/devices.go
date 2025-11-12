package cli

import (
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var (
	showAllDevices bool
)

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List connected devices",
	Long:  `List all connected iOS and Android devices, both real devices and simulators/emulators.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.DevicesCommand(showAllDevices)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(devicesCmd)

	// devices command flags
	devicesCmd.Flags().StringVar(&platform, "platform", "", "target platform (ios or android)")
	devicesCmd.Flags().StringVar(&deviceType, "type", "", "filter by device type (real or simulator/emulator)")
	devicesCmd.Flags().BoolVar(&showAllDevices, "all", false, "show all devices including offline ones")
}
