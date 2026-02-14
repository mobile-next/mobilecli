package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mobile-next/mobilecli/cli"
	"github.com/mobile-next/mobilecli/commands"
	"github.com/mobile-next/mobilecli/daemon"
	"github.com/mobile-next/mobilecli/devices"
)

func main() {
	// daemon child sets up its own signal handling in server.StartServer
	if daemon.IsChild() {
		if err := cli.Execute(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// create shutdown hook for cleanup tracking
	hook := devices.NewShutdownHook()
	commands.SetShutdownHook(hook)

	// setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// run command in goroutine
	done := make(chan error, 1)
	go func() {
		done <- cli.Execute()
	}()

	// wait for command completion or signal
	select {
	case <-sigChan:
		// cleanup resources on signal
		hook.Shutdown()
		os.Exit(0)
	case err := <-done:
		// cleanup resources on normal exit
		hook.Shutdown()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
