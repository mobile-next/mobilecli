package commands

import (
	"context"
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
)

// ─── Request types ────────────────────────────────────────────

// NetworkListenRequest are the params for cli.network.listen (streaming).
type NetworkListenRequest struct {
	DeviceID string                 `json:"deviceId"`
	Routes   []devices.NetworkRoute `json:"routes,omitempty"`
}

// NetworkContinueRequest answers one paused request, letting it proceed
// (optionally with overrides). Body is base64.
type NetworkContinueRequest struct {
	DeviceID  string            `json:"deviceId"`
	RequestID string            `json:"requestId"`
	URL       string            `json:"url,omitempty"`
	Method    string            `json:"method,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      string            `json:"body,omitempty"`
}

// NetworkFulfillRequest answers one paused request with a synthetic response.
// Body is base64.
type NetworkFulfillRequest struct {
	DeviceID  string            `json:"deviceId"`
	RequestID string            `json:"requestId"`
	Status    int               `json:"status"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      string            `json:"body,omitempty"`
}

// NetworkAbortRequest fails one paused request. ErrorCode is an optional hint
// (e.g. "failed", "timedout", "connectionrefused").
type NetworkAbortRequest struct {
	DeviceID  string `json:"deviceId"`
	RequestID string `json:"requestId"`
	ErrorCode string `json:"errorCode,omitempty"`
}

// ─── Shared helper ────────────────────────────────────────────

func networkInterceptor(deviceID string) (devices.NetworkInterceptable, error) {
	device, err := FindDeviceOrAutoSelect(deviceID)
	if err != nil {
		return nil, fmt.Errorf("error finding device: %w", err)
	}
	provider, ok := device.(devices.NetworkInterceptorProvider)
	if !ok {
		return nil, fmt.Errorf("network interception is not supported on %s (%s)", device.ID(), device.Platform())
	}
	return provider.NetworkInterceptor()
}

// ─── Commands ─────────────────────────────────────────────────

// NetworkListenCommand enables interception, then relays each agent event to the
// client until the context is cancelled (the client disconnected / pressed
// Ctrl-C). On exit it disables interception, so any request still paused in the
// app continues unchanged — the fail-open guarantee.
func NetworkListenCommand(ctx context.Context, req NetworkListenRequest, notify func(any) error) *CommandResponse {
	interceptor, err := networkInterceptor(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	if err := interceptor.NetworkEnable(req.Routes); err != nil {
		return NewErrorResponse(fmt.Errorf("enable network interception: %w", err))
	}
	defer func() { _ = interceptor.NetworkDisable() }()

	for {
		if ctx.Err() != nil {
			return NewSuccessResponse(OK)
		}
		events, err := interceptor.NetworkPoll(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return NewSuccessResponse(OK)
			}
			return NewErrorResponse(fmt.Errorf("poll network events: %w", err))
		}
		for _, event := range events {
			if err := notify(event); err != nil {
				// client went away: stop and let the deferred disable release the app
				return NewSuccessResponse(OK)
			}
		}
	}
}

// NetworkContinueCommand lets a paused request proceed, with optional overrides.
func NetworkContinueCommand(req NetworkContinueRequest) *CommandResponse {
	if req.RequestID == "" {
		return NewErrorResponse(fmt.Errorf("requestId is required"))
	}
	interceptor, err := networkInterceptor(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	params := map[string]any{"requestId": req.RequestID}
	if req.URL != "" {
		params["url"] = req.URL
	}
	if req.Method != "" {
		params["method"] = req.Method
	}
	if len(req.Headers) > 0 {
		params["headers"] = req.Headers
	}
	if req.Body != "" {
		params["body"] = req.Body
	}
	if err := interceptor.NetworkContinue(params); err != nil {
		return NewErrorResponse(err)
	}
	return NewSuccessResponse(OK)
}

// NetworkFulfillCommand answers a paused request with a synthetic response.
func NetworkFulfillCommand(req NetworkFulfillRequest) *CommandResponse {
	if req.RequestID == "" {
		return NewErrorResponse(fmt.Errorf("requestId is required"))
	}
	interceptor, err := networkInterceptor(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	params := map[string]any{"requestId": req.RequestID, "status": req.Status}
	if len(req.Headers) > 0 {
		params["headers"] = req.Headers
	}
	if req.Body != "" {
		params["body"] = req.Body
	}
	if err := interceptor.NetworkFulfill(params); err != nil {
		return NewErrorResponse(err)
	}
	return NewSuccessResponse(OK)
}

// NetworkAbortCommand fails a paused request.
func NetworkAbortCommand(req NetworkAbortRequest) *CommandResponse {
	if req.RequestID == "" {
		return NewErrorResponse(fmt.Errorf("requestId is required"))
	}
	interceptor, err := networkInterceptor(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	params := map[string]any{"requestId": req.RequestID}
	if req.ErrorCode != "" {
		params["errorCode"] = req.ErrorCode
	}
	if err := interceptor.NetworkAbort(params); err != nil {
		return NewErrorResponse(err)
	}
	return NewSuccessResponse(OK)
}
