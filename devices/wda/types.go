package wda

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

type WdaClient struct {
	baseURL    string
	httpClient *http.Client
	sessionId  string
	mu         sync.Mutex
}

func NewWdaClient(hostPort string) *WdaClient {
	baseURL := hostPort
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &WdaClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type TapAction struct {
	Type     string `json:"type"`
	Duration int    `json:"duration"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Button   int    `json:"button"`
}

type PointerParameters struct {
	PointerType string `json:"pointerType"`
}

type Pointer struct {
	Type       string            `json:"type"`
	ID         string            `json:"id"`
	Parameters PointerParameters `json:"parameters"`
	Actions    []TapAction       `json:"actions"`
}

type ActionsRequest struct {
	Actions []Pointer `json:"actions"`
}
