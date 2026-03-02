package cli

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/utils"
	"github.com/spf13/cobra"
)

var (
	screencaptureScale float64
	screencaptureFPS   int
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
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

var screencaptureCmd = &cobra.Command{
	Use:   "screencapture",
	Short: "Stream screen capture from a connected device",
	Long:  `Streams screen capture from a specified device to stdout. Formats: mjpeg (all devices), avc (all devices via DeviceKit /h264), avc+replay-kit (real devices only, requires BroadcastExtension).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate format
		if screencaptureFormat != "mjpeg" && screencaptureFormat != "avc" && screencaptureFormat != "avc+replay-kit" {
			response := commands.NewErrorResponse(fmt.Errorf("format must be 'mjpeg', 'avc', or 'avc+replay-kit' for screen capture"))
			printJson(response)
			return fmt.Errorf("%s", response.Error)
		}

		// Find the target device
		targetDevice, err := commands.FindDeviceOrAutoSelect(deviceId)
		if err != nil {
			response := commands.NewErrorResponse(fmt.Errorf("error finding device: %v", err))
			printJson(response)
			return fmt.Errorf("%s", response.Error)
		}

		// Start agent
		err = targetDevice.StartAgent(devices.StartAgentConfig{
			OnProgress: func(message string) {
				utils.Verbose(message)
			},
			Hook: commands.GetShutdownHook(),
		})
		if err != nil {
			response := commands.NewErrorResponse(fmt.Errorf("error starting agent: %v", err))
			printJson(response)
			return fmt.Errorf("%s", response.Error)
		}

		// set defaults if not provided
		scale := screencaptureScale
		if scale == 0.0 {
			scale = devices.DefaultScale
		}

		fps := screencaptureFPS
		if fps == 0 {
			fps = devices.DefaultFramerate
		}

		// Start screen capture and stream to stdout
		err = targetDevice.StartScreenCapture(devices.ScreenCaptureConfig{
			Format:  screencaptureFormat,
			Quality: devices.DefaultQuality,
			Scale:   scale,
			FPS:     fps,
			OnProgress: func(message string) {
				utils.Verbose(message)
			},
			OnData: func(data []byte) bool {
				_, writeErr := os.Stdout.Write(data)
				if writeErr != nil {
					fmt.Fprintf(os.Stderr, "Error writing data: %v\n", writeErr)
					return false
				}
				return true
			},
		})

		if err != nil {
			response := commands.NewErrorResponse(fmt.Errorf("error starting screen capture: %v", err))
			printJson(response)
			return fmt.Errorf("%s", response.Error)
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
	screencaptureCmd.Flags().Float64Var(&screencaptureScale, "scale", 0, "Scale factor for screen capture (0 for default)")
	screencaptureCmd.Flags().IntVar(&screencaptureFPS, "fps", 0, "Frames per second for screen capture (0 for default)")
}
