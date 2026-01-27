package server

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mobile-next/mobilecli/utils"
)

type wsConnection struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

type validationError struct {
	code    int
	message string
	data    interface{}
}

const (
	wsMaxMessageSize = 64 * 1024
	wsWriteWait      = 10 * time.Second
	wsPongWait       = 60 * time.Second
	wsPingPeriod     = (wsPongWait * 9) / 10

	jsonRPCVersion        = "2.0"
	errMsgParseError      = "expecting jsonrpc payload"
	errMsgInvalidJSONRPC  = "'jsonrpc' must be '2.0'"
	errMsgIDRequired      = "'id' field is required"
	errMsgMethodRequired  = "'method' is required"
	errMsgTextOnly        = "only text messages accepted for requests"
	errMsgScreencapture   = "screencapture not supported over WebSocket, use HTTP /rpc endpoint"
	errTitleParseError    = "Parse error"
	errTitleInvalidReq    = "Invalid Request"
	errTitleMethodNotSupp = "Method not supported"
)

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

func upgradeConnection(w http.ResponseWriter, r *http.Request, upgrader *websocket.Upgrader) (*websocket.Conn, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func configureConnection(conn *websocket.Conn) {
	conn.SetReadLimit(wsMaxMessageSize)
	if err := conn.SetReadDeadline(time.Now().Add(wsPongWait)); err != nil {
		utils.Verbose("failed to set read deadline: %v", err)
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})
}

func startPingRoutine(wsConn *wsConnection) func() {
	pingDone := make(chan struct{})
	go pingLoop(wsConn, pingDone)
	return func() { close(pingDone) }
}

func pingLoop(wsConn *wsConnection, done <-chan struct{}) {
	ticker := time.NewTicker(wsPingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			wsConn.writeMu.Lock()
			if err := wsConn.conn.SetWriteDeadline(time.Now().Add(wsWriteWait)); err != nil {
				utils.Verbose("failed to set write deadline: %v", err)
				wsConn.writeMu.Unlock()
				return
			}
			if err := wsConn.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				wsConn.writeMu.Unlock()
				return
			}
			wsConn.writeMu.Unlock()
		case <-done:
			return
		}
	}
}

func readMessages(wsConn *wsConnection) {
	for {
		messageType, message, err := wsConn.conn.ReadMessage()
		if err != nil {
			utils.Verbose("WebSocket connection closed: %v", err)
			break
		}

		if messageType != websocket.TextMessage {
			wsConn.sendError(nil, ErrCodeInvalidRequest, errTitleInvalidReq, errMsgTextOnly)
			continue
		}

		handleWSMessage(wsConn, message)
	}
}

func NewWebSocketHandler(enableCORS bool) http.HandlerFunc {
	upgrader := newUpgrader(enableCORS)
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgradeConnection(w, r, upgrader)
		if err != nil {
			log.Printf("WebSocket upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		wsConn := &wsConnection{conn: conn}
		configureConnection(conn)
		stopPing := startPingRoutine(wsConn)
		defer stopPing()

		readMessages(wsConn)
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

func validateJSONRPCRequest(req JSONRPCRequest) *validationError {
	if req.JSONRPC != jsonRPCVersion {
		return &validationError{
			code:    ErrCodeInvalidRequest,
			message: errTitleInvalidReq,
			data:    errMsgInvalidJSONRPC,
		}
	}

	if req.ID == nil {
		return &validationError{
			code:    ErrCodeInvalidRequest,
			message: errTitleInvalidReq,
			data:    errMsgIDRequired,
		}
	}

	if req.Method == "" {
		return &validationError{
			code:    ErrCodeInvalidRequest,
			message: errTitleInvalidReq,
			data:    errMsgMethodRequired,
		}
	}

	if req.Method == "screencapture" {
		return &validationError{
			code:    ErrCodeMethodNotFound,
			message: errTitleMethodNotSupp,
			data:    errMsgScreencapture,
		}
	}

	return nil
}

func handleWSMessage(wsConn *wsConnection, message []byte) {
	var req JSONRPCRequest
	if err := json.Unmarshal(message, &req); err != nil {
		wsConn.sendError(nil, ErrCodeParseError, errTitleParseError, errMsgParseError)
		return
	}

	if validationErr := validateJSONRPCRequest(req); validationErr != nil {
		wsConn.sendError(req.ID, validationErr.code, validationErr.message, validationErr.data)
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
		JSONRPC: jsonRPCVersion,
		Result:  result,
		ID:      id,
	}
	return wsc.sendJSON(response)
}

func (wsc *wsConnection) sendError(id interface{}, code int, message string, data interface{}) error {
	response := JSONRPCResponse{
		JSONRPC: jsonRPCVersion,
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
