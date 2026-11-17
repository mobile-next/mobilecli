package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mobile-next/mobilecli/internal/commands"
	"github.com/mobile-next/mobilecli/internal/daemon"
	"github.com/mobile-next/mobilecli/internal/utils"
)

// defaultDaemonIdleTimeout is how long an auto-started daemon lives without requests.
// statusError is the envelope status of a failed command.
const statusError = "error"

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
		Status string          `json:"status"`
		Error  string          `json:"error"`
		Data   json.RawMessage `json:"data"` // any shape: object, array, or absent
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("invalid response from daemon: %w", err)
	}
	switch env.Status {
	case statusError:
		return errors.New(env.Error)
	case "fail":
		// e.g. agent status/uninstall with no agent installed: the envelope
		// carries the reason in data.message, and the exit code must be non-zero
		var data struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(env.Data, &data) == nil && data.Message != "" {
			return errors.New(data.Message)
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

// localOutputPath makes an --output argument absolute for the daemon, whose
// cwd is not ours. A trailing separator survives, because it is what tells the
// daemon to generate a filename inside that directory; unset means our cwd as
// that directory. "-" (stdout) passes through.
func localOutputPath(p string) (string, error) {
	if p == "-" {
		return p, nil
	}
	if p == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot resolve the current directory: %w", err)
		}
		return withTrailingSeparator(cwd), nil
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("invalid output path %q: %w", p, err)
	}
	if os.IsPathSeparator(p[len(p)-1]) {
		return withTrailingSeparator(abs), nil
	}
	return abs, nil
}

func withTrailingSeparator(dir string) string {
	if os.IsPathSeparator(dir[len(dir)-1]) {
		return dir
	}
	return dir + string(filepath.Separator)
}
