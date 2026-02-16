package wda

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mobile-next/mobilecli/types"
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
			Timeout: 60 * time.Second,
		},
	}
}

// Re-export shared types for use within the wda package
type TapAction = types.TapAction
type WindowSize = types.WindowSize
type Size = types.Size
type ActiveAppInfo = types.ActiveAppInfo

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
