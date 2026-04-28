package devices

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/mobile-next/mobilecli/devices/wda"
	"github.com/mobile-next/mobilecli/rpc"
	"github.com/mobile-next/mobilecli/utils"
)

const artifactsHost = "mobilenexthq-artifacts.s3.us-west-2.amazonaws.com"

type params map[string]any

type RemoteDevice struct {
	deviceID   string
	name       string
	platform   string
	deviceType string
	version    string
	state      string
	model      string
	token      string
}

func NewRemoteDevice(info DeviceInfo, token string) *RemoteDevice {
	devType := info.Type
	if devType == "" {
		devType = "remote"
	}

	return &RemoteDevice{
		deviceID:   info.ID,
		name:       info.Name,
		platform:   info.Platform,
		deviceType: devType,
		version:    info.Version,
		state:      info.State,
		model:      info.Model,
		token:      token,
	}
}

func (r *RemoteDevice) ID() string         { return r.deviceID }
func (r *RemoteDevice) Name() string       { return r.name }
func (r *RemoteDevice) Platform() string   { return r.platform }
func (r *RemoteDevice) DeviceType() string { return r.deviceType }
func (r *RemoteDevice) Version() string    { return r.version }
func (r *RemoteDevice) State() string      { return r.state }

func (r *RemoteDevice) StartAgent(config StartAgentConfig) error {
	return nil
}

func (r *RemoteDevice) callRPC(method string, params params) (any, error) {
	var result any
	if err := rpc.Call(r.token, method, params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func rpcCall[T any](r *RemoteDevice, method string, extra params) (T, error) {
	p := params{"deviceId": r.deviceID}
	for k, v := range extra {
		p[k] = v
	}
	result, err := r.callRPC(method, p)
	var zero T
	if err != nil {
		return zero, err
	}
	var out T
	if err := rpc.Remarshal(result, &out); err != nil {
		return zero, err
	}
	return out, nil
}

func (r *RemoteDevice) fireRPC(method string, extra params) error {
	p := params{"deviceId": r.deviceID}
	for k, v := range extra {
		p[k] = v
	}
	_, err := r.callRPC(method, p)
	return err
}

func (r *RemoteDevice) TakeScreenshot() ([]byte, error) {
	resp, err := rpcCall[struct {
		Data string `json:"data"`
	}](r, "device.screenshot", params{})
	if err != nil {
		return nil, err
	}

	data := resp.Data
	if strings.HasPrefix(data, "data:") {
		if idx := strings.Index(data, ","); idx != -1 {
			data = data[idx+1:]
		}
	}

	return base64.StdEncoding.DecodeString(data)
}

func (r *RemoteDevice) Tap(x, y int) error {
	return r.fireRPC("device.io.tap", params{"x": x, "y": y})
}

func (r *RemoteDevice) LongPress(x, y, duration int) error {
	return r.fireRPC("device.io.longpress", params{"x": x, "y": y, "duration": duration})
}

func (r *RemoteDevice) Swipe(x1, y1, x2, y2 int) error {
	return r.fireRPC("device.io.swipe", params{"x1": x1, "y1": y1, "x2": x2, "y2": y2})
}

func (r *RemoteDevice) Gesture(actions []wda.TapAction) error {
	return r.fireRPC("device.io.gesture", params{"actions": actions})
}

func (r *RemoteDevice) SendKeys(text string) error {
	return r.fireRPC("device.io.text", params{"text": text})
}

func (r *RemoteDevice) PressButton(key string) error {
	return r.fireRPC("device.io.button", params{"button": key})
}

func (r *RemoteDevice) OpenURL(url string) error {
	return r.fireRPC("device.url", params{"url": url})
}

func (r *RemoteDevice) LaunchApp(bundleID string, locales []string) error {
	p := params{"bundleId": bundleID}
	if len(locales) > 0 {
		p["locales"] = locales
	}
	return r.fireRPC("device.apps.launch", p)
}

func (r *RemoteDevice) TerminateApp(bundleID string) error {
	return r.fireRPC("device.apps.terminate", params{"bundleId": bundleID})
}

func (r *RemoteDevice) Boot() error {
	return r.fireRPC("device.boot", params{})
}

func (r *RemoteDevice) Shutdown() error {
	return r.fireRPC("device.shutdown", params{})
}

func (r *RemoteDevice) Reboot() error {
	return r.fireRPC("device.reboot", params{})
}

func (r *RemoteDevice) GetOrientation() (string, error) {
	resp, err := rpcCall[struct {
		Orientation string `json:"orientation"`
	}](r, "device.io.orientation.get", params{})
	if err != nil {
		return "", err
	}
	return resp.Orientation, nil
}

func (r *RemoteDevice) SetOrientation(orientation string) error {
	return r.fireRPC("device.io.orientation.set", params{"orientation": orientation})
}

func (r *RemoteDevice) Info() (*FullDeviceInfo, error) {
	return rpcCall[*FullDeviceInfo](r, "device.info", params{})
}

func (r *RemoteDevice) ListApps(onlyLaunchable bool) ([]InstalledAppInfo, error) {
	return rpcCall[[]InstalledAppInfo](r, "device.apps.list", params{})
}

func (r *RemoteDevice) GetForegroundApp() (*ForegroundAppInfo, error) {
	return rpcCall[*ForegroundAppInfo](r, "device.apps.foreground", params{})
}

func (r *RemoteDevice) DumpSource() ([]ScreenElement, error) {
	resp, err := rpcCall[struct {
		Elements []ScreenElement `json:"elements"`
	}](r, "device.dump.ui", params{})
	if err != nil {
		return nil, err
	}
	return resp.Elements, nil
}

func (r *RemoteDevice) DumpSourceRaw() (any, error) {
	resp, err := rpcCall[struct {
		RawData any `json:"rawData"`
	}](r, "device.dump.ui", params{"format": "raw"})
	if err != nil {
		return nil, err
	}
	return resp.RawData, nil
}

type uploadResult struct {
	UploadID  string `json:"uploadId"`
	UploadURL string `json:"uploadUrl"`
}

var sanitizeRe = regexp.MustCompile(`[^0-9a-zA-Z_.]`)

func sanitizeFilename(name string) string {
	return sanitizeRe.ReplaceAllString(name, "_")
}

var uploadHTTPClient = &http.Client{Timeout: 5 * time.Minute}

func uploadFileToURL(filePath, uploadURL string) error {
	u, err := url.Parse(uploadURL)
	if err != nil {
		return fmt.Errorf("invalid upload URL: %w", err)
	}
	if u.Scheme != "https" || u.Hostname() != artifactsHost {
		return fmt.Errorf("upload URL must be https://%s/..., got: %s", artifactsHost, uploadURL)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	sizeMB := float64(fi.Size()) / (1024 * 1024)
	utils.Verbose("upload started, file size: %.2f MB", sizeMB)

	start := time.Now()

	req, err := http.NewRequest(http.MethodPut, u.String(), f)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}
	req.ContentLength = fi.Size()
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := uploadHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upload failed with status %d", resp.StatusCode)
	}

	elapsed := time.Since(start).Seconds()
	speedMB := sizeMB / elapsed
	utils.Verbose("upload completed in %.1f seconds, speed: %.3f MB/sec", elapsed, speedMB)

	return nil
}

func downloadFile(downloadURL, outputPath string, cb *ScreenRecordCallbacks) error {
	parsed, err := url.Parse(downloadURL)
	if err != nil {
		return fmt.Errorf("invalid download URL: %w", err)
	}

	if parsed.Scheme != "https" || parsed.Hostname() != artifactsHost {
		return fmt.Errorf("download URL must be https://%s/..., got: %s", artifactsHost, downloadURL)
	}

	utils.Verbose("downloading from %v", parsed)
	resp, err := http.Get(parsed.String())
	if err != nil {
		return fmt.Errorf("HTTP GET failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	deadline := time.NewTimer(1 * time.Minute)
	var onProgress func(float64, float64)
	if cb != nil {
		onProgress = cb.OnDownloadProgress
	}
	pr := &progressReader{
		reader:     resp.Body,
		total:      resp.ContentLength,
		onProgress: onProgress,
		deadline:   deadline,
	}
	var body io.Reader = pr

	start := time.Now()
	written, err := io.Copy(f, body)
	deadline.Stop()
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// report final progress to ensure 100% is shown
	if onProgress != nil && resp.ContentLength > 0 {
		totalMB := float64(resp.ContentLength) / (1024 * 1024)
		onProgress(totalMB, totalMB)
	}

	if cb != nil && cb.OnDownloaded != nil {
		elapsed := time.Since(start).Seconds()
		if elapsed > 0 {
			writtenMB := float64(written) / (1024 * 1024)
			cb.OnDownloaded(writtenMB / elapsed)
		}
	}

	return nil
}

func (r *RemoteDevice) InstallApp(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	filename := sanitizeFilename(filepath.Base(path))

	result, err := r.callRPC("uploads.create", params{
		"filename": filename,
		"filesize": fi.Size(),
	})
	if err != nil {
		return fmt.Errorf("failed to request upload url: %w", err)
	}

	var upload uploadResult
	if err := rpc.Remarshal(result, &upload); err != nil {
		return err
	}

	if err := uploadFileToURL(path, upload.UploadURL); err != nil {
		return err
	}

	return r.fireRPC("device.apps.install", params{"uploadId": upload.UploadID})
}

func (r *RemoteDevice) UninstallApp(packageName string) (*InstalledAppInfo, error) {
	return nil, fmt.Errorf("uninstall app is not supported on remote devices")
}

// ScreenRecordCallbacks provides optional progress callbacks for screen recording
type ScreenRecordCallbacks struct {
	OnRecordingEnded   func()
	OnDownloadProgress func(downloadedMB, totalMB float64)
	OnDownloaded       func(speedMBps float64)
}

type progressReader struct {
	reader     io.Reader
	total      int64
	read       int64
	onProgress func(readMB, totalMB float64)
	lastReport time.Time
	deadline   *time.Timer
}

func (pr *progressReader) Read(p []byte) (int, error) {
	if pr.deadline != nil {
		// read with a deadline: fail if no data arrives within the timeout
		type readResult struct {
			n   int
			err error
		}
		ch := make(chan readResult, 1)
		go func() {
			n, err := pr.reader.Read(p)
			ch <- readResult{n, err}
		}()
		select {
		case res := <-ch:
			if res.n > 0 {
				pr.deadline.Reset(1 * time.Minute)
			}
			pr.updateProgress(res.n)
			return res.n, res.err
		case <-pr.deadline.C:
			return 0, fmt.Errorf("download stalled for over 1 minute")
		}
	}

	n, err := pr.reader.Read(p)
	pr.updateProgress(n)
	return n, err
}

func (pr *progressReader) updateProgress(n int) {
	pr.read += int64(n)
	if pr.onProgress != nil && time.Since(pr.lastReport) >= 100*time.Millisecond {
		pr.lastReport = time.Now()
		readMB := float64(pr.read) / (1024 * 1024)
		totalMB := float64(pr.total) / (1024 * 1024)
		pr.onProgress(readMB, totalMB)
	}
}

func (r *RemoteDevice) ScreenRecord(outputPath string, timeLimit int, stopChan <-chan struct{}, cb *ScreenRecordCallbacks) error {
	_, err := rpcCall[struct {
		Status string `json:"status"`
		Output string `json:"output"`
	}](r, "device.screenrecord", params{
		"output":    outputPath,
		"timeLimit": timeLimit,
	})
	if err != nil {
		return err
	}

	if stopChan == nil {
		stopChan = make(chan struct{})
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	if timeLimit > 0 {
		timer := time.NewTimer(time.Duration(timeLimit) * time.Second)
		select {
		case <-sigChan:
		case <-stopChan:
		case <-timer.C:
		}
		timer.Stop()
	} else {
		select {
		case <-sigChan:
		case <-stopChan:
		}
	}

	signal.Stop(sigChan)
	close(sigChan)

	if cb != nil && cb.OnRecordingEnded != nil {
		cb.OnRecordingEnded()
	}

	stopResult, err := rpcCall[struct {
		Status   string `json:"status"`
		Duration int    `json:"duration"`
		URL      string `json:"url"`
	}](r, "device.screenrecord.stop", params{})
	if err != nil {
		return err
	}

	if stopResult.URL != "" {
		err = downloadFile(stopResult.URL, outputPath, cb)
		if err != nil {
			return fmt.Errorf("failed to download recording: %w", err)
		}
	}

	return nil
}

func (r *RemoteDevice) StartScreenCapture(config ScreenCaptureConfig) error {
	return fmt.Errorf("screen capture is not supported on remote devices")
}

func (r *RemoteDevice) ListCrashReports() ([]CrashReport, error) {
	return rpcCall[[]CrashReport](r, "device.crashes.list", params{})
}

func (r *RemoteDevice) GetCrashReport(id string) ([]byte, error) {
	type crashGetResult struct {
		Content string `json:"content"`
	}
	result, err := rpcCall[crashGetResult](r, "device.crashes.get", params{"id": id})
	if err != nil {
		return nil, err
	}
	return []byte(result.Content), nil
}

func (r *RemoteDevice) StreamLogs(ctx context.Context, onLog func(LogEntry) bool) error {
	return fmt.Errorf("device logs not yet supported for remote devices")
}
