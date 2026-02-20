package devices

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/mobile-next/mobilecli/devices/wda"
)

// json-rpc structs defined locally to avoid import cycle with commands package
type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      int         `json:"id"`
}

type rpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

type RemoteDevice struct {
	deviceID string
	name     string
	platform string
	devType  string
	version  string
	state    string
	model    string
	token    string
	endpoint string
}

func NewRemoteDevice(info DeviceInfo, token string, endpoint string) *RemoteDevice {
	return &RemoteDevice{
		deviceID: info.ID,
		name:     info.Name,
		platform: info.Platform,
		devType:  info.Type,
		version:  info.Version,
		state:    info.State,
		model:    info.Model,
		token:    token,
		endpoint: endpoint,
	}
}

func (r *RemoteDevice) ID() string       { return r.deviceID }
func (r *RemoteDevice) Name() string     { return r.name }
func (r *RemoteDevice) Platform() string { return r.platform }
func (r *RemoteDevice) DeviceType() string {
	if r.devType != "" {
		return r.devType
	}
	return "remote"
}
func (r *RemoteDevice) Version() string { return r.version }
func (r *RemoteDevice) State() string   { return r.state }

func (r *RemoteDevice) StartAgent(config StartAgentConfig) error {
	return nil
}

func (r *RemoteDevice) callRPC(method string, params map[string]interface{}) (interface{}, error) {
	url := r.endpoint + "?token=" + r.token
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to pool server: %w", err)
	}
	defer conn.Close()

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	if err := conn.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("failed to send rpc request: %w", err)
	}

	var resp rpcResponse
	if err := conn.ReadJSON(&resp); err != nil {
		return nil, fmt.Errorf("failed to read rpc response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("rpc error: %v", resp.Error)
	}

	return resp.Result, nil
}

// remarshal converts an interface{} result to a typed struct via json round-trip
func remarshal(src interface{}, dst interface{}) error {
	data, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("failed to marshal rpc result: %w", err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("failed to unmarshal rpc result: %w", err)
	}
	return nil
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
	if err := remarshal(result, &screenshotResp); err != nil {
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
	if err := remarshal(result, &orientationResp); err != nil {
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
	if err := remarshal(result, &info); err != nil {
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
	if err := remarshal(result, &apps); err != nil {
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
	if err := remarshal(result, &app); err != nil {
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
	if err := remarshal(result, &resp); err != nil {
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
	if err := remarshal(result, &resp); err != nil {
		return nil, err
	}

	return resp.RawData, nil
}

func (r *RemoteDevice) InstallApp(path string) error {
	return fmt.Errorf("install app is not supported on remote devices")
}

func (r *RemoteDevice) UninstallApp(packageName string) (*InstalledAppInfo, error) {
	return nil, fmt.Errorf("uninstall app is not supported on remote devices")
}

func (r *RemoteDevice) StartScreenCapture(config ScreenCaptureConfig) error {
	return fmt.Errorf("screen capture is not supported on remote devices")
}
