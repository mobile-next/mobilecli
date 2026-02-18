package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/mobile-next/mobilecli/commands"
	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/utils"
)

const (
	// Parse error: Invalid JSON was received by the server
	ErrCodeParseError = -32700

	// Invalid Request: The JSON sent is not a valid Request object
	ErrCodeInvalidRequest = -32600

	// Method not found: The method does not exist / is not available
	ErrCodeMethodNotFound = -32601

	// Server error: Internal JSON-RPC error
	ErrCodeServerError = -32000

	// Invalid params: Invalid method parameters
	ErrCodeInvalidParams = -32602

	// Internal error: Internal JSON-RPC error
	ErrCodeInternalError = -32603
)

// Server timeouts
const (
	ReadTimeout  = 10 * time.Second
	WriteTimeout = 10 * time.Second
	IdleTimeout  = 120 * time.Second
)

var Version = "dev"

var okResponse = map[string]interface{}{"status": "ok"}

// StreamSession represents a screen capture streaming session
type StreamSession struct {
	ID        string
	DeviceID  string
	Format    string  // "mjpeg" or "avc"
	Quality   int
	Scale     float64
	CreatedAt time.Time
	ExpiresAt time.Time // CreatedAt + 1 minute
	InUse     bool      // prevents duplicate connections
}

// SessionManager manages screen capture streaming sessions
type SessionManager struct {
	sessions map[string]*StreamSession
	mu       sync.RWMutex
}

// global session manager instance
var sessionManager *SessionManager

// global shutdown channel for JSON-RPC shutdown command
var shutdownChan chan os.Signal

type JSONRPCRequest struct {
	// these fields are all omitempty, so we can report back to client if they are missing
	JSONRPC string          `json:"jsonrpc,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// ScreenshotParams represents the parameters for the screenshot request
type ScreenshotParams struct {
	DeviceID string `json:"deviceId"`
	Format   string `json:"format,omitempty"`  // "png" or "jpeg"
	Quality  int    `json:"quality,omitempty"` // 1-100, only used for JPEG
}

// DevicesParams represents the parameters for the devices request
type DevicesParams struct {
	IncludeOffline bool   `json:"includeOffline,omitempty"`
	Platform       string `json:"platform,omitempty"`
	Type           string `json:"type,omitempty"`
}

// corsMiddleware handles CORS preflight requests and adds CORS headers to responses.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// AddSession adds a new session to the manager, sweeps expired sessions first
func (sm *SessionManager) AddSession(session *StreamSession) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// sweep expired sessions first
	now := time.Now()
	for id, s := range sm.sessions {
		// remove sessions that are expired and not in use
		if now.After(s.ExpiresAt) && !s.InUse {
			delete(sm.sessions, id)
		}
	}

	// check session limit
	if len(sm.sessions) >= 128 {
		return fmt.Errorf("session limit reached (128), please try again later")
	}

	sm.sessions[session.ID] = session
	return nil
}

// GetSession retrieves a session by ID, returns error if not found or expired for new connections
func (sm *SessionManager) GetSession(id string) (*StreamSession, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[id]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}

	// check if expired for NEW connections (in-use sessions are allowed to continue)
	if time.Now().After(session.ExpiresAt) && !session.InUse {
		return nil, fmt.Errorf("session not found")
	}

	return session, nil
}

// MarkInUse atomically marks a session as in use, returns error if already in use
func (sm *SessionManager) MarkInUse(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[id]
	if !exists {
		return fmt.Errorf("session not found")
	}

	if session.InUse {
		return fmt.Errorf("session already in use")
	}

	session.InUse = true
	return nil
}

// RemoveSession removes a session from the manager
func (sm *SessionManager) RemoveSession(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, id)
}

func StartServer(addr string, enableCORS bool) error {
	// create shutdown hook for cleanup tracking
	hook := devices.NewShutdownHook()
	commands.SetShutdownHook(hook)

	// initialize session manager
	sessionManager = &SessionManager{
		sessions: make(map[string]*StreamSession),
	}

	// initialize shutdown channel for JSON-RPC shutdown command
	shutdownChan = make(chan os.Signal, 1)

	mux := http.NewServeMux()

	mux.HandleFunc("/", sendBanner)
	mux.HandleFunc("/rpc", handleJSONRPC)
	mux.HandleFunc("/ws", NewWebSocketHandler(enableCORS))
	mux.HandleFunc("/stream", handleStream)

	// if host is missing, default to localhost
	if !strings.Contains(addr, ":") {
		// convert addr to integer
		port, err := strconv.Atoi(addr)
		if err != nil {
			return fmt.Errorf("invalid port: %v", err)
		}

		addr = fmt.Sprintf(":%d", port)
	}

	var handler http.Handler = mux
	if enableCORS {
		handler = corsMiddleware(mux)
	}

	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  ReadTimeout,
		WriteTimeout: WriteTimeout,
		IdleTimeout:  IdleTimeout,
	}

	// channel to catch server errors
	serverErr := make(chan error, 1)

	// start server in goroutine
	go func() {
		utils.Info("Starting server on http://%s...", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	performShutdown := func() error {
		if err := hook.Shutdown(); err != nil {
			utils.Info("hook shutdown error: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			return fmt.Errorf("server shutdown error: %w", err)
		}

		utils.Info("Server stopped")
		return nil
	}

	// wait for shutdown signal or server error
	select {
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	case sig := <-sigChan:
		utils.Info("Received signal %v, shutting down gracefully...", sig)
		return performShutdown()
	case <-shutdownChan:
		utils.Info("Received shutdown command via JSON-RPC, shutting down gracefully...")
		return performShutdown()
	}
}

func handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONRPCError(w, nil, ErrCodeParseError, "Parse error", "expecting jsonrpc payload")
		return
	}

	if req.JSONRPC != "2.0" {
		sendJSONRPCError(w, req.ID, ErrCodeInvalidRequest, "Invalid Request", "'jsonrpc' must be '2.0'")
		return
	}

	if req.ID == nil {
		sendJSONRPCError(w, nil, ErrCodeInvalidRequest, "Invalid Request", "'id' field is required")
		return
	}

	utils.Info("Request ID: %v, Method: %s, Params: %s", req.ID, req.Method, string(req.Params))

	var result interface{}
	var err error

	// HTTP-specific: device_boot needs extended timeout (can take up to 2 minutes)
	if req.Method == "device_boot" {
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(3 * time.Minute))
	}

	// Use registry for all methods
	if req.Method == "" {
		err = fmt.Errorf("'method' is required")
	} else {
		registry := GetMethodRegistry()
		handler, exists := registry[req.Method]
		if exists {
			result, err = handler(req.Params)
		} else {
			sendJSONRPCError(w, req.ID, ErrCodeMethodNotFound, "Method not found", fmt.Sprintf("Method '%s' not found", req.Method))
			return
		}
	}

	if err != nil {
		log.Printf("Error decoding JSON-RPC request: %v", err)
		sendJSONRPCError(w, req.ID, ErrCodeServerError, "Server error", err.Error())
		return
	}

	sendJSONRPCResponse(w, req.ID, result)
}

func sendJSONRPCResponse(w http.ResponseWriter, id interface{}, result interface{}) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleDevicesList(params json.RawMessage) (interface{}, error) {
	// default to showing all devices if no params provided
	opts := devices.DeviceListOptions{
		IncludeOffline: false,
		Platform:       "",
		DeviceType:     "",
	}

	// parse params if provided
	if len(params) > 0 {
		var devicesParams DevicesParams
		if err := json.Unmarshal(params, &devicesParams); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		opts.IncludeOffline = devicesParams.IncludeOffline
		opts.Platform = devicesParams.Platform
		opts.DeviceType = devicesParams.Type
	}

	response := commands.DevicesCommand(opts)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}
	return response.Data, nil
}

func handleScreenshot(params json.RawMessage) (interface{}, error) {
	var screenshotParams ScreenshotParams
	if err := json.Unmarshal(params, &screenshotParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v", err)
	}

	req := commands.ScreenshotRequest{
		DeviceID:   screenshotParams.DeviceID,
		Format:     screenshotParams.Format,
		Quality:    screenshotParams.Quality,
		OutputPath: "-", // Always return base64 data for server
	}

	response := commands.ScreenshotCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	// Convert the response data to the expected server format
	if screenshotResp, ok := response.Data.(commands.ScreenshotResponse); ok {
		return map[string]interface{}{
			"format": screenshotResp.Format,
			"data":   fmt.Sprintf("data:image/%s;base64,%s", screenshotResp.Format, screenshotResp.Data),
		}, nil
	}

	return nil, fmt.Errorf("unexpected response format")
}

type IoTapParams struct {
	DeviceID string `json:"deviceId"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
}

type IoLongPressParams struct {
	DeviceID string `json:"deviceId"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Duration int    `json:"duration"`
}

type IoSwipeParams struct {
	DeviceID string `json:"deviceId"`
	X1       int    `json:"x1"`
	Y1       int    `json:"y1"`
	X2       int    `json:"x2"`
	Y2       int    `json:"y2"`
}

func handleIoTap(params json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("'params' is required with fields: deviceId, x, y")
	}

	var ioTapParams IoTapParams
	if err := json.Unmarshal(params, &ioTapParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId, x, y", err)
	}

	req := commands.TapRequest{
		DeviceID: ioTapParams.DeviceID,
		X:        ioTapParams.X,
		Y:        ioTapParams.Y,
	}

	response := commands.TapCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return okResponse, nil
}

func handleIoLongPress(params json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("'params' is required with fields: deviceId, x, y")
	}

	var ioLongPressParams IoLongPressParams
	if err := json.Unmarshal(params, &ioLongPressParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId, x, y", err)
	}

	// default duration to 500ms if not provided
	duration := ioLongPressParams.Duration
	if duration == 0 {
		duration = 500
	}

	req := commands.LongPressRequest{
		DeviceID: ioLongPressParams.DeviceID,
		X:        ioLongPressParams.X,
		Y:        ioLongPressParams.Y,
		Duration: duration,
	}

	response := commands.LongPressCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return okResponse, nil
}

func handleIoSwipe(params json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("'params' is required with fields: deviceId, x1, y1, x2, y2")
	}

	var ioSwipeParams IoSwipeParams
	if err := json.Unmarshal(params, &ioSwipeParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId, x1, y1, x2, y2", err)
	}

	if ioSwipeParams.DeviceID == "" {
		return nil, fmt.Errorf("'deviceId' is required")
	}

	// validate that coordinates are provided (x1,y1,x2,y2 must be present)
	var rawParams map[string]interface{}
	if err := json.Unmarshal(params, &rawParams); err != nil {
		return nil, fmt.Errorf("invalid parameters format")
	}

	requiredFields := []string{"x1", "y1", "x2", "y2"}
	for _, field := range requiredFields {
		if _, exists := rawParams[field]; !exists {
			return nil, fmt.Errorf("'%s' is required", field)
		}
	}

	req := commands.SwipeRequest{
		DeviceID: ioSwipeParams.DeviceID,
		X1:       ioSwipeParams.X1,
		Y1:       ioSwipeParams.Y1,
		X2:       ioSwipeParams.X2,
		Y2:       ioSwipeParams.Y2,
	}

	response := commands.SwipeCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return okResponse, nil
}

type IoTextParams struct {
	DeviceID string `json:"deviceId"`
	Text     string `json:"text"`
}

func handleIoText(params json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("'params' is required with fields: deviceId, text")
	}

	var ioTextParams IoTextParams
	if err := json.Unmarshal(params, &ioTextParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId, text", err)
	}

	req := commands.TextRequest{
		DeviceID: ioTextParams.DeviceID,
		Text:     ioTextParams.Text,
	}

	response := commands.TextCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return okResponse, nil
}

type IoButtonParams struct {
	DeviceID string `json:"deviceId"`
	Button   string `json:"button"`
}

type IoGestureParams struct {
	DeviceID string        `json:"deviceId"`
	Actions  []interface{} `json:"actions"`
}

type URLParams struct {
	DeviceID string `json:"deviceId"`
	URL      string `json:"url"`
}

type InfoParams struct {
	DeviceID string `json:"deviceId"`
}

type IoOrientationGetParams struct {
	DeviceID string `json:"deviceId"`
}

type IoOrientationSetParams struct {
	DeviceID    string `json:"deviceId"`
	Orientation string `json:"orientation"`
}

type DeviceBootParams struct {
	DeviceID string `json:"deviceId"`
}

type DeviceShutdownParams struct {
	DeviceID string `json:"deviceId"`
}

type DeviceRebootParams struct {
	DeviceID string `json:"deviceId"`
}

type DumpUIParams struct {
	DeviceID string `json:"deviceId"`
	Format   string `json:"format,omitempty"` // "json" or "raw"
}

type AppsLaunchParams struct {
	DeviceID string `json:"deviceId"`
	BundleID string `json:"bundleId"`
}

type AppsTerminateParams struct {
	DeviceID string `json:"deviceId"`
	BundleID string `json:"bundleId"`
}

type AppsListParams struct {
	DeviceID string `json:"deviceId"`
}

type AppsForegroundParams struct {
	DeviceID string `json:"deviceId"`
}

func handleIoButton(params json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("'params' is required with fields: deviceId, button")
	}

	var ioButtonParams IoButtonParams
	if err := json.Unmarshal(params, &ioButtonParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId, button", err)
	}

	req := commands.ButtonRequest{
		DeviceID: ioButtonParams.DeviceID,
		Button:   ioButtonParams.Button,
	}

	response := commands.ButtonCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return okResponse, nil
}

func handleIoGesture(params json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("'params' is required with fields: deviceId, actions")
	}

	var ioGestureParams IoGestureParams
	if err := json.Unmarshal(params, &ioGestureParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId, actions", err)
	}

	req := commands.GestureRequest{
		DeviceID: ioGestureParams.DeviceID,
		Actions:  ioGestureParams.Actions,
	}

	response := commands.GestureCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return okResponse, nil
}

func handleURL(params json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("'params' is required with fields: deviceId, url")
	}

	var urlParams URLParams
	if err := json.Unmarshal(params, &urlParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId, url", err)
	}

	req := commands.URLRequest{
		DeviceID: urlParams.DeviceID, // Can be empty for auto-selection
		URL:      urlParams.URL,
	}

	response := commands.URLCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return okResponse, nil
}

func handleDeviceInfo(params json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("'params' is required with fields: deviceId")
	}

	var infoParams InfoParams
	if err := json.Unmarshal(params, &infoParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId", err)
	}

	targetDevice, err := commands.FindDeviceOrAutoSelect(infoParams.DeviceID)
	if err != nil {
		return nil, fmt.Errorf("error finding device: %w", err)
	}

	err = targetDevice.StartAgent(devices.StartAgentConfig{
		Hook: commands.GetShutdownHook(),
	})
	if err != nil {
		return nil, fmt.Errorf("error starting agent: %w", err)
	}

	response := commands.InfoCommand(infoParams.DeviceID)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return response.Data, nil
}

func handleIoOrientationGet(params json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("'params' is required with fields: deviceId")
	}

	var orientationGetParams IoOrientationGetParams
	if err := json.Unmarshal(params, &orientationGetParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId", err)
	}

	req := commands.OrientationGetRequest{
		DeviceID: orientationGetParams.DeviceID,
	}

	response := commands.OrientationGetCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return response.Data, nil
}

func handleIoOrientationSet(params json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("'params' is required with fields: deviceId, orientation")
	}

	var orientationSetParams IoOrientationSetParams
	if err := json.Unmarshal(params, &orientationSetParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId, orientation", err)
	}

	req := commands.OrientationSetRequest{
		DeviceID:    orientationSetParams.DeviceID,
		Orientation: orientationSetParams.Orientation,
	}

	response := commands.OrientationSetCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return okResponse, nil
}

func handleDeviceBoot(params json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("'params' is required with fields: deviceId")
	}

	var bootParams DeviceBootParams
	if err := json.Unmarshal(params, &bootParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId", err)
	}

	req := commands.BootRequest{
		DeviceID: bootParams.DeviceID,
	}

	response := commands.BootCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return response.Data, nil
}

func handleDeviceShutdown(params json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("'params' is required with fields: deviceId")
	}

	var shutdownParams DeviceShutdownParams
	if err := json.Unmarshal(params, &shutdownParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId", err)
	}

	req := commands.ShutdownRequest{
		DeviceID: shutdownParams.DeviceID,
	}

	response := commands.ShutdownCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return response.Data, nil
}

func handleDeviceReboot(params json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("'params' is required with fields: deviceId")
	}

	var rebootParams DeviceRebootParams
	if err := json.Unmarshal(params, &rebootParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId", err)
	}

	req := commands.RebootRequest{
		DeviceID: rebootParams.DeviceID,
	}

	response := commands.RebootCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return response.Data, nil
}

func handleDumpUI(params json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("'params' is required with fields: deviceId")
	}

	var dumpUIParams DumpUIParams
	if err := json.Unmarshal(params, &dumpUIParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId, format (optional)", err)
	}

	req := commands.DumpUIRequest{
		DeviceID: dumpUIParams.DeviceID,
		Format:   dumpUIParams.Format,
	}

	response := commands.DumpUICommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return response.Data, nil
}

func handleAppsLaunch(params json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("'params' is required with fields: deviceId, bundleId")
	}

	var appsLaunchParams AppsLaunchParams
	if err := json.Unmarshal(params, &appsLaunchParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId, bundleId", err)
	}

	req := commands.AppRequest{
		DeviceID: appsLaunchParams.DeviceID,
		BundleID: appsLaunchParams.BundleID,
	}

	response := commands.LaunchAppCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return response.Data, nil
}

func handleAppsTerminate(params json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("'params' is required with fields: deviceId, bundleId")
	}

	var appsTerminateParams AppsTerminateParams
	if err := json.Unmarshal(params, &appsTerminateParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId, bundleId", err)
	}

	req := commands.AppRequest{
		DeviceID: appsTerminateParams.DeviceID,
		BundleID: appsTerminateParams.BundleID,
	}

	response := commands.TerminateAppCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return response.Data, nil
}

func handleAppsList(params json.RawMessage) (interface{}, error) {
	var appsListParams AppsListParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &appsListParams); err != nil {
			return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId (optional)", err)
		}
	}

	req := commands.ListAppsRequest{
		DeviceID: appsListParams.DeviceID,
	}

	response := commands.ListAppsCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return response.Data, nil
}

func handleAppsForeground(params json.RawMessage) (interface{}, error) {
	var appsForegroundParams AppsForegroundParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &appsForegroundParams); err != nil {
			return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId (optional)", err)
		}
	}

	req := commands.ForegroundAppRequest{
		DeviceID: appsForegroundParams.DeviceID,
	}

	response := commands.ForegroundAppCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return response.Data, nil
}

func handleServerInfo(params json.RawMessage) (interface{}, error) {
	return map[string]string{
		"name":    "mobilecli",
		"version": Version,
	}, nil
}

// handleServerShutdown initiates graceful server shutdown
func handleServerShutdown(params json.RawMessage) (interface{}, error) {
	// trigger shutdown in background (after response is sent)
	go func() {
		time.Sleep(100 * time.Millisecond) // allow response to be sent
		select {
		case shutdownChan <- syscall.SIGTERM:
		default:
		}
	}()

	return map[string]string{"status": "ok"}, nil
}

func sendJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		Error: map[string]interface{}{
			"code":    code,
			"message": message,
			"data":    data,
		},
		ID: id,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func sendBanner(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(okResponse)
}

// newJsonRpcNotification creates a JSON-RPC notification message
func newJsonRpcNotification(message string) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notification/message",
		"params": map[string]string{
			"message": message,
		},
	}
}

// handleScreenCaptureSession creates a streaming session and returns sessionUrl
func handleScreenCaptureSession(params json.RawMessage) (interface{}, error) {
	var screenCaptureParams commands.ScreenCaptureRequest
	if err := json.Unmarshal(params, &screenCaptureParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v", err)
	}

	// set default format if not provided
	if screenCaptureParams.Format == "" {
		screenCaptureParams.Format = "mjpeg"
	}

	// validate format
	if screenCaptureParams.Format != "mjpeg" && screenCaptureParams.Format != "avc" {
		return nil, fmt.Errorf("format must be 'mjpeg' or 'avc' for screen capture")
	}

	// validate device exists (early error detection)
	targetDevice, err := commands.FindDeviceOrAutoSelect(screenCaptureParams.DeviceID)
	if err != nil {
		return nil, fmt.Errorf("error finding device: %w", err)
	}

	// avc format validation based on device type
	if screenCaptureParams.Format == "avc" {
		if targetDevice.Platform() == "ios" && targetDevice.DeviceType() == "simulator" {
			return nil, fmt.Errorf("avc format is not supported on iOS simulators")
		}
	}

	// ensure session manager is initialized for non-server Execute usage
	if sessionManager == nil {
		sessionManager = &SessionManager{sessions: make(map[string]*StreamSession)}
	}

	// set defaults for quality and scale
	quality := screenCaptureParams.Quality
	if quality == 0 {
		quality = devices.DefaultQuality
	}

	scale := screenCaptureParams.Scale
	if scale == 0.0 {
		scale = devices.DefaultScale
	}

	// generate session ID
	sessionID := uuid.New().String()

	// pin resolved device ID (handles auto-select)
	resolvedDeviceID := screenCaptureParams.DeviceID
	if resolvedDeviceID == "" {
		resolvedDeviceID = targetDevice.ID()
	}

	// create session entry
	session := &StreamSession{
		ID:        sessionID,
		DeviceID:  resolvedDeviceID,
		Format:    screenCaptureParams.Format,
		Quality:   quality,
		Scale:     scale,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Minute),
		InUse:     false,
	}

	// store in session manager
	if err := sessionManager.AddSession(session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// return response with format and sessionUrl
	result := map[string]interface{}{
		"format":     screenCaptureParams.Format,
		"sessionUrl": fmt.Sprintf("/stream?s=%s", sessionID),
	}

	return result, nil
}

// handleStream handles the /stream endpoint for screen capture streaming
func handleStream(w http.ResponseWriter, r *http.Request) {
	// extract session ID from query parameter
	sessionID := r.URL.Query().Get("s")
	if sessionID == "" {
		http.Error(w, "Missing session ID", http.StatusBadRequest)
		return
	}

	// look up session
	session, err := sessionManager.GetSession(sessionID)
	if err != nil {
		http.Error(w, "Invalid or expired session", http.StatusNotFound)
		return
	}

	// mark session as in use (prevents duplicate connections)
	if err := sessionManager.MarkInUse(sessionID); err != nil {
		http.Error(w, "Session already in use", http.StatusConflict)
		return
	}

	// ensure cleanup on exit
	defer sessionManager.RemoveSession(sessionID)

	// set extended write deadline for long-running stream
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(10 * time.Minute))

	// find device
	targetDevice, err := commands.FindDeviceOrAutoSelect(session.DeviceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Device not found: %v", err), http.StatusNotFound)
		return
	}

	// set streaming headers based on format
	if session.Format == "mjpeg" {
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=BoundaryString")
	} else {
		// avc format
		w.Header().Set("Content-Type", "video/h264")
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	// setup progress callback for MJPEG format
	var progressCallback func(string)
	if session.Format == "mjpeg" {
		progressCallback = func(message string) {
			notification := newJsonRpcNotification(message)
			statusJSON, err := json.Marshal(notification)
			if err != nil {
				log.Printf("Failed to marshal progress message: %v", err)
				return
			}
			mimeMessage := fmt.Sprintf("--BoundaryString\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s\r\n", len(statusJSON), statusJSON)
			_, _ = w.Write([]byte(mimeMessage))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}

	// start agent
	err = targetDevice.StartAgent(devices.StartAgentConfig{
		OnProgress: progressCallback,
		Hook:       commands.GetShutdownHook(),
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Error starting agent: %v", err), http.StatusInternalServerError)
		return
	}

	// start screen capture and stream
	err = targetDevice.StartScreenCapture(devices.ScreenCaptureConfig{
		Format:     session.Format,
		Quality:    session.Quality,
		Scale:      session.Scale,
		OnProgress: progressCallback,
		OnData: func(data []byte) bool {
			_, writeErr := w.Write(data)
			if writeErr != nil {
				fmt.Println("Error writing data:", writeErr)
				return false
			}

			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}

			return true
		},
	})

	if err != nil {
		// can't send HTTP error after streaming started, just log
		log.Printf("Error starting screen capture: %v", err)
		return
	}

	// session cleaned up by defer
}

func handleScreenCapture(r *http.Request, w http.ResponseWriter, params json.RawMessage) error {

	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(10 * time.Minute))

	var screenCaptureParams commands.ScreenCaptureRequest
	if err := json.Unmarshal(params, &screenCaptureParams); err != nil {
		return fmt.Errorf("invalid parameters: %v", err)
	}

	// Find the target device
	targetDevice, err := commands.FindDeviceOrAutoSelect(screenCaptureParams.DeviceID)
	if err != nil {
		return fmt.Errorf("error finding device: %w", err)
	}

	// Set default format if not provided
	if screenCaptureParams.Format == "" {
		screenCaptureParams.Format = "mjpeg"
	}

	// Validate format
	if screenCaptureParams.Format != "mjpeg" && screenCaptureParams.Format != "avc" {
		return fmt.Errorf("format must be 'mjpeg' or 'avc' for screen capture")
	}

	// avc format is supported on Android and iOS real devices (not simulators)
	if screenCaptureParams.Format == "avc" {
		if targetDevice.Platform() == "ios" && targetDevice.DeviceType() == "simulator" {
			return fmt.Errorf("avc format is not supported on iOS simulators")
		}
	}

	// Set defaults if not provided
	quality := screenCaptureParams.Quality
	if quality == 0 {
		quality = devices.DefaultQuality
	}

	scale := screenCaptureParams.Scale
	if scale == 0.0 {
		scale = devices.DefaultScale
	}

	// Set headers for streaming response based on format
	if screenCaptureParams.Format == "mjpeg" {
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=BoundaryString")
	} else {
		// avc format
		w.Header().Set("Content-Type", "video/h264")
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	// progress callback sends JSON-RPC notifications through the MJPEG stream
	// only used for MJPEG format, not for AVC
	var progressCallback func(string)
	if screenCaptureParams.Format == "mjpeg" {
		progressCallback = func(message string) {
			notification := newJsonRpcNotification(message)
			statusJSON, err := json.Marshal(notification)
			if err != nil {
				log.Printf("Failed to marshal progress message: %v", err)
				return
			}
			mimeMessage := fmt.Sprintf("--BoundaryString\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s\r\n", len(statusJSON), statusJSON)
			_, _ = w.Write([]byte(mimeMessage))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}

	err = targetDevice.StartAgent(devices.StartAgentConfig{
		OnProgress: progressCallback,
		Hook:       commands.GetShutdownHook(),
	})
	if err != nil {
		return fmt.Errorf("error starting agent: %w", err)
	}

	// start screen capture and stream to the response writer
	err = targetDevice.StartScreenCapture(devices.ScreenCaptureConfig{
		Format:     screenCaptureParams.Format,
		Quality:    quality,
		Scale:      scale,
		OnProgress: progressCallback,
		OnData: func(data []byte) bool {
			_, writeErr := w.Write(data)
			if writeErr != nil {
				fmt.Println("Error writing data:", writeErr)
				return false
			}

			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}

			return true
		},
	})

	if err != nil {
		return fmt.Errorf("error starting screen capture: %v", err)
	}

	return nil
}
