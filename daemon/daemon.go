package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/server"
	"github.com/sevlyar/go-daemon"
)

const (
	// DaemonEnvVar is the environment variable that marks a daemon child process
	DaemonEnvVar = "MOBILECLI_DAEMON_CHILD"

	// shutdownRequestID is the JSON-RPC request ID for shutdown commands
	shutdownRequestID = 1
)

// Daemonize detaches the process and returns the child process handle
// If the returned process is nil, this is the child process
// If the returned process is non-nil, this is the parent process
func Daemonize() (*os.Process, error) {
	// no PID file needed
	// we don't want log file, server handles its own logging
	ctx := &daemon.Context{
		PidFileName: "",
		PidFilePerm: 0,
		LogFileName: "",
		LogFilePerm: 0,
		WorkDir:     "/",
		Umask:       027,
		Args:        os.Args,
		Env:         append(os.Environ(), fmt.Sprintf("%s=1", DaemonEnvVar)),
	}

	child, err := ctx.Reborn()
	if err != nil {
		return nil, fmt.Errorf("failed to daemonize: %w", err)
	}

	return child, nil
}

// IsChild returns true if this is the daemon child process
func IsChild() bool {
	return os.Getenv(DaemonEnvVar) == "1"
}

// KillServer connects to the server and sends a shutdown command via JSON-RPC
func KillServer(addr string) error {
	// normalize address to match server's format
	// if no colon, assume it's a bare port number
	if !strings.Contains(addr, ":") {
		// validate it's a number
		if _, err := strconv.Atoi(addr); err == nil {
			addr = ":" + addr
		}
	}

	// if address starts with colon, prepend localhost
	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}

	// prepend http:// scheme
	addr = "http://" + addr

	// create JSON-RPC request
	reqBody := server.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "server.shutdown",
		ID:      shutdownRequestID,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// send request
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost, addr+"/rpc", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return fmt.Errorf("server is not running on %s", addr)
		}
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	// check response
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return fmt.Errorf("server returned error: %s", resp.Status)
	}

	return resp.Body.Close()
}
