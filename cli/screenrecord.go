package cli

import (
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var (
	screenrecordOutput    string
	screenrecordTimeLimit int
)

var screenrecordCmd = &cobra.Command{
	Use:   "screenrecord",
	Short: "Record device screen to an MP4 file",
	Long:  `Records the screen of a connected device (iOS, Android, or simulator) to an MP4 file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if screenrecordOutput == "" {
			return fmt.Errorf("--output is required")
		}

		req := commands.ScreenRecordRequest{
			DeviceID:   deviceId,
			OutputPath: screenrecordOutput,
			TimeLimit:  screenrecordTimeLimit,
		}

		response := commands.ScreenRecordCommand(req)

		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(screenrecordCmd)

	screenrecordCmd.Flags().StringVarP(&screenrecordOutput, "output", "o", "", "Output MP4 file path")
	screenrecordCmd.Flags().IntVar(&screenrecordTimeLimit, "time-limit", 0, "Max recording duration in seconds (0 = no limit)")
}
