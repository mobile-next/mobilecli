package devicekit

import (
	"net/http"
	"net/url"
	"strconv"
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

// Port returns the port this client talks to, parsed from its base URL.
func (c *DeviceKitClient) Port() int {
	parsed, err := url.Parse(c.baseURL)
	if err != nil {
		return 0
	}

	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		return 0
	}

	return port
}

type TapAction struct {
	Type     string `json:"type"`
	Duration int    `json:"duration"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Button   int    `json:"button"`
}
