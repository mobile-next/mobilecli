package devices

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/devices/wda"
	"github.com/mobile-next/mobilecli/rpc"
	"github.com/mobile-next/mobilecli/utils"
)

type RemoteDevice struct {
	deviceID   string
	name       string
	platform   string
	deviceType string
	version    string
	state      string
	model      string
	token      string
	endpoint   string
}

func NewRemoteDevice(info DeviceInfo, token string, endpoint string) *RemoteDevice {
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
		endpoint:   endpoint,
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

func (r *RemoteDevice) callRPC(method string, params map[string]interface{}) (interface{}, error) {
	var result interface{}
	if err := rpc.Call(r.token, method, params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *RemoteDevice) TakeScreenshot() ([]byte, error) {
	result, err := r.callRPC("device.screenshot", map[string]interface{}{
		"deviceId": r.deviceID,
	})
	if err != nil {
		return nil, err
	}

	var screenshotResp struct {
		Data string `json:"data"`
	}
	if err := rpc.Remarshal(result, &screenshotResp); err != nil {
		return nil, err
	}

	data := screenshotResp.Data
	// handle data URI format: data:image/png;base64,...
	if idx := strings.Index(data, ","); idx != -1 {
		data = data[idx+1:]
	}

	return base64.StdEncoding.DecodeString(data)
}

func (r *RemoteDevice) Tap(x, y int) error {
	_, err := r.callRPC("device.io.tap", map[string]interface{}{
		"deviceId": r.deviceID,
		"x":        x,
		"y":        y,
	})
	return err
}

func (r *RemoteDevice) LongPress(x, y, duration int) error {
	_, err := r.callRPC("device.io.longpress", map[string]interface{}{
		"deviceId": r.deviceID,
		"x":        x,
		"y":        y,
		"duration": duration,
	})
	return err
}

func (r *RemoteDevice) Swipe(x1, y1, x2, y2 int) error {
	_, err := r.callRPC("device.io.swipe", map[string]interface{}{
		"deviceId": r.deviceID,
		"x1":       x1,
		"y1":       y1,
		"x2":       x2,
		"y2":       y2,
	})
	return err
}

func (r *RemoteDevice) Gesture(actions []wda.TapAction) error {
	_, err := r.callRPC("device.io.gesture", map[string]interface{}{
		"deviceId": r.deviceID,
		"actions":  actions,
	})
	return err
}

func (r *RemoteDevice) SendKeys(text string) error {
	_, err := r.callRPC("device.io.text", map[string]interface{}{
		"deviceId": r.deviceID,
		"text":     text,
	})
	return err
}

func (r *RemoteDevice) PressButton(key string) error {
	_, err := r.callRPC("device.io.button", map[string]interface{}{
		"deviceId": r.deviceID,
		"button":   key,
	})
	return err
}

func (r *RemoteDevice) OpenURL(url string) error {
	_, err := r.callRPC("device.url", map[string]interface{}{
		"deviceId": r.deviceID,
		"url":      url,
	})
	return err
}

func (r *RemoteDevice) LaunchApp(bundleID string) error {
	_, err := r.callRPC("device.apps.launch", map[string]interface{}{
		"deviceId": r.deviceID,
		"bundleId": bundleID,
	})
	return err
}

func (r *RemoteDevice) TerminateApp(bundleID string) error {
	_, err := r.callRPC("device.apps.terminate", map[string]interface{}{
		"deviceId": r.deviceID,
		"bundleId": bundleID,
	})
	return err
}

func (r *RemoteDevice) Boot() error {
	_, err := r.callRPC("device.boot", map[string]interface{}{
		"deviceId": r.deviceID,
	})
	return err
}

func (r *RemoteDevice) Shutdown() error {
	_, err := r.callRPC("device.shutdown", map[string]interface{}{
		"deviceId": r.deviceID,
	})
	return err
}

func (r *RemoteDevice) Reboot() error {
	_, err := r.callRPC("device.reboot", map[string]interface{}{
		"deviceId": r.deviceID,
	})
	return err
}

func (r *RemoteDevice) GetOrientation() (string, error) {
	result, err := r.callRPC("device.io.orientation.get", map[string]interface{}{
		"deviceId": r.deviceID,
	})
	if err != nil {
		return "", err
	}

	var orientationResp struct {
		Orientation string `json:"orientation"`
	}
	if err := rpc.Remarshal(result, &orientationResp); err != nil {
		return "", err
	}

	return orientationResp.Orientation, nil
}

func (r *RemoteDevice) SetOrientation(orientation string) error {
	_, err := r.callRPC("device.io.orientation.set", map[string]interface{}{
		"deviceId":    r.deviceID,
		"orientation": orientation,
	})
	return err
}

func (r *RemoteDevice) Info() (*FullDeviceInfo, error) {
	result, err := r.callRPC("device.info", map[string]interface{}{
		"deviceId": r.deviceID,
	})
	if err != nil {
		return nil, err
	}

	var info FullDeviceInfo
	if err := rpc.Remarshal(result, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

func (r *RemoteDevice) ListApps() ([]InstalledAppInfo, error) {
	result, err := r.callRPC("device.apps.list", map[string]interface{}{
		"deviceId": r.deviceID,
	})
	if err != nil {
		return nil, err
	}

	var apps []InstalledAppInfo
	if err := rpc.Remarshal(result, &apps); err != nil {
		return nil, err
	}

	return apps, nil
}

func (r *RemoteDevice) GetForegroundApp() (*ForegroundAppInfo, error) {
	result, err := r.callRPC("device.apps.foreground", map[string]interface{}{
		"deviceId": r.deviceID,
	})
	if err != nil {
		return nil, err
	}

	var app ForegroundAppInfo
	if err := rpc.Remarshal(result, &app); err != nil {
		return nil, err
	}

	return &app, nil
}

func (r *RemoteDevice) DumpSource() ([]ScreenElement, error) {
	result, err := r.callRPC("device.dump.ui", map[string]interface{}{
		"deviceId": r.deviceID,
	})
	if err != nil {
		return nil, err
	}

	var resp struct {
		Elements []ScreenElement `json:"elements"`
	}
	if err := rpc.Remarshal(result, &resp); err != nil {
		return nil, err
	}

	return resp.Elements, nil
}

func (r *RemoteDevice) DumpSourceRaw() (interface{}, error) {
	result, err := r.callRPC("device.dump.ui", map[string]interface{}{
		"deviceId": r.deviceID,
		"format":   "raw",
	})
	if err != nil {
		return nil, err
	}

	var resp struct {
		RawData interface{} `json:"rawData"`
	}
	if err := rpc.Remarshal(result, &resp); err != nil {
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

	req, err := http.NewRequest(http.MethodPut, uploadURL, f)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}
	req.ContentLength = fi.Size()

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

func (r *RemoteDevice) InstallApp(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	filename := sanitizeFilename(filepath.Base(path))

	result, err := r.callRPC("uploads.create", map[string]interface{}{
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

	_, err = r.callRPC("device.apps.install", map[string]interface{}{
		"deviceId": r.deviceID,
		"uploadId": upload.UploadID,
	})
	return err
}

func (r *RemoteDevice) UninstallApp(packageName string) (*InstalledAppInfo, error) {
	return nil, fmt.Errorf("uninstall app is not supported on remote devices")
}

func (r *RemoteDevice) StartScreenCapture(config ScreenCaptureConfig) error {
	return fmt.Errorf("screen capture is not supported on remote devices")
}
