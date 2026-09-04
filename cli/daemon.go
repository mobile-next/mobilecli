package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/mobile-next/mobilecli/daemon"
	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/server"
	"github.com/mobile-next/mobilecli/utils"
	"github.com/spf13/cobra"
)

var daemonIdleTimeout time.Duration

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the mobilecli daemon (background process that keeps devices connected)",
	Long: `The daemon keeps discovered devices, iOS tunnels and on-device agents alive
between commands. Device commands start it automatically; use these commands to
inspect or control it explicitly.`,
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon in the foreground",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDaemon()
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := daemon.Status()
		if errors.Is(err, daemon.ErrNotRunning) {
			printJson(map[string]any{"running": false})
			return nil
		}
		if err != nil {
			return err
		}
		printJson(map[string]any{
			"running":   true,
			"pid":       st.PID,
			"version":   st.Version,
			"uptime":    st.Uptime,
			"startedAt": st.StartedAt,
			"socket":    st.Socket,
		})
		return nil
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the daemon",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := daemon.Shutdown()
		if errors.Is(err, daemon.ErrNotRunning) {
			printJson(map[string]any{"running": false})
			return nil
		}
		if err != nil {
			return err
		}
		printJson(commands.OK)
		return nil
	},
}

// runDaemon is the daemon process body: it owns device state and serves the
// unix socket until stopped, idle, or signalled.
func runDaemon() error {
	paths, err := daemon.Paths()
	if err != nil {
		return err
	}

	hook := devices.NewShutdownHook()
	commands.SetShutdownHook(hook)
	if token, _ := getRemoteToken(); token != "" {
		commands.SetFleetConfig(token)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	utils.Info("daemon %s listening on %s (pid %d)", utils.Version, paths.Socket, os.Getpid())
	err = daemon.Run(ctx, daemon.Options{
		Paths:       paths,
		Version:     utils.Version,
		IdleTimeout: daemonIdleTimeout,
		Dispatch:    server.DaemonDispatch,
		Busy:        server.DaemonBusy,
		OnShutdown: func() {
			server.StopRecordingForShutdown()
			if hookErr := hook.Shutdown(); hookErr != nil {
				utils.Info("shutdown hook error: %v", hookErr)
			}
		},
	})
	if err != nil {
		return fmt.Errorf("daemon: %w", err)
	}
	return nil
}

// stopDaemonIfRunning is used by auth commands: the daemon caches the fleet
// token, so a login/logout must not leave a stale one behind.
func stopDaemonIfRunning() {
	if err := daemon.Shutdown(); err != nil && !errors.Is(err, daemon.ErrNotRunning) {
		fmt.Fprintf(os.Stderr, "warning: could not stop daemon: %v\n", err)
	}
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonStartCmd, daemonStatusCmd, daemonStopCmd)
	daemonStartCmd.Flags().DurationVar(&daemonIdleTimeout, "idle-timeout", defaultDaemonIdleTimeout, "exit after this long without requests (0 = never)")
}
