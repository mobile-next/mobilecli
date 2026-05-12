package devices

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	// default dial/read/write timeout for non-streaming adb operations
	defaultADBTimeout = 10 * time.Second
	// maximum payload we will buffer in memory from adb
	defaultADBMaxResponseBytes = 2 << 20 // 2MiB
)

// ADB is a small protocol-level client for talking to an adb server over TCP.
//
// note: many adb services are connection-scoped (e.g. host:transport), so callers
// must keep operations that depend on each other on the same connection.
type ADB struct {
	host             string
	port             int
	timeout          time.Duration
	maxResponseBytes int64
}

// creates a new adb client with default timeout/limits.
func NewADB(host string, port int) *ADB {
	return &ADB{
		host:             host,
		port:             port,
		timeout:          defaultADBTimeout,
		maxResponseBytes: defaultADBMaxResponseBytes,
	}
}

// WithTimeout sets the dial/read/write timeout used for non-streaming operations.
//
// note: TrackDevices uses this timeout for its initial handshake only.
func (a *ADB) WithTimeout(timeout time.Duration) *ADB {
	a.timeout = timeout
	return a
}

// WithMaxResponseBytes sets the maximum number of bytes the client will buffer in memory
// for any single response.
func (a *ADB) WithMaxResponseBytes(max int64) *ADB {
	a.maxResponseBytes = max
	return a
}

type adbConn struct {
	conn             net.Conn
	timeout          time.Duration
	maxResponseBytes int64
}

func (c *adbConn) close() error {
	return c.conn.Close()
}

func (c *adbConn) setDeadlineFromNow() {
	_ = c.conn.SetDeadline(time.Now().Add(c.timeout))
}

func (c *adbConn) clearDeadline() {
	_ = c.conn.SetDeadline(time.Time{})
}

func (c *adbConn) writeService(service string) error {
	if len(service) > 0xFFFF {
		return fmt.Errorf("adb service string too long: %d bytes (max 65535)", len(service))
	}
	length := fmt.Sprintf("%04x", len(service))
	message := length + service

	if _, err := c.conn.Write([]byte(message)); err != nil {
		return fmt.Errorf("failed to write adb service %q: %w", service, err)
	}

	return nil
}

func (c *adbConn) readStatus() error {
	status := make([]byte, 4)
	if _, err := io.ReadFull(c.conn, status); err != nil {
		return fmt.Errorf("failed to read adb status: %w", err)
	}

	s := string(status)
	switch s {
	case "OKAY":
		return nil
	case "FAIL":
		errorData, err := c.readLengthPrefixedData()
		if err != nil {
			return fmt.Errorf("failed to read adb error message: %w", err)
		}
		return fmt.Errorf("adb server error: %s", string(errorData))
	default:
		return fmt.Errorf("unexpected adb status: %q", s)
	}
}

func (c *adbConn) service(service string) error {
	if err := c.writeService(service); err != nil {
		return err
	}
	if err := c.readStatus(); err != nil {
		return err
	}
	return nil
}

func (c *adbConn) readLengthPrefixedData() ([]byte, error) {
	lengthBytes := make([]byte, 4)
	if _, err := io.ReadFull(c.conn, lengthBytes); err != nil {
		return nil, fmt.Errorf("failed to read adb length: %w", err)
	}

	length, err := strconv.ParseInt(string(lengthBytes), 16, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid adb length %q: %w", string(lengthBytes), err)
	}
	if length < 0 {
		return nil, fmt.Errorf("invalid adb length: %d", length)
	}
	if length > c.maxResponseBytes {
		return nil, fmt.Errorf("adb response too large: %d bytes (limit %d)", length, c.maxResponseBytes)
	}

	data := make([]byte, int(length))
	if _, err := io.ReadFull(c.conn, data); err != nil {
		return nil, fmt.Errorf("failed to read adb payload: %w", err)
	}
	return data, nil
}

func (c *adbConn) readAllLimited() ([]byte, error) {
	// read up to max+1 so we can detect truncation.
	maxPlusOne := c.maxResponseBytes + 1
	data, err := io.ReadAll(io.LimitReader(c.conn, maxPlusOne))
	if err != nil {
		return nil, fmt.Errorf("failed to read adb stream: %w", err)
	}
	if int64(len(data)) > c.maxResponseBytes {
		return nil, fmt.Errorf("adb response too large: >%d bytes", c.maxResponseBytes)
	}
	return data, nil
}

func (a *ADB) dial(ctx context.Context) (*adbConn, error) {
	addr := net.JoinHostPort(a.host, strconv.Itoa(a.port))
	d := net.Dialer{Timeout: a.timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to adb server at %s: %w", addr, err)
	}

	return &adbConn{
		conn:             conn,
		timeout:          a.timeout,
		maxResponseBytes: a.maxResponseBytes,
	}, nil
}

// Shell runs a shell command on a specific device serial and returns stdout/stderr.
func (a *ADB) Shell(ctx context.Context, serial, command string) (string, error) {
	if serial == "" {
		return "", fmt.Errorf("serial is required")
	}
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	c, err := a.dial(ctx)
	if err != nil {
		return "", err
	}
	defer c.close()
	c.setDeadlineFromNow()

	if err := c.service("host:transport:" + serial); err != nil {
		return "", err
	}
	if err := c.service("shell:" + command); err != nil {
		return "", err
	}

	data, err := c.readAllLimited()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ExecOut runs a command using adb's exec service (similar to `adb exec-out`).
//
// exec is a variant of shell that uses a raw PTY to avoid output mangling.
func (a *ADB) ExecOut(ctx context.Context, serial, command string) (string, error) {
	c, err := a.dial(ctx)
	if err != nil {
		return "", err
	}
	defer c.close()
	c.setDeadlineFromNow()

	if serial == "" {
		return "", fmt.Errorf("serial is required")
	}
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	if err := c.service("host:transport:" + serial); err != nil {
		return "", err
	}
	if err := c.service("exec:" + command); err != nil {
		return "", err
	}

	data, err := c.readAllLimited()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// TrackDevices subscribes to host:track-devices and streams each update frame.
//
// the returned channels are closed when the context is canceled or an error occurs.
func (a *ADB) TrackDevices(ctx context.Context) (<-chan string, <-chan error, error) {
	c, err := a.dial(ctx)
	if err != nil {
		return nil, nil, err
	}

	updates := make(chan string, 1)
	errs := make(chan error, 1)

	// do the handshake with a deadline, then switch to ctx-driven cancellation for streaming.
	c.setDeadlineFromNow()
	if err := c.service("host:track-devices"); err != nil {
		_ = c.close()
		close(updates)
		close(errs)
		return nil, nil, err
	}
	c.clearDeadline()

	go func() {
		var closeOnce sync.Once
		closeConn := func() {
			closeOnce.Do(func() {
				_ = c.close()
			})
		}

		// ensure ctx cancellation interrupts any blocking reads.
		go func() {
			<-ctx.Done()
			closeConn()
		}()

		defer func() {
			closeConn()
			close(updates)
			close(errs)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// each frame is length-prefixed.
			frame, err := c.readLengthPrefixedData()
			if err != nil {
				// if the context was canceled, the close-on-cancel goroutine will
				// typically make the read fail; don't surface that as an error.
				select {
				case <-ctx.Done():
					return
				default:
				}

				select {
				case errs <- err:
				default:
				}
				return
			}

			select {
			case updates <- string(frame):
			case <-ctx.Done():
				return
			}
		}
	}()

	return updates, errs, nil
}
