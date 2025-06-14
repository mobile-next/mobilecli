package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mobile-next/mobilectl/devices"
	"github.com/mobile-next/mobilectl/utils"
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
	DeviceID string `json:"device_id"`
	Format   string `json:"format,omitempty"`  // "png" or "jpeg"
	Quality  int    `json:"quality,omitempty"` // 1-100, only used for JPEG
}

func StartServer(addr string) error {
	mux := http.NewServeMux()

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

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  ReadTimeout,
		WriteTimeout: WriteTimeout,
		IdleTimeout:  IdleTimeout,
	}

	log.Printf("Starting server on %s...", server.Addr)
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

	var result interface{}
	var err error

	switch req.Method {
	case "list_devices":
		result, err = handleDevicesList()
	case "take_screenshot":
		result, err = handleScreenshot(req.Params)
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
	return devices.GetDeviceInfoList()
}

func handleScreenshot(params json.RawMessage) (interface{}, error) {
	var screenshotParams ScreenshotParams
	if err := json.Unmarshal(params, &screenshotParams); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v", err)
	}

	if screenshotParams.DeviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	if screenshotParams.Format != "" {
		if screenshotParams.Format != "png" && screenshotParams.Format != "jpeg" {
			return nil, fmt.Errorf("invalid format '%s'. Supported formats are 'png' and 'jpeg'", screenshotParams.Format)
		}
	} else {
		screenshotParams.Format = "png" // Default to PNG
	}

	if screenshotParams.Format == "jpeg" {
		if screenshotParams.Quality < 1 || screenshotParams.Quality > 100 {
			screenshotParams.Quality = 90 // Default quality for JPEG
		}
	}

	allDevices, err := devices.GetAllControllableDevices()
	if err != nil {
		return nil, fmt.Errorf("error getting devices: %v", err)
	}

	var targetDevice devices.ControllableDevice
	for _, d := range allDevices {
		if d.ID() == screenshotParams.DeviceID {
			targetDevice = d
			break
		}
	}

	if targetDevice == nil {
		return nil, fmt.Errorf("device not found: %s", screenshotParams.DeviceID)
	}

	imageBytes, err := targetDevice.TakeScreenshot()
	if err != nil {
		return nil, fmt.Errorf("error taking screenshot: %v", err)
	}

	if screenshotParams.Format == "jpeg" {
		convertedBytes, err := utils.ConvertPngToJpeg(imageBytes, screenshotParams.Quality)
		if err != nil {
			return nil, fmt.Errorf("error converting to JPEG: %v", err)
		}
		imageBytes = convertedBytes
	}

	// Return base64 encoded image
	return map[string]interface{}{
		"format": screenshotParams.Format,
		"data":   fmt.Sprintf("data:image/%s;base64,%s", screenshotParams.Format, base64.StdEncoding.EncodeToString(imageBytes)),
	}, nil
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
