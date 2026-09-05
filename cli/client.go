package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/mobile-next/mobilecli/daemon"
	"github.com/mobile-next/mobilecli/utils"
)

// defaultDaemonIdleTimeout is how long an auto-started daemon lives without requests.
const defaultDaemonIdleTimeout = 30 * time.Minute

// daemonSpawnArgs are the argv used to auto-start the daemon from this binary.
func daemonSpawnArgs() []string {
	args := []string{"daemon", "start"}
	if verbose {
		args = append(args, "--verbose")
	}
	if insecureStorage {
		args = append(args, "--insecure-storage")
	}
	return args
}

func ensureDaemon() error {
	return daemon.EnsureRunning(utils.Version, daemonSpawnArgs())
}

// runViaDaemon executes a CLI method on the daemon and prints the response
// envelope exactly as the in-process command used to. Returns an error when
// the envelope status is "error" so the process exits non-zero.
func runViaDaemon(method string, params any) error {
	raw, err := callDaemon(method, params, daemon.NoTimeout)
	if err != nil {
		printJson(errorEnvelope(err))
		return err
	}
	out, err := indentedJSON(raw)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return envelopeError(raw)
}

// callDaemon returns the raw envelope of a CLI method, for commands that need
// to inspect the data (screenshot to stdout, dump ui --format text). CLI calls
// carry no read deadline, like the in-process commands they replaced (installs
// and pushes can take minutes); Ctrl-C cancels via cmdContext.
func callDaemon(method string, params any, timeout time.Duration) (json.RawMessage, error) {
	if err := ensureDaemon(); err != nil {
		return nil, err
	}
	return daemon.CallWithTimeout(cmdContext(), method, params, timeout)
}

// decodeEnvelope unmarshals the envelope's data field into out and returns
// the envelope error, if any.
func decodeEnvelope(raw json.RawMessage, out any) error {
	if err := envelopeError(raw); err != nil {
		return err
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return err
	}
	if len(env.Data) == 0 {
		return nil
	}
	return json.Unmarshal(env.Data, out)
}

func envelopeError(raw json.RawMessage) error {
	var env struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Data   struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("invalid response from daemon: %w", err)
	}
	switch env.Status {
	case "error":
		return errors.New(env.Error)
	case "fail":
		// e.g. agent status/uninstall with no agent installed: the envelope
		// carries the reason in data.message, and the exit code must be non-zero
		if env.Data.Message != "" {
			return errors.New(env.Data.Message)
		}
		return errors.New("command failed")
	}
	return nil
}

func errorEnvelope(err error) *commands.CommandResponse {
	return commands.NewErrorResponse(err)
}

func indentedJSON(raw json.RawMessage) (string, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return "", fmt.Errorf("invalid response from daemon: %w", err)
	}
	return buf.String(), nil
}

// absLocalPath makes a local file argument absolute so the daemon, whose cwd
// is not ours, reads or writes the file the user meant. "-" (stdout) and ""
// (unset) pass through.
func absLocalPath(p string) string {
	if p == "" || p == "-" {
		return p
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// cwdWithSeparator is the caller's working directory as a directory path, used
// when a command generates a default output filename.
func cwdWithSeparator() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd + string(filepath.Separator)
}
