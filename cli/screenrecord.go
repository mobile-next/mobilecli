package cli

import (
	"fmt"
	"os"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var (
	screenrecordFormat    string
	screenrecordOutput    string
	screenrecordTimeLimit int
)

var screenrecordCmd = &cobra.Command{
	Use:   "screenrecord",
	Short: "Record device screen to an MP4 file or stream raw AVC",
	Long:  `Records the screen of a connected iOS real device. Supports MP4 (file) and AVC (raw H.264 stream to stdout).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if screenrecordFormat != "mp4" && screenrecordFormat != "avc" {
			return fmt.Errorf("format must be 'mp4' or 'avc'")
		}

		if screenrecordFormat == "mp4" && screenrecordOutput == "" {
			return fmt.Errorf("--output is required for mp4 format")
		}

		req := commands.ScreenRecordRequest{
			DeviceID:   deviceId,
			Format:     screenrecordFormat,
			OutputPath: screenrecordOutput,
			TimeLimit:  screenrecordTimeLimit,
		}

		response := commands.ScreenRecordCommand(req, func(data []byte) bool {
			_, err := os.Stdout.Write(data)
			return err == nil
		})

		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(screenrecordCmd)

	screenrecordCmd.Flags().StringVarP(&screenrecordFormat, "format", "f", "mp4", "Output format (mp4 or avc)")
	screenrecordCmd.Flags().StringVarP(&screenrecordOutput, "output", "o", "", "Output file path for mp4 format")
	screenrecordCmd.Flags().IntVar(&screenrecordTimeLimit, "time-limit", 0, "Max recording duration in seconds (0 = no limit)")
}
