package server

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/mobile-next/mobilecli/utils"
)

type wsConnection struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func newUpgrader(enableCORS bool) *websocket.Upgrader {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	if enableCORS {
		upgrader.CheckOrigin = func(r *http.Request) bool {
			return true
		}
	} else {
		upgrader.CheckOrigin = isSameOrigin
	}

	return &upgrader
}

func handleWebSocket(w http.ResponseWriter, r *http.Request, enableCORS bool) {
	conn, err := newUpgrader(enableCORS).Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	wsConn := &wsConnection{conn: conn}

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			// connection closed or error
			utils.Verbose("WebSocket connection closed: %v", err)
			break
		}

		if messageType != websocket.TextMessage {
			wsConn.sendError(nil, ErrCodeInvalidRequest, "Invalid Request", "only text messages accepted for requests")
			continue
		}

		handleWSMessage(wsConn, message)
	}
}

func isSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}

	return originURL.Host == r.Host
}

func handleWSMessage(wsConn *wsConnection, message []byte) {
	var req JSONRPCRequest
	if err := json.Unmarshal(message, &req); err != nil {
		wsConn.sendError(nil, ErrCodeParseError, "Parse error", "expecting jsonrpc payload")
		return
	}

	if req.JSONRPC != "2.0" {
		wsConn.sendError(req.ID, ErrCodeInvalidRequest, "Invalid Request", "'jsonrpc' must be '2.0'")
		return
	}

	if req.ID == nil {
		wsConn.sendError(nil, ErrCodeInvalidRequest, "Invalid Request", "'id' field is required")
		return
	}

	if req.Method == "" {
		wsConn.sendError(req.ID, ErrCodeInvalidRequest, "Invalid Request", "'method' is required")
		return
	}

	// screencapture is not supported over WebSocket
	if req.Method == "screencapture" {
		wsConn.sendError(req.ID, ErrCodeMethodNotFound, "Method not supported", "screencapture not supported over WebSocket, use HTTP /rpc endpoint")
		return
	}

	utils.Info("WebSocket Request ID: %v, Method: %s, Params: %s", req.ID, req.Method, string(req.Params))

	handleWSMethodCall(wsConn, req)
}

func handleWSMethodCall(wsConn *wsConnection, req JSONRPCRequest) {
	registry := GetMethodRegistry()
	handler, exists := registry[req.Method]
	if !exists {
		wsConn.sendError(req.ID, ErrCodeMethodNotFound, "Method not found", req.Method+" not found")
		return
	}

	result, err := handler(req.Params)
	if err != nil {
		log.Printf("Error executing method %s: %v", req.Method, err)
		wsConn.sendError(req.ID, ErrCodeServerError, "Server error", err.Error())
		return
	}

	wsConn.sendResponse(req.ID, result)
}

func (wsc *wsConnection) sendResponse(id interface{}, result interface{}) error {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}
	return wsc.sendJSON(response)
}

func (wsc *wsConnection) sendError(id interface{}, code int, message string, data interface{}) error {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		Error: map[string]interface{}{
			"code":    code,
			"message": message,
			"data":    data,
		},
		ID: id,
	}
	return wsc.sendJSON(response)
}

func (wsc *wsConnection) sendJSON(v interface{}) error {
	wsc.writeMu.Lock()
	defer wsc.writeMu.Unlock()
	return wsc.conn.WriteJSON(v)
}
