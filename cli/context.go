package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// cmdContext is cancelled on Ctrl-C / SIGTERM so an in-flight daemon call is
// abandoned (and cancelled daemon-side) instead of the process hanging.
func cmdContext() context.Context {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	return ctx
}
