//go:build !windows

package devices

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAdbPidFileEnv is where the fake adb writes the pid of the process that
// hangs, so a test can check it is gone afterwards.
const fakeAdbPidFileEnv = "MOBILECLI_TEST_FAKE_ADB_PIDFILE"

// hungAdbScript stands in for adb talking to a device that stopped answering.
const hungAdbScript = `#!/bin/sh
echo $$ > "$` + fakeAdbPidFileEnv + `"
exec sleep 60
`

// hungAdbWithChildScript also leaves a child holding adb's output open, which
// keeps a plain CombinedOutput waiting after adb itself was killed.
const hungAdbWithChildScript = `#!/bin/sh
echo $$ > "$` + fakeAdbPidFileEnv + `"
sleep 60 &
echo $! > "$` + fakeAdbPidFileEnv + `.child"
exec sleep 60
`

// useFakeAdb makes getAdbPath find script as adb and returns the pid file the
// script writes.
func useFakeAdb(t *testing.T, script string) string {
	t.Helper()
	sdk := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(sdk, "platform-tools"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sdk, "platform-tools", "adb"), []byte(script), 0o755)) //nolint:gosec // the fake adb must be executable
	t.Setenv("ANDROID_HOME", sdk)

	pidFile := filepath.Join(sdk, "adb.pid")
	t.Setenv(fakeAdbPidFileEnv, pidFile)
	t.Cleanup(func() { killPidInFile(pidFile + ".child") })
	return pidFile
}

func readPid(t *testing.T, pidFile string) int {
	t.Helper()
	data, err := os.ReadFile(pidFile)
	require.NoError(t, err, "the fake adb never ran")
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	require.NoError(t, err)
	return pid
}

func killPidInFile(pidFile string) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return
	}
	if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

// processIsGone reports whether pid no longer exists, zombie included: an
// unreaped child still answers kill(pid, 0).
func processIsGone(pid int) bool {
	return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
}

func onlineEmulator() *AndroidDevice {
	return &AndroidDevice{id: "Test_AVD", transportID: "emulator-5570", state: StateOnline}
}

// hungAdbDeadline leaves the fake adb time to start and write its pid, even on
// a busy machine.
const hungAdbDeadline = time.Second

func runAdbShellWithDeadline(d *AndroidDevice, timeout time.Duration) (out []byte, took time.Duration, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	start := time.Now()
	out, err = d.runAdbCommandContext(ctx, "shell", "dumpsys", "window", "displays", "--and-more")
	return out, time.Since(start), err
}

func TestAdbCommandOnHungDeviceEndsAtItsDeadline(t *testing.T) {
	useFakeAdb(t, hungAdbScript)

	_, took, err := runAdbShellWithDeadline(onlineEmulator(), hungAdbDeadline)

	require.ErrorIs(t, err, errDeviceNotResponding)
	assert.Less(t, took, 5*time.Second)
	assert.Contains(t, err.Error(), "emulator-5570 gave no answer to `adb shell dumpsys window displays ...`")
}

func TestAdbCommandOnHungDeviceIsKilledAndReaped(t *testing.T) {
	pidFile := useFakeAdb(t, hungAdbScript)

	_, _, err := runAdbShellWithDeadline(onlineEmulator(), hungAdbDeadline)

	require.ErrorIs(t, err, errDeviceNotResponding)
	assert.True(t, processIsGone(readPid(t, pidFile)), "adb is still running or was never reaped")
}

func TestAdbCommandReturnsEvenWhenAChildKeepsItsOutputOpen(t *testing.T) {
	pidFile := useFakeAdb(t, hungAdbWithChildScript)

	_, took, err := runAdbShellWithDeadline(onlineEmulator(), hungAdbDeadline)

	require.ErrorIs(t, err, errDeviceNotResponding)
	assert.Less(t, took, adbWaitDelay+3*time.Second)
	assert.True(t, processIsGone(readPid(t, pidFile)), "adb is still running or was never reaped")
}

func TestAdbCommandFailureThatIsNotATimeoutIsReportedAsIs(t *testing.T) {
	useFakeAdb(t, "#!/bin/sh\necho \"adb: device 'emulator-5570' not found\" >&2\nexit 1\n")

	out, _, err := runAdbShellWithDeadline(onlineEmulator(), 5*time.Second)

	require.Error(t, err)
	assert.NotErrorIs(t, err, errDeviceNotResponding)
	assert.Contains(t, string(out), "not found")
}

func TestStreamLogsOfOfflineEmulatorFailsInsteadOfWaiting(t *testing.T) {
	pidFile := useFakeAdb(t, hungAdbScript)
	offline := &AndroidDevice{id: "Test_AVD", state: StateOffline}

	err := offline.StreamLogs(context.Background(), func(LogEntry) bool { return true })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "device is offline")
	assert.NoFileExists(t, pidFile, "adb logcat should not have been started")
}

func TestStreamLogsKillsAndReapsLogcatWhenCancelled(t *testing.T) {
	pidFile := useFakeAdb(t, hungAdbScript)
	ctx, cancel := context.WithTimeout(context.Background(), hungAdbDeadline)
	defer cancel()

	err := onlineEmulator().StreamLogs(ctx, func(LogEntry) bool { return true })

	require.NoError(t, err, "a cancelled stream is a normal end")
	assert.True(t, processIsGone(readPid(t, pidFile)), "logcat is still running or was never reaped")
}

func TestAdbCommandSummaryKeepsTheFirstWords(t *testing.T) {
	assert.Equal(t, "reboot", adbCommandSummary([]string{"reboot"}))
	assert.Equal(t, "exec-out screencap -p", adbCommandSummary([]string{"exec-out", "screencap", "-p"}))
	assert.Equal(t, "shell dumpsys window displays ...", adbCommandSummary([]string{"shell", "dumpsys window displays --and-more"}))
}
