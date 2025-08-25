package wda

import (
	"net/http"
	"strings"
	"time"
)

type WdaClient struct {
	baseURL    string
	httpClient *http.Client
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
	Duration int    `json:"duration,omitempty"`
	X        int    `json:"x,omitempty"`
	Y        int    `json:"y,omitempty"`
	Button   int    `json:"button,omitempty"`
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
