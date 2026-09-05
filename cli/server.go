package cli

import (
	"github.com/mobile-next/mobilecli/server"
	"github.com/spf13/cobra"
)

const defaultServerAddress = "localhost:12000"

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Server management commands",
	Long:  `Commands for managing the mobilecli server.`,
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the mobilecli server",
	Long:  `Starts the mobilecli server.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		listenAddr := cmd.Flag("listen").Value.String()
		if listenAddr == "" {
			listenAddr = defaultServerAddress
		}

		// GetBool cannot fail for defined flags
		enableCORS, _ := cmd.Flags().GetBool("cors")

		// the server is a front for the daemon, which owns the devices
		if err := ensureDaemon(); err != nil {
			return err
		}
		// the daemon may exit on idle while the server keeps running
		server.SetDaemonEnsurer(ensureDaemon)
		return server.StartServer(listenAddr, enableCORS)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.AddCommand(serverStartCmd)

	serverStartCmd.Flags().String("listen", "", "Address to listen on (e.g., 'localhost:12000' or '0.0.0.0:13000')")
	serverStartCmd.Flags().Bool("cors", false, "Enable CORS support")
}
