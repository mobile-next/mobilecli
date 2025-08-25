package cli

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var screenshotCmd = &cobra.Command{
	Use:   "screenshot",
	Short: "Take a screenshot of a connected device",
	Long:  `Takes a screenshot of a specified device (using its ID) and saves it locally as a PNG file. Supports iOS (real/simulator) and Android (real/emulator).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.ScreenshotRequest{
			DeviceID:   deviceId,
			Format:     screenshotFormat,
			Quality:    screenshotJpegQuality,
			OutputPath: screenshotOutputPath,
		}

		response := commands.ScreenshotCommand(req)

		// Handle stdout output for binary data
		if screenshotOutputPath == "-" && response.Status == "ok" {
			if screenshotResp, ok := response.Data.(commands.ScreenshotResponse); ok && screenshotResp.Data != "" {
				// Write binary data to stdout
				imageBytes, err := base64.StdEncoding.DecodeString(screenshotResp.Data)
				if err != nil {
					return fmt.Errorf("failed to decode image data: %v", err)
				}
				_, err = os.Stdout.Write(imageBytes)
				if err != nil {
					return fmt.Errorf("failed to write to stdout: %v", err)
				}
				return nil
			}
		}

		// Print JSON response
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf(response.Error)
		}
		return nil
	},
}

var screencaptureCmd = &cobra.Command{
	Use:   "screencapture",
	Short: "Stream screen capture from a connected device",
	Long:  `Streams MJPEG screen capture from a specified device to stdout. Only supports MJPEG format.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate format
		if screencaptureFormat != "mjpeg" {
			response := commands.NewErrorResponse(fmt.Errorf("format must be 'mjpeg' for screen capture"))
			printJson(response)
			return fmt.Errorf(response.Error)
		}

		// Find the target device
		targetDevice, err := commands.FindDeviceOrAutoSelect(deviceId)
		if err != nil {
			response := commands.NewErrorResponse(fmt.Errorf("error finding device: %v", err))
			printJson(response)
			return fmt.Errorf(response.Error)
		}

		// Start agent
		err = targetDevice.StartAgent()
		if err != nil {
			response := commands.NewErrorResponse(fmt.Errorf("error starting agent: %v", err))
			printJson(response)
			return fmt.Errorf(response.Error)
		}

		// Start screen capture and stream to stdout
		err = targetDevice.StartScreenCapture(screencaptureFormat, func(data []byte) bool {
			_, writeErr := os.Stdout.Write(data)
			if writeErr != nil {
				fmt.Fprintf(os.Stderr, "Error writing data: %v\n", writeErr)
				return false
			}
			return true
		})

		if err != nil {
			response := commands.NewErrorResponse(fmt.Errorf("error starting screen capture: %v", err))
			printJson(response)
			return fmt.Errorf(response.Error)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(screenshotCmd)
	rootCmd.AddCommand(screencaptureCmd)

	// screenshot command flags
	screenshotCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to take screenshot from")
	screenshotCmd.Flags().StringVarP(&screenshotOutputPath, "output", "o", "", "Output file path for screenshot (e.g., screen.png, or '-' for stdout)")
	screenshotCmd.Flags().StringVarP(&screenshotFormat, "format", "f", "png", "Output format for screenshot (png or jpeg)")
	screenshotCmd.Flags().IntVarP(&screenshotJpegQuality, "quality", "q", 90, "JPEG quality (1-100, only applies if format is jpeg)")

	// screencapture command flags
	screencaptureCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to capture from")
	screencaptureCmd.Flags().StringVarP(&screencaptureFormat, "format", "f", "mjpeg", "Output format for screen capture")
}
