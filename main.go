package main

import (
	"fmt"
	"os"

	"github.com/mobile-next/mobilecli/internal/cli"
)

func main() {
	// signal handling is per command: the daemon shuts down gracefully on
	// SIGINT/SIGTERM, and streaming commands cancel their daemon call.
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
