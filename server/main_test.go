package server

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mobile-next/mobilecli/daemon"
)

// TestMain runs an in-process daemon for the whole package: the HTTP front
// forwards every device method to it, so tests need one listening.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("/tmp", "mcli-srv")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv(daemon.HomeEnv, dir)
	paths, err := daemon.Paths()
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- daemon.Run(ctx, daemon.Options{Paths: paths, Version: "test", Dispatch: DaemonDispatch, Busy: DaemonBusy})
	}()
	if err := daemon.WaitForSocket(paths.Socket, 5*time.Second); err != nil {
		panic(err)
	}

	code := m.Run()

	cancel()
	<-done
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
