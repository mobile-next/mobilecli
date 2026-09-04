package daemon

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	startTimeout = 5 * time.Second
	logTailBytes = 4096
)

// EnsureRunning makes sure a daemon of exactly `version` is listening. It
// cleans up stale files, replaces a daemon of a different version, and spawns
// os.Executable() with spawnArgs when needed.
func EnsureRunning(version string, spawnArgs []string) error {
	paths, err := Paths()
	if err != nil {
		return err
	}

	if conn, err := dial(paths.Socket); err == nil {
		_ = conn.Close()
		info, err := readPidFile(paths.Pid)
		if err == nil && info.Version == version {
			return nil
		}
		fmt.Fprintf(os.Stderr, "Restarting daemon (pid %d, %s): version mismatch, client is %s\n", info.PID, info.Version, version)
		if err := Shutdown(); err != nil {
			return fmt.Errorf("stop outdated daemon: %w", err)
		}
	} else if startedByAnotherClient(paths, version) {
		// a concurrent invocation is already spawning a matching daemon: wait for it
		return WaitForSocket(paths.Socket, startTimeout)
	} else {
		cleanStale(paths)
	}

	return spawn(paths, version, spawnArgs)
}

// startedByAnotherClient reports whether the pid file names a live daemon of
// the right version that is not listening yet.
func startedByAnotherClient(paths FilePaths, version string) bool {
	info, err := readPidFile(paths.Pid)
	return err == nil && info.Version == version && processExists(info.PID)
}

// cleanStale removes leftovers of a daemon that is no longer listening. The
// recorded pid is never signalled: after a crash or reboot the OS may have
// handed it to an unrelated process, and a daemon that lost its socket is
// harmless anyway (it idles out on its own).
func cleanStale(paths FilePaths) {
	_ = os.Remove(paths.Pid)
	_ = os.Remove(paths.Socket)
}

func spawn(paths FilePaths, version string, args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	if err := os.MkdirAll(paths.Dir, 0o700); err != nil {
		return fmt.Errorf("create daemon dir: %w", err)
	}
	logFile, err := os.OpenFile(paths.Log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open daemon log: %w", err)
	}
	defer func() { _ = logFile.Close() }()

	// claim the pid file exclusively: whoever loses the race waits for the
	// winner's daemon instead of spawning a second one
	claim, err := os.OpenFile(paths.Pid, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return WaitForSocket(paths.Socket, startTimeout)
		}
		return fmt.Errorf("claim pid file: %w", err)
	}
	_ = claim.Close()

	cmd := exec.Command(exe, args...)
	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Dir = daemonWorkDir()
	detach(cmd)

	if err := cmd.Start(); err != nil {
		_ = os.Remove(paths.Pid)
		return fmt.Errorf("start daemon: %w", err)
	}
	// the child outlives us; reap it in the background so a quick death is not a zombie
	go func() { _ = cmd.Wait() }()
	// provisional pid: the daemon overwrites this once it is listening
	if err := writePidFile(paths.Pid, pidInfo{PID: cmd.Process.Pid, Version: version}); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}

	if err := WaitForSocket(paths.Socket, startTimeout); err != nil {
		return fmt.Errorf("%w\n--- tail of %s ---\n%s", err, paths.Log, tailFile(paths.Log, logTailBytes))
	}
	return nil
}

func daemonWorkDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return os.TempDir()
}

func tailFile(path string, n int64) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return ""
	}
	start := st.Size() - n
	if start < 0 {
		start = 0
	}
	buf := make([]byte, st.Size()-start)
	if _, err := f.ReadAt(buf, start); err != nil && !errors.Is(err, os.ErrClosed) {
		return strings.TrimSpace(string(buf))
	}
	return strings.TrimSpace(string(buf))
}
