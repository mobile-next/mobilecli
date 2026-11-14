package cli

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/mobile-next/mobilecli/devices"
	"github.com/spf13/cobra"
)

var (
	networkPauses    []string
	networkRequestID string
	networkURL       string
	networkMethod    string
	networkHeaders   []string
	networkBody      string
	networkStatus    int
	networkError     string
)

var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "Capture and intercept the HTTP traffic of a debuggable app",
	Long: `Observe and intercept the network calls of a debuggable app you own
(Android debuggable build, or iOS get-task-allow / Debug build).

'network listen' streams a paused event for each request that matches a --pause
glob; answer it from another terminal (or a coprocess) with 'network continue',
'network fulfill', or 'network abort', keyed by the requestId in the event.
Requests that match no --pause are never touched, and a paused request you never
answer continues unchanged.`,
}

var networkListenCmd = &cobra.Command{
	Use:   "listen",
	Short: "Stream paused network requests from a debuggable app",
	Long: `Enables interception and streams one JSON event per matched request.
Press Ctrl+C to stop; stopping releases any request still paused.

Each --pause is a URL glob, optionally prefixed with an HTTP method:
  --pause '**/api/**'
  --pause 'POST **/login'
Repeat --pause for multiple routes. With no --pause, every request is paused.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		routes, err := parseNetworkRoutes(networkPauses)
		if err != nil {
			return err
		}
		return streamViaDaemon("cli.network.listen", commands.NetworkListenRequest{
			DeviceID: deviceId,
			Routes:   routes,
		})
	},
}

var networkContinueCmd = &cobra.Command{
	Use:   "continue",
	Short: "Let a paused request proceed, optionally with overrides",
	RunE: func(cmd *cobra.Command, args []string) error {
		headers, err := parseNetworkHeaders(networkHeaders)
		if err != nil {
			return err
		}
		body, err := readNetworkBody(networkBody)
		if err != nil {
			return err
		}
		return runViaDaemon("cli.network.continue", commands.NetworkContinueRequest{
			DeviceID:  deviceId,
			RequestID: networkRequestID,
			URL:       networkURL,
			Method:    networkMethod,
			Headers:   headers,
			Body:      body,
		})
	},
}

var networkFulfillCmd = &cobra.Command{
	Use:   "fulfill",
	Short: "Answer a paused request with a synthetic response",
	RunE: func(cmd *cobra.Command, args []string) error {
		headers, err := parseNetworkHeaders(networkHeaders)
		if err != nil {
			return err
		}
		body, err := readNetworkBody(networkBody)
		if err != nil {
			return err
		}
		return runViaDaemon("cli.network.fulfill", commands.NetworkFulfillRequest{
			DeviceID:  deviceId,
			RequestID: networkRequestID,
			Status:    networkStatus,
			Headers:   headers,
			Body:      body,
		})
	},
}

var networkAbortCmd = &cobra.Command{
	Use:   "abort",
	Short: "Fail a paused request",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runViaDaemon("cli.network.abort", commands.NetworkAbortRequest{
			DeviceID:  deviceId,
			RequestID: networkRequestID,
			ErrorCode: networkError,
		})
	},
}

// parseNetworkRoutes turns "--pause" values into routes. A value may be a bare
// URL glob, or "METHOD glob" to restrict to one method.
func parseNetworkRoutes(pauses []string) ([]devices.NetworkRoute, error) {
	routes := make([]devices.NetworkRoute, 0, len(pauses))
	for _, p := range pauses {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		method := ""
		glob := p
		if fields := strings.SplitN(p, " ", 2); len(fields) == 2 && isHTTPMethod(fields[0]) {
			method = strings.ToUpper(fields[0])
			glob = strings.TrimSpace(fields[1])
		}
		if glob == "" {
			return nil, fmt.Errorf("invalid --pause %q: empty url pattern", p)
		}
		routes = append(routes, devices.NetworkRoute{URL: glob, Method: method})
	}
	return routes, nil
}

func isHTTPMethod(s string) bool {
	switch strings.ToUpper(s) {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "TRACE", "CONNECT":
		return true
	}
	return false
}

// parseNetworkHeaders turns "--header K=V" values into a map.
func parseNetworkHeaders(headers []string) (map[string]string, error) {
	if len(headers) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(headers))
	for _, h := range headers {
		k, v, ok := strings.Cut(h, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid --header %q: expected key=value", h)
		}
		out[k] = v
	}
	return out, nil
}

// readNetworkBody returns the base64 of the body. "@path" reads a file; anything
// else is taken literally. An empty value stays empty (no body override).
func readNetworkBody(body string) (string, error) {
	if body == "" {
		return "", nil
	}
	raw := []byte(body)
	if strings.HasPrefix(body, "@") {
		data, err := os.ReadFile(body[1:])
		if err != nil {
			return "", fmt.Errorf("read --body file: %w", err)
		}
		raw = data
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func init() {
	rootCmd.AddCommand(networkCmd)
	networkCmd.AddCommand(networkListenCmd, networkContinueCmd, networkFulfillCmd, networkAbortCmd)

	networkListenCmd.Flags().StringVar(&deviceId, "device", "", "ID of the device to listen on")
	networkListenCmd.Flags().StringArrayVar(&networkPauses, "pause", nil, "URL glob to pause on, optionally 'METHOD glob' (repeatable)")

	for _, c := range []*cobra.Command{networkContinueCmd, networkFulfillCmd, networkAbortCmd} {
		c.Flags().StringVar(&deviceId, "device", "", "ID of the device")
		c.Flags().StringVar(&networkRequestID, "id", "", "requestId of the paused request (required)")
		_ = c.MarkFlagRequired("id")
	}

	networkContinueCmd.Flags().StringVar(&networkURL, "url", "", "override the request URL")
	networkContinueCmd.Flags().StringVar(&networkMethod, "method", "", "override the HTTP method")
	networkContinueCmd.Flags().StringArrayVar(&networkHeaders, "header", nil, "override/add a header, key=value (repeatable)")
	networkContinueCmd.Flags().StringVar(&networkBody, "body", "", "override the request body (literal, or @file)")

	networkFulfillCmd.Flags().IntVar(&networkStatus, "status", 200, "response status code")
	networkFulfillCmd.Flags().StringArrayVar(&networkHeaders, "header", nil, "response header, key=value (repeatable)")
	networkFulfillCmd.Flags().StringVar(&networkBody, "body", "", "response body (literal, or @file)")

	networkAbortCmd.Flags().StringVar(&networkError, "error", "", "failure code hint (e.g. failed, timedout)")
}
