package devices

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"testing"
	"time"
)

func startFakeADBServer(t *testing.T, handler func(conn net.Conn)) (host string, port int, closeFn func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	addr := ln.Addr().(*net.TCPAddr)

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		handler(conn)
	}()

	return "127.0.0.1", addr.Port, func() {
		_ = ln.Close()
		<-done
	}
}

func readADBService(t *testing.T, conn net.Conn) string {
	t.Helper()

	hexLen := make([]byte, 4)
	if _, err := io.ReadFull(conn, hexLen); err != nil {
		t.Fatalf("read len: %v", err)
	}

	l, err := strconv.ParseInt(string(hexLen), 16, 64)
	if err != nil {
		t.Fatalf("parse len %q: %v", string(hexLen), err)
	}

	buf := make([]byte, l)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read service bytes: %v", err)
	}

	return string(buf)
}

func writeOKAY(t *testing.T, conn net.Conn) {
	t.Helper()
	if _, err := conn.Write([]byte("OKAY")); err != nil {
		t.Fatalf("write OKAY: %v", err)
	}
}

func writeLenPrefixed(t *testing.T, conn net.Conn, payload string) {
	t.Helper()
	prefix := fmt.Sprintf("%04x", len(payload))
	if _, err := conn.Write([]byte(prefix + payload)); err != nil {
		t.Fatalf("write frame: %v", err)
	}
}

func TestADB_Shell_UsesTransportOnSameConnection(t *testing.T) {
	host, port, closeFn := startFakeADBServer(t, func(conn net.Conn) {
		defer conn.Close()

		got := readADBService(t, conn)
		if got != "host:transport:serial-123" {
			t.Fatalf("expected transport select, got %q", got)
		}
		writeOKAY(t, conn)

		got = readADBService(t, conn)
		if got != "shell:echo hi" {
			t.Fatalf("expected shell, got %q", got)
		}
		writeOKAY(t, conn)

		_, _ = conn.Write([]byte("hi\n"))
	})
	defer closeFn()

	adb := NewADB(host, port)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := adb.Shell(ctx, "serial-123", "echo hi")
	if err != nil {
		t.Fatalf("Shell error: %v", err)
	}
	if out != "hi\n" {
		t.Fatalf("unexpected output %q", out)
	}
}

func TestADB_TrackDevices_StreamsFramesAndStopsOnCancel(t *testing.T) {
	host, port, closeFn := startFakeADBServer(t, func(conn net.Conn) {
		defer conn.Close()

		got := readADBService(t, conn)
		if got != "host:track-devices" {
			t.Fatalf("expected host:track-devices, got %q", got)
		}
		writeOKAY(t, conn)

		writeLenPrefixed(t, conn, "a\tdevice\n")
		writeLenPrefixed(t, conn, "b\toffline\n")

		// keep the connection open until the client cancels (which closes the conn).
		_, _ = io.Copy(io.Discard, conn)
	})
	defer closeFn()

	adb := NewADB(host, port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	updates, errs, err := adb.TrackDevices(ctx)
	if err != nil {
		t.Fatalf("TrackDevices error: %v", err)
	}

	got1 := <-updates
	got2 := <-updates
	if got1 != "a\tdevice\n" {
		t.Fatalf("unexpected frame 1: %q", got1)
	}
	if got2 != "b\toffline\n" {
		t.Fatalf("unexpected frame 2: %q", got2)
	}

	cancel()

	select {
	case e := <-errs:
		if e != nil {
			// cancellation is not an error we surface on the error channel.
			t.Fatalf("unexpected error: %v", e)
		}
	case <-time.After(250 * time.Millisecond):
		// ok
	}
}

func TestADB_ResponseLimit(t *testing.T) {
	host, port, closeFn := startFakeADBServer(t, func(conn net.Conn) {
		defer conn.Close()

		_ = readADBService(t, conn) // host:transport:...
		writeOKAY(t, conn)

		_ = readADBService(t, conn) // shell:...
		writeOKAY(t, conn)

		// write slightly over 2MiB
		payload := make([]byte, defaultADBMaxResponseBytes+1)
		for i := range payload {
			payload[i] = 'x'
		}
		_, _ = conn.Write(payload)
	})
	defer closeFn()

	adb := NewADB(host, port)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := adb.Shell(ctx, "serial-123", "echo big")
	if err == nil {
		t.Fatalf("expected error due to response limit")
	}
}

func TestADB_ExecOut_UsesTransportOnSameConnection(t *testing.T) {
	host, port, closeFn := startFakeADBServer(t, func(conn net.Conn) {
		defer conn.Close()

		got := readADBService(t, conn)
		if got != "host:transport:serial-123" {
			t.Fatalf("expected transport select, got %q", got)
		}
		writeOKAY(t, conn)

		got = readADBService(t, conn)
		if got != "exec:cmd" {
			t.Fatalf("expected exec, got %q", got)
		}
		writeOKAY(t, conn)

		_, _ = conn.Write([]byte("out\n"))
	})
	defer closeFn()

	adb := NewADB(host, port)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := adb.ExecOut(ctx, "serial-123", "cmd")
	if err != nil {
		t.Fatalf("ExecOut error: %v", err)
	}
	if out != "out\n" {
		t.Fatalf("unexpected output %q", out)
	}
}

func TestADB_TrackDevices_IntegrationLocalhost5037(t *testing.T) {
	adb := NewADB("localhost", 5037)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	updates, errs, err := adb.TrackDevices(ctx)
	if err != nil {
		// if adb isn't running locally, skip.
		t.Skipf("adb server not reachable on localhost:5037: %v", err)
	}

	select {
	case <-updates:
		// any first update is acceptable (it can be empty if there are no devices).
	case e := <-errs:
		if e != nil {
			t.Fatalf("TrackDevices error: %v", e)
		}
	case <-ctx.Done():
		t.Fatalf("timed out waiting for track-devices update")
	}
}
