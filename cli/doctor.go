package cli

import (
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run system diagnostics",
	Long:  `Performs system diagnostics for better troubleshooting`,
	RunE: func(cmd *cobra.Command, args []string) error {
		response := commands.DoctorCommand(GetVersion())
		printJson(response)
		if response.Status == "error" {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
