package devices

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/devices/wda"
	"github.com/mobile-next/mobilecli/utils"
	"howett.net/plist"
)

const (
	tvosRunnerXctestrunName = "devicekit-tvos.xctestrun"
	tvosRunnerCacheKeyFile  = "cache.key"
	tvosAgentReadyTimeout   = 90 * time.Second
	tvosAgentReadyInterval  = 2 * time.Second
)

// tvosAgentCacheDir returns the per-device cache directory used to persist the
// signed runner app + generated .xctestrun under the user's application cache.
func tvosAgentCacheDir(deviceUDID string) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user cache dir: %w", err)
	}
	return filepath.Join(base, "mobilecli", "agent-cache", deviceUDID), nil
}

// tvosRunnerCacheKey keys the cache on the input artifact checksum, device UDID,
// and provisioning-profile UUID so a rebuild/re-sign is only needed when one of
// those changes.
func tvosRunnerCacheKey(artifactChecksum, deviceUDID, profileUUID string) string {
	sum := sha256.Sum256([]byte(artifactChecksum + "|" + deviceUDID + "|" + profileUUID))
	return hex.EncodeToString(sum[:])
}

// tvosRunnerDeviceKey derives the device-binding token persisted alongside the
// cache. It lets cachedTVOSXctestrunPath confirm a cache belongs to the expected
// device UDID at reuse time, when the artifact checksum and profile UUID inputs
// to tvosRunnerCacheKey are not available.
func tvosRunnerDeviceKey(deviceUDID string) string {
	sum := sha256.Sum256([]byte("device|" + deviceUDID))
	return hex.EncodeToString(sum[:])
}

// CacheTVOSRunner extracts the signed runner app from a re-signed IPA into the
// per-device cache and generates a .xctestrun that Milestone 3's StartAgent path
// launches via `xcodebuild test-without-building`. It returns the .xctestrun path.
func CacheTVOSRunner(deviceUDID, resignedIPAPath, artifactChecksum, profileUUID string) (string, error) {
	cacheDir, err := tvosAgentCacheDir(deviceUDID)
	if err != nil {
		return "", err
	}

	// start from a clean slate so a stale runner app never lingers
	if err := os.RemoveAll(cacheDir); err != nil {
		return "", fmt.Errorf("failed to clear runner cache: %w", err)
	}

	payloadRoot := filepath.Join(cacheDir, "Payload")
	if err := utils.UnzipToDir(resignedIPAPath, cacheDir); err != nil {
		return "", fmt.Errorf("failed to unzip runner into cache: %w", err)
	}

	runnerApp, err := findRunnerApp(payloadRoot)
	if err != nil {
		return "", err
	}

	xctestBundle, err := findRunnerTestBundle(runnerApp)
	if err != nil {
		return "", err
	}

	xctestrunPath := filepath.Join(cacheDir, tvosRunnerXctestrunName)
	if err := writeTVOSXctestrun(xctestrunPath, cacheDir, runnerApp, xctestBundle); err != nil {
		return "", err
	}

	keyPath := filepath.Join(cacheDir, tvosRunnerCacheKeyFile)
	key := tvosRunnerCacheKey(artifactChecksum, deviceUDID, profileUUID)
	// Line 1 is the device-binding token verified at reuse time; line 2 is the
	// full artifact/profile-bound key retained for a future check once StartAgent
	// can supply those inputs.
	keyContents := tvosRunnerDeviceKey(deviceUDID) + "\n" + key + "\n"
	if err := os.WriteFile(keyPath, []byte(keyContents), 0600); err != nil {
		return "", fmt.Errorf("failed to write cache key: %w", err)
	}

	utils.Verbose("cached tvOS runner + xctestrun at %s", xctestrunPath)
	return xctestrunPath, nil
}

// cachedTVOSXctestrunPath returns the cached .xctestrun path for a device only
// when a well-formed cache key is present whose device-binding token matches the
// requested device UDID, so StartAgent never launches a runner from a cache that
// was not produced by an install for THIS device.
//
// What is validated at reuse time:
//   - the .xctestrun exists in the per-device cache directory
//   - the cache.key file exists and is well-formed
//   - the stored device-binding token equals tvosRunnerDeviceKey(deviceUDID)
//
// What is NOT validated here: the artifact checksum and provisioning-profile
// UUID. StartAgent has neither value at reuse time, so the full artifact/profile
// key (stored on line 2 of cache.key) is retained for a future check but cannot
// be enforced here. Any artifact/profile change is fully re-applied by
// CacheTVOSRunner, which wipes and rewrites the per-device cache on every install.
func cachedTVOSXctestrunPath(deviceUDID string) (string, bool) {
	cacheDir, err := tvosAgentCacheDir(deviceUDID)
	if err != nil {
		return "", false
	}
	xctestrunPath := filepath.Join(cacheDir, tvosRunnerXctestrunName)
	if _, err := os.Stat(xctestrunPath); err != nil {
		return "", false
	}
	if !tvosCacheKeyMatchesDevice(cacheDir, deviceUDID) {
		utils.Verbose("refusing cached tvOS runner for %s: cache key missing or device mismatch", deviceUDID)
		return "", false
	}
	return xctestrunPath, true
}

// tvosCacheKeyMatchesDevice reports whether cache.key exists in cacheDir and its
// device-binding token (line 1) matches the given device UDID.
func tvosCacheKeyMatchesDevice(cacheDir, deviceUDID string) bool {
	data, err := os.ReadFile(filepath.Join(cacheDir, tvosRunnerCacheKeyFile))
	if err != nil {
		return false
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return false
	}
	return strings.TrimSpace(lines[0]) == tvosRunnerDeviceKey(deviceUDID)
}

// findRunnerApp locates the single *.app bundle inside an extracted Payload dir.
func findRunnerApp(payloadDir string) (string, error) {
	entries, err := os.ReadDir(payloadDir)
	if err != nil {
		return "", fmt.Errorf("failed to read runner Payload dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".app") {
			return filepath.Join(payloadDir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("no .app bundle found in %s", payloadDir)
}

// findRunnerTestBundle locates the UITests .xctest bundle inside the runner app.
func findRunnerTestBundle(runnerApp string) (string, error) {
	pluginsDir := filepath.Join(runnerApp, "PlugIns")
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return "", fmt.Errorf("failed to read runner PlugIns dir: %w", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".xctest") {
			return filepath.Join(pluginsDir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("no .xctest bundle found in %s", pluginsDir)
}

// writeTVOSXctestrun generates a format-version-1 .xctestrun for the tvOS UITests
// runner. Paths use __TESTROOT__ so the file is relocatable within the cache dir.
func writeTVOSXctestrun(xctestrunPath, cacheDir, runnerApp, xctestBundle string) error {
	runnerRel, err := filepath.Rel(cacheDir, runnerApp)
	if err != nil {
		return fmt.Errorf("failed to compute runner rel path: %w", err)
	}
	xctestRel, err := filepath.Rel(runnerApp, xctestBundle)
	if err != nil {
		return fmt.Errorf("failed to compute xctest rel path: %w", err)
	}

	moduleName := strings.TrimSuffix(filepath.Base(xctestBundle), ".xctest")
	targetName := moduleName
	moduleName = strings.ReplaceAll(moduleName, "-", "_")

	dyld := "__PLATFORMS__/AppleTVOS.platform/Developer/usr/lib/libXCTestBundleInject.dylib"

	target := map[string]any{
		"TestHostPath":                "__TESTROOT__/" + runnerRel,
		"TestBundlePath":              "__TESTHOST__/" + xctestRel,
		"IsUITestBundle":              true,
		"IsXCTRunnerHostedTestBundle": true,
		"ProductModuleName":           moduleName,
		"SystemAttachmentLifetime":    "deleteOnSuccess",
		"UserAttachmentLifetime":      "deleteOnSuccess",
		"CommandLineArguments":        []string{},
		"EnvironmentVariables":        map[string]any{},
		"TestingEnvironmentVariables": map[string]any{
			"DYLD_INSERT_LIBRARIES": dyld,
			"DYLD_FRAMEWORK_PATH":   "__TESTROOT__",
			"DYLD_LIBRARY_PATH":     "__TESTROOT__",
		},
		"DependentProductPaths": []string{
			"__TESTROOT__/" + runnerRel,
			"__TESTHOST__/" + xctestRel,
		},
	}

	doc := map[string]any{targetName: target}

	data, err := plist.MarshalIndent(doc, plist.XMLFormat, "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal xctestrun: %w", err)
	}

	if err := os.WriteFile(xctestrunPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write xctestrun: %w", err)
	}
	return nil
}

// tvosRunnerAction is a pure decision helper for the runner lifecycle: reuse a
// healthy session, restart an owned-but-dead session, or launch a fresh one.
func tvosRunnerAction(healthy, hasOwnedProcess bool) string {
	switch {
	case healthy:
		return "reuse"
	case hasOwnedProcess:
		return "restart"
	default:
		return "launch"
	}
}

// tvosTunnelBaseURL builds an IPv6-safe DeviceKit URL for the CoreDevice tunnel IP.
func tvosTunnelBaseURL(tunnelIP string, port int) string {
	return "http://" + net.JoinHostPort(tunnelIP, strconv.Itoa(port))
}

// startTVOSAgent starts (or reuses) the DeviceKit XCTest runner on a real Apple
// TV over the CoreDevice tunnel, without go-ios lookup or usbmuxd port forwarding.
func (d *IOSDevice) startTVOSAgent(config StartAgentConfig) error {
	if config.Hook != nil {
		hookName := fmt.Sprintf("tvos-device-%s", d.Udid)
		config.Hook.Register(hookName, d.Cleanup)
	}

	tunnelIP := d.TunnelIP
	if tunnelIP == "" {
		details, err := getCoreDeviceDetails(d.Udid)
		if err != nil {
			return fmt.Errorf("failed to resolve CoreDevice tunnel for %s: %w", d.Udid, err)
		}
		tunnelIP = details.TunnelIP
	}
	if tunnelIP == "" {
		return fmt.Errorf("no CoreDevice tunnel IP available for device %s; ensure it is awake and paired", d.Udid)
	}

	// bind the DeviceKit client directly at the tunnel IP (never the LAN)
	d.mu.Lock()
	d.TunnelIP = tunnelIP
	d.wdaClient = wda.NewWdaClient(tvosTunnelBaseURL(tunnelIP, deviceKitHTTPPort))
	hasOwnedProcess := d.tvosRunnerCancel != nil
	d.mu.Unlock()

	_, statusErr := d.wdaClient.GetStatus()
	action := tvosRunnerAction(statusErr == nil, hasOwnedProcess)
	utils.Verbose("tvOS runner lifecycle action for %s: %s", d.Udid, action)

	switch action {
	case "reuse":
		return nil
	case "restart":
		_ = d.cleanupTVOSRunner()
	}

	xctestrunPath, ok := cachedTVOSXctestrunPath(d.Udid)
	if !ok {
		return fmt.Errorf("no cached tvOS runner for device %s; run 'mobilecli agent install --device %s --agent-path <ipa> --provisioning-profile <profile>' first", d.Udid, d.Udid)
	}

	if config.OnProgress != nil {
		config.OnProgress("Launching tvOS XCTest runner")
	}
	if err := d.launchTVOSTestRunner(xctestrunPath); err != nil {
		return err
	}

	if config.OnProgress != nil {
		config.OnProgress("Waiting for DeviceKit server over CoreDevice tunnel")
	}
	return d.waitForTVOSAgentReady()
}

// launchTVOSTestRunner spawns and owns an xcodebuild test-without-building process
// that runs the DeviceKit UITest runner on the Apple TV over CoreDevice.
func (d *IOSDevice) launchTVOSTestRunner(xctestrunPath string) error {
	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, "xcodebuild", "test-without-building",
		"-xctestrun", xctestrunPath,
		"-destination", "id="+d.coreDeviceID())

	// Pass the tunnel bind host to the on-device DeviceKit server via the
	// TEST_RUNNER_ prefix so it binds a tunnel-reachable address (never 0.0.0.0,
	// never the LAN).
	cmd.Env = append(os.Environ(),
		"TEST_RUNNER_DEVICEKIT_LISTEN_HOST="+d.TunnelIP,
		fmt.Sprintf("TEST_RUNNER_DEVICEKIT_LISTEN_PORT=%d", deviceKitHTTPPort),
	)

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to launch xcodebuild runner for device %s: %w", d.Udid, err)
	}

	d.mu.Lock()
	d.tvosRunnerCancel = cancel
	d.mu.Unlock()

	go func() {
		err := cmd.Wait()
		if err != nil {
			utils.Verbose("tvOS xcodebuild runner for %s ended: %v", d.Udid, err)
		} else {
			utils.Verbose("tvOS xcodebuild runner for %s ended", d.Udid)
		}
		d.mu.Lock()
		d.tvosRunnerCancel = nil
		d.mu.Unlock()
	}()

	utils.Verbose("tvOS xcodebuild runner launched for device %s", d.Udid)
	return nil
}

// waitForTVOSAgentReady polls the DeviceKit health endpoint over the tunnel until
// it answers or a bounded timeout elapses, returning an actionable error.
func (d *IOSDevice) waitForTVOSAgentReady() error {
	deadline := time.After(tvosAgentReadyTimeout)
	ticker := time.NewTicker(tvosAgentReadyInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		if _, err := d.wdaClient.GetStatus(); err == nil {
			utils.Verbose("tvOS DeviceKit server ready for device %s", d.Udid)
			return nil
		} else {
			lastErr = err
		}

		select {
		case <-deadline:
			return fmt.Errorf("tvOS DeviceKit server for device %s (%s) did not become ready within %s: %w",
				d.DeviceName, d.Udid, tvosAgentReadyTimeout, lastErr)
		case <-ticker.C:
		}
	}
}

// cleanupTVOSRunner cancels an owned xcodebuild runner process, if any.
func (d *IOSDevice) cleanupTVOSRunner() error {
	d.mu.Lock()
	cancel := d.tvosRunnerCancel
	d.tvosRunnerCancel = nil
	d.mu.Unlock()

	if cancel != nil {
		utils.Verbose("Stopping tvOS xcodebuild runner for device %s", d.Udid)
		cancel()
	}
	return nil
}

// Focus selects an on-screen element by accessibility identifier and/or label and
// drives Siri Remote focus to it via the DeviceKit device.io.focus RPC over the
// tunnel transport. It returns the focused element as raw JSON.
func (d *IOSDevice) Focus(identifier, label string) (any, error) {
	return d.wdaClient.Focus(identifier, label)
}

// TVOSAgentReachable reports whether the DeviceKit runner session answers over the
// CoreDevice tunnel. It is a lightweight, read-only health ping for agent status.
func (d *IOSDevice) TVOSAgentReachable() bool {
	tunnelIP := d.TunnelIP
	if tunnelIP == "" {
		details, err := getCoreDeviceDetails(d.Udid)
		if err != nil {
			return false
		}
		tunnelIP = details.TunnelIP
	}
	if tunnelIP == "" {
		return false
	}

	client := wda.NewWdaClient(tvosTunnelBaseURL(tunnelIP, deviceKitHTTPPort))
	_, err := client.GetStatus()
	return err == nil
}
