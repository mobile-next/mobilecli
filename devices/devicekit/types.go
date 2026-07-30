package devicekit

import (
	"net/http"
	"strings"
	"time"
)

type DeviceKitClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewDeviceKitClient(hostPort string) *DeviceKitClient {
	baseURL := hostPort
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &DeviceKitClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
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
