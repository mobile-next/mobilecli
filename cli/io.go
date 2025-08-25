package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var ioCmd = &cobra.Command{
	Use:   "io",
	Short: "Input/output operations with devices",
	Long:  `Perform input/output operations like tapping, pressing buttons, and sending text to devices.`,
}

var ioTapCmd = &cobra.Command{
	Use:   "tap [x,y]",
	Short: "Tap on a device screen at the given coordinates",
	Long:  `Sends a tap event to the specified device at the given x,y coordinates. Coordinates should be provided as a single string "x,y".`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		coordsStr := args[0]
		parts := strings.Split(coordsStr, ",")
		if len(parts) != 2 {
			response := commands.NewErrorResponse(fmt.Errorf("invalid coordinate format. Expected 'x,y', got '%s'", coordsStr))
			printJson(response)
			return fmt.Errorf(response.Error)
		}

		x, errX := strconv.Atoi(strings.TrimSpace(parts[0]))
		y, errY := strconv.Atoi(strings.TrimSpace(parts[1]))

		if errX != nil || errY != nil {
			response := commands.NewErrorResponse(fmt.Errorf("invalid coordinate values. x and y must be integers. Got x='%s', y='%s'", parts[0], parts[1]))
			printJson(response)
			return fmt.Errorf(response.Error)
		}

		req := commands.TapRequest{
			DeviceID: deviceId,
			X:        x,
			Y:        y,
		}

		response := commands.TapCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

var ioButtonCmd = &cobra.Command{
	Use:   "button [button_name]",
	Short: "Press a hardware button on a device",
	Long:  `Sends a hardware button press event to the specified device (e.g., "HOME", "VOLUME_UP", "VOLUME_DOWN", "POWER"). Button names are case-insensitive.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.ButtonRequest{
			DeviceID: deviceId,
			Button:   args[0],
		}

		response := commands.ButtonCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

var ioTextCmd = &cobra.Command{
	Use:   "text [text]",
	Short: "Send text input to a device",
	Long:  `Sends text input to the currently focused element on the specified device.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.TextRequest{
			DeviceID: deviceId,
			Text:     args[0],
		}

		response := commands.TextCommand(req)
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(ioCmd)

	// add io subcommands
	ioCmd.AddCommand(ioTapCmd)
	ioCmd.AddCommand(ioButtonCmd)
	ioCmd.AddCommand(ioTextCmd)

	// io command flags
	ioTapCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to tap on")
	ioButtonCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to press button on")
	ioTextCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to send keys to")
}
