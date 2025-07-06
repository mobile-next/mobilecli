package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/commands"
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

var okResponse = map[string]interface{}{"status": "ok"}

// JSONRPCRequest represents a JSON-RPC request
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      interface{}     `json:"id"`
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

// corsMiddleware handles CORS preflight requests and adds CORS headers to responses.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func StartServer(addr string, enableCORS bool) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", sendBanner)
	mux.HandleFunc("/rpc", handleJSONRPC)

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
		log.Printf("CORS enabled")
	}

	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  ReadTimeout,
		WriteTimeout: WriteTimeout,
		IdleTimeout:  IdleTimeout,
	}

	log.Printf("Starting server on http://%s...", server.Addr)
	return server.ListenAndServe()
}

func handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONRPCError(w, nil, ErrCodeParseError, "Parse error", err.Error())
		return
	}

	if req.JSONRPC != "2.0" {
		sendJSONRPCError(w, req.ID, ErrCodeInvalidRequest, "Invalid Request", "jsonrpc must be '2.0'")
		return
	}

	log.Printf("Request ID: %v, Method: %s, Params: %s", req.ID, req.Method, string(req.Params))

	var result interface{}
	var err error

	switch req.Method {
	case "devices":
		result, err = handleDevicesList()
	case "screenshot":
		result, err = handleScreenshot(req.Params)
	case "screencapture":
		err = handleScreenCapture(w, req.Params)
	case "io_tap":
		result, err = handleIoTap(req.Params)
	case "io_text":
		result, err = handleIoText(req.Params)
	case "io_button":
		result, err = handleIoButton(req.Params)
	case "url":
		result, err = handleURL(req.Params)
	case "device_info":
		result, err = handleDeviceInfo(req.Params)

	default:
		sendJSONRPCError(w, req.ID, ErrCodeMethodNotFound, "Method not found", fmt.Sprintf("Method '%s' not found", req.Method))
		return
	}

	if err != nil {
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
	json.NewEncoder(w).Encode(response)
}

func handleDevicesList() (interface{}, error) {
	response := commands.DevicesCommand()
	if response.Status == "error" {
		return nil, fmt.Errorf(response.Error)
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
		return nil, fmt.Errorf(response.Error)
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

func handleIoTap(params json.RawMessage) (interface{}, error) {
	var ioTapParams IoTapParams
	if err := json.Unmarshal(params, &ioTapParams); err != nil {
		return nil, err
	}

	req := commands.TapRequest{
		DeviceID: ioTapParams.DeviceID,
		X:        ioTapParams.X,
		Y:        ioTapParams.Y,
	}

	response := commands.TapCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf(response.Error)
	}

	return okResponse, nil
}

type IoTextParams struct {
	DeviceID string `json:"deviceId"`
	Text     string `json:"text"`
}

func handleIoText(params json.RawMessage) (interface{}, error) {
	var ioTextParams IoTextParams
	if err := json.Unmarshal(params, &ioTextParams); err != nil {
		return nil, err
	}

	req := commands.TextRequest{
		DeviceID: ioTextParams.DeviceID,
		Text:     ioTextParams.Text,
	}

	response := commands.TextCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf(response.Error)
	}

	return okResponse, nil
}

type IoButtonParams struct {
	DeviceID string `json:"deviceId"`
	Button   string `json:"button"`
}

type URLParams struct {
	DeviceID string `json:"deviceId"`
	URL      string `json:"url"`
}

type InfoParams struct {
	DeviceID string `json:"deviceId"`
}

func handleIoButton(params json.RawMessage) (interface{}, error) {
	var ioButtonParams IoButtonParams
	if err := json.Unmarshal(params, &ioButtonParams); err != nil {
		return nil, err
	}

	req := commands.ButtonRequest{
		DeviceID: ioButtonParams.DeviceID,
		Button:   ioButtonParams.Button,
	}

	response := commands.ButtonCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf(response.Error)
	}

	return okResponse, nil
}

func handleURL(params json.RawMessage) (interface{}, error) {
	var urlParams URLParams
	if err := json.Unmarshal(params, &urlParams); err != nil {
		return nil, err
	}

	req := commands.URLRequest{
		DeviceID: urlParams.DeviceID, // Can be empty for auto-selection
		URL:      urlParams.URL,
	}

	response := commands.URLCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf(response.Error)
	}

	return okResponse, nil
}

func handleDeviceInfo(params json.RawMessage) (interface{}, error) {
	var infoParams InfoParams
	if err := json.Unmarshal(params, &infoParams); err != nil {
		return nil, err
	}

	response := commands.InfoCommand(infoParams.DeviceID)
	if response.Status == "error" {
		return nil, fmt.Errorf(response.Error)
	}

	return response.Data, nil
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
	json.NewEncoder(w).Encode(response)
}

func sendBanner(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(okResponse)
}

func handleScreenCapture(w http.ResponseWriter, params json.RawMessage) error {

	http.NewResponseController(w).SetWriteDeadline(time.Now().Add(10 * time.Minute))

	var screenCaptureParams commands.ScreenCaptureRequest
	if err := json.Unmarshal(params, &screenCaptureParams); err != nil {
		return fmt.Errorf("invalid parameters: %v", err)
	}

	// Find the target device
	targetDevice, err := commands.FindDeviceOrAutoSelect(screenCaptureParams.DeviceID)
	if err != nil {
		return fmt.Errorf("error finding device: %v", err)
	}

	if screenCaptureParams.Format == "" || screenCaptureParams.Format != "mjpeg" {
		return fmt.Errorf("format must be 'mjpeg' for screen capture")
	}

	// Set headers for streaming response
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=BoundaryString")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	// Start screen capture and stream to the response writer
	err = targetDevice.StartScreenCapture(screenCaptureParams.Format, func(data []byte) bool {
		_, writeErr := w.Write(data)
		if writeErr != nil {
			fmt.Println("Error writing data:", writeErr)
			return false
		}

		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		return true
	})

	if err != nil {
		return fmt.Errorf("error starting screen capture: %v", err)
	}

	return nil
}
