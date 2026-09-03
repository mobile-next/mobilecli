package cli

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/mobile-next/mobilecli/daemon"
	"github.com/mobile-next/mobilecli/server"
	"github.com/mobile-next/mobilecli/types"
	"github.com/spf13/cobra"
)

var (
	screenshotScale      float64
	screenshotMaxSize    int
	screenshotClip       string
	screencaptureScale   float64
	screencaptureFPS     int
	screencaptureBitrate int
)

const (
	minScreencaptureBitrate = 100_000
	maxScreencaptureBitrate = 10_000_000
)

// parseScreenshotClip parses an "x,y,width,height" flag value (in screen
// points); an empty value means no cropping.
func parseScreenshotClip(value string) (*types.ScreenElementRect, error) {
	if value == "" {
		return nil, nil
	}

	parts := strings.Split(value, ",")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid --clip %q, expected x,y,width,height", value)
	}

	numbers := make([]int, 4)
	for i, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid --clip %q, expected x,y,width,height", value)
		}
		numbers[i] = number
	}

	return &types.ScreenElementRect{X: numbers[0], Y: numbers[1], Width: numbers[2], Height: numbers[3]}, nil
}

var screenshotCmd = &cobra.Command{
	Use:   "screenshot",
	Short: "Take a screenshot of a connected device",
	Long:  `Takes a screenshot of a specified device (using its ID) and saves it locally as a PNG file. Supports iOS (real/simulator) and Android (real/emulator).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rect, err := parseScreenshotClip(screenshotClip)
		if err != nil {
			return err
		}

		// the daemon's cwd is not ours: send an absolute path, or our cwd as the
		// directory for the generated default filename
		outputPath := absLocalPath(screenshotOutputPath)
		if outputPath == "" {
			outputPath = cwdWithSeparator()
		}

		req := commands.ScreenshotRequest{
			DeviceID:   deviceId,
			Format:     screenshotFormat,
			Quality:    screenshotJpegQuality,
			Scale:      screenshotScale,
			MaxSize:    screenshotMaxSize,
			Clip:       rect,
			OutputPath: outputPath,
		}

		if outputPath != "-" {
			return runViaDaemon("cli.screenshot", req)
		}

		raw, err := callDaemon("cli.screenshot", req, daemon.NoTimeout)
		if err != nil {
			printJson(errorEnvelope(err))
			return err
		}

		var screenshotResp commands.ScreenshotResponse
		if err := decodeEnvelope(raw, &screenshotResp); err != nil {
			out, _ := indentedJSON(raw)
			fmt.Println(out)
			return err
		}

		// Write binary data to stdout
		imageBytes, err := base64.StdEncoding.DecodeString(screenshotResp.Data)
		if err != nil {
			return fmt.Errorf("failed to decode image data: %v", err)
		}
		if _, err := os.Stdout.Write(imageBytes); err != nil {
			return fmt.Errorf("failed to write to stdout: %v", err)
		}
		return nil
	},
}

var screencaptureCmd = &cobra.Command{
	Use:   "screencapture",
	Short: "Stream screen capture from a connected device",
	Long:  `Streams screen capture from a specified device to stdout. Supports MJPEG (all devices) and AVC (Android and iOS real devices).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate format
		if screencaptureFormat != "mjpeg" && screencaptureFormat != "avc" {
			response := commands.NewErrorResponse(fmt.Errorf("format must be 'mjpeg' or 'avc' for screen capture"))
			printJson(response)
			return fmt.Errorf("%s", response.Error)
		}

		// Validate bitrate (0 means use default; ignored for anything but AVC)
		if screencaptureFormat == "avc" && screencaptureBitrate != 0 && (screencaptureBitrate < minScreencaptureBitrate || screencaptureBitrate > maxScreencaptureBitrate) {
			response := commands.NewErrorResponse(fmt.Errorf("bitrate must be between %d and %d", minScreencaptureBitrate, maxScreencaptureBitrate))
			printJson(response)
			return fmt.Errorf("%s", response.Error)
		}

		return streamViaDaemon("cli.screencapture", server.ScreenCaptureStreamRequest{
			DeviceID: deviceId,
			Format:   screencaptureFormat,
			Scale:    screencaptureScale,
			FPS:      screencaptureFPS,
			Bitrate:  screencaptureBitrate,
		})
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
	screenshotCmd.Flags().Float64Var(&screenshotScale, "scale", 1.0, "Scale factor for screenshot (0.0-1.0)")
	screenshotCmd.Flags().IntVar(&screenshotMaxSize, "max-size", 0, "Maximum of width/height in pixels, keeping aspect ratio (takes precedence over --scale, 0 for no limit)")
	screenshotCmd.Flags().StringVar(&screenshotClip, "clip", "", "Crop to x,y,width,height in screen points, applied before --scale/--max-size")

	// screencapture command flags
	screencaptureCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to capture from")
	screencaptureCmd.Flags().StringVarP(&screencaptureFormat, "format", "f", "mjpeg", "Output format for screen capture")
	screencaptureCmd.Flags().Float64Var(&screencaptureScale, "scale", 0, "Scale factor for screen capture (0 for default)")
	screencaptureCmd.Flags().IntVar(&screencaptureFPS, "fps", 0, "Frames per second for screen capture (0 for default)")
	screencaptureCmd.Flags().IntVar(&screencaptureBitrate, "bitrate", 0, "Bitrate in bits per second for AVC capture (100000-10000000, 0 for default)")
}
