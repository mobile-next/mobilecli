package cli

import (
	"fmt"
	"github.com/mobile-next/mobilecli/internal/commands"

	"github.com/spf13/cobra"
)

var (
	screenrecordOutput    string
	screenrecordTimeLimit int
	screenrecordSilent    bool
)

var screenrecordCmd = &cobra.Command{
	Use:   "screenrecord",
	Short: "Record device screen to an MP4 file",
	Long:  `Records the screen of a connected device (iOS, Android, or simulator) to an MP4 file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// the daemon's cwd is not ours: send an absolute path, or our cwd as the
		// directory for the generated default filename
		outputPath := absLocalPath(screenrecordOutput)
		if outputPath == "" {
			outputPath = cwdWithSeparator()
		}

		raw, err := streamResultViaDaemon("cli.screenrecord", commands.ScreenRecordStreamRequest{
			DeviceID:   deviceId,
			OutputPath: outputPath,
			TimeLimit:  screenrecordTimeLimit,
			Silent:     screenrecordSilent,
		})
		if err != nil || raw == nil {
			return err
		}

		out, err := indentedJSON(raw)
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(screenrecordCmd)

	screenrecordCmd.Flags().StringVarP(&screenrecordOutput, "output", "o", "", "Output MP4 file path (default: screenrecord-<device>-<timestamp>.mp4 in the current directory)")
	screenrecordCmd.Flags().IntVar(&screenrecordTimeLimit, "time-limit", 0, "Max recording duration in seconds (0 = no limit)")
	screenrecordCmd.Flags().BoolVar(&screenrecordSilent, "silent", false, "Suppress progress output")
}
