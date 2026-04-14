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
			Timeout: 60 * time.Second,
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

