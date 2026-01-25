package cli

import (
	"fmt"
	"os"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/utils"
	"github.com/spf13/cobra"
)

var audiocaptureCmd = &cobra.Command{
	Use:   "audiocapture",
	Short: "Stream audio capture from a connected device",
	Long:  "Streams audio capture from a specified device to stdout. Supports Opus (real iOS devices only).",
	RunE: func(cmd *cobra.Command, args []string) error {
		if audiocaptureFormat != "opus+rtp" && audiocaptureFormat != "opus+ogg" {
			response := commands.NewErrorResponse(fmt.Errorf("format must be 'opus+rtp' or 'opus+ogg' for audio capture"))
			printJson(response)
			return fmt.Errorf("%s", response.Error)
		}

		targetDevice, err := commands.FindDeviceOrAutoSelect(deviceId)
		if err != nil {
			response := commands.NewErrorResponse(fmt.Errorf("error finding device: %v", err))
			printJson(response)
			return fmt.Errorf("%s", response.Error)
		}

		if targetDevice.Platform() != "ios" || targetDevice.DeviceType() != "real" {
			response := commands.NewErrorResponse(fmt.Errorf("audio capture is only supported on real iOS devices"))
			printJson(response)
			return fmt.Errorf("%s", response.Error)
		}

		var parser *utils.OpusFrameParser
		var oggWriter *utils.OggOpusWriter
		if audiocaptureFormat == "opus+ogg" {
			var err error
			oggWriter, err = utils.NewOggOpusWriter(os.Stdout)
			if err != nil {
				response := commands.NewErrorResponse(fmt.Errorf("failed to initialize ogg writer: %v", err))
				printJson(response)
				return fmt.Errorf("%s", response.Error)
			}
			parser = utils.NewOpusFrameParser(func(packet []byte) error {
				return oggWriter.WritePacket(packet)
			})
		}

		err = targetDevice.StartAudioCapture(devices.AudioCaptureConfig{
			Format: audiocaptureFormat,
			OnProgress: func(message string) {
				utils.Verbose(message)
			},
			OnData: func(data []byte) bool {
				if parser != nil {
					if err := parser.Write(data); err != nil {
						fmt.Fprintf(os.Stderr, "Error writing Ogg Opus data: %v\n", err)
						return false
					}
				} else {
					_, writeErr := os.Stdout.Write(data)
					if writeErr != nil {
						fmt.Fprintf(os.Stderr, "Error writing data: %v\n", writeErr)
						return false
					}
				}
				return true
			},
		})

		if err != nil {
			response := commands.NewErrorResponse(fmt.Errorf("error starting audio capture: %v", err))
			printJson(response)
			return fmt.Errorf("%s", response.Error)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(audiocaptureCmd)

	audiocaptureCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to capture from")
	audiocaptureCmd.Flags().StringVarP(&audiocaptureFormat, "format", "f", "opus+rtp", "Output format for audio capture (opus+rtp, opus+ogg)")
}
