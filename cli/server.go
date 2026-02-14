package cli

import (
	"fmt"

	"github.com/mobile-next/mobilecli/daemon"
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

		// GetBool/GetString cannot fail for defined flags
		enableCORS, _ := cmd.Flags().GetBool("cors")
		isDaemon, _ := cmd.Flags().GetBool("daemon")

		if isDaemon && !daemon.IsChild() {
			_, err := daemon.Daemonize()
			if err != nil {
				return fmt.Errorf("failed to start daemon: %w", err)
			}

			fmt.Printf("Server daemon spawned, attempting to listen on %s\n", listenAddr)
			return nil
		}

		return server.StartServer(listenAddr, enableCORS)
	},
}

var serverKillCmd = &cobra.Command{
	Use:   "kill",
	Short: "Stop the daemonized mobilecli server",
	Long:  `Connects to the server and sends a shutdown command via JSON-RPC.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// GetString cannot fail for defined flags
		addr, _ := cmd.Flags().GetString("listen")
		if addr == "" {
			addr = defaultServerAddress
		}

		err := daemon.KillServer(addr)
		if err != nil {
			return err
		}

		fmt.Printf("Server shutdown command sent successfully\n")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	// add server subcommands
	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverKillCmd)

	// server start flags
	serverStartCmd.Flags().String("listen", "", "Address to listen on (e.g., 'localhost:12000' or '0.0.0.0:13000')")
	serverStartCmd.Flags().Bool("cors", false, "Enable CORS support")
	serverStartCmd.Flags().BoolP("daemon", "d", false, "Run server in daemon mode (background)")

	// server kill flags
	serverKillCmd.Flags().String("listen", "", fmt.Sprintf("Address of server to kill (default: %s)", defaultServerAddress))
}
