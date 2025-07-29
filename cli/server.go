package cli

import (
	"github.com/mobile-next/mobilecli/server"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Server management commands",
	Long:  `Commands for managing the mobilecli server.`,
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the mobilecli server",
	Long:  `Starts the mobilecli server.`,
	Args:  cobra.NoArgs, // No arguments allowed after "start"
	RunE: func(cmd *cobra.Command, args []string) error {
		listenAddr := cmd.Flag("listen").Value.String()
		if listenAddr == "" {
			listenAddr = "localhost:12000"
		}

		enableCORS, _ := cmd.Flags().GetBool("cors")
		return server.StartServer(listenAddr, enableCORS)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	
	// add server subcommands
	serverCmd.AddCommand(serverStartCmd)
	
	// server command flags
	serverStartCmd.Flags().String("listen", "", "Address to listen on (e.g., 'localhost:12000' or '0.0.0.0:13000')")
	serverStartCmd.Flags().Bool("cors", false, "Enable CORS support")
}
