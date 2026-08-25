package cli

import (
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var clipboardCmd = &cobra.Command{
	Use:   "clipboard",
	Short: "Clipboard commands",
	Long:  `Commands for reading and writing the device clipboard.`,
}

var clipboardGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Read the clipboard of a device",
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.ClipboardGetCommand(commands.ClipboardGetRequest{
			DeviceID: deviceId,
		})

		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var clipboardSetCmd = &cobra.Command{
	Use:   "set [text]",
	Short: "Replace the clipboard of a device",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.ClipboardSetCommand(commands.ClipboardSetRequest{
			DeviceID: deviceId,
			Text:     args[0],
		})

		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

func init() {
	ioCmd.AddCommand(clipboardCmd)
	clipboardCmd.AddCommand(clipboardGetCmd)
	clipboardCmd.AddCommand(clipboardSetCmd)

	clipboardGetCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to read the clipboard from")
	clipboardSetCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to write the clipboard on")
}
