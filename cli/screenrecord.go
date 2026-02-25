package cli

import (
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var screenrecordCmd = &cobra.Command{
	Use:   "screenrecord",
	Short: "Record video from a connected device",
	Long:  `Records video from a specified device and saves it as an MP4 file. Supports iOS (real/simulator) and Android (real/emulator).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := commands.ScreenRecordRequest{
			DeviceID:   deviceId,
			BitRate:    screenrecordBitRate,
			TimeLimit:  screenrecordTimeLimit,
			OutputPath: screenrecordOutput,
		}

		response := commands.ScreenRecordCommand(req)

		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(screenrecordCmd)

	screenrecordCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to record from")
	screenrecordCmd.Flags().StringVar(&screenrecordBitRate, "bit-rate", "8M", "Video bit rate (e.g., 4M, 500K, 8000000)")
	screenrecordCmd.Flags().IntVar(&screenrecordTimeLimit, "time-limit", 300, "Maximum recording time in seconds (max 300)")
	screenrecordCmd.Flags().StringVarP(&screenrecordOutput, "output", "o", "", "Output file path (default: screenrecord-<deviceID>-<timestamp>.mp4)")
}
