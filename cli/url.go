package cli

import (
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var urlCmd = &cobra.Command{
	Use:   "url [url]",
	Short: "Open a URL on a device",
	Long:  `Opens a URL in the default browser on the specified device`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.URLRequest{
			DeviceID: deviceId,
			URL:      args[0],
		}

		response := commands.URLCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(urlCmd)
	
	// url command flags
	urlCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to open URL on")
}