package devices

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// webViewAgentDefaultTimeoutMs mirrors the agent's own default wait, and is used
// when the caller does not ask for a specific timeout.
const webViewAgentDefaultTimeoutMs = 30_000

// ensureReturnExpression turns a value expression into a statement body that
// returns that value, so the agent's eval wrapper can capture it. A bare
// expression — even an IIFE that internally uses ';', '{' or newlines — must be
// wrapped; only skip wrapping when the caller already supplied a top-level
// "return". A trailing ';' is stripped so the wrapped form stays valid.
func ensureReturnExpression(expression string) string {
	trimmed := strings.TrimSpace(expression)
	if strings.HasPrefix(trimmed, "return ") || strings.HasPrefix(trimmed, "return(") {
		return expression
	}
	trimmed = strings.TrimRight(trimmed, " \t\r\n;")
	return "return (" + trimmed + ")"
}

// webViewEvaluate runs expression in the webview and returns the JS result value.
// Both the Android and iOS agents speak the same webview protocol, so the only
// per-platform difference is which port the agent is listening on.
func webViewEvaluate(port int, webviewID, expression string, args []any) (any, error) {
	params := map[string]any{
		"id":         webviewID,
		"expression": ensureReturnExpression(expression),
	}
	if len(args) > 0 {
		params["args"] = args
	}

	raw, err := agentRequest(port, "device.webview.evaluate", params)
	if err != nil {
		return nil, err
	}

	// agent returns {"result": <value>} — unwrap one level
	var wrapper struct {
		Result any `json:"result"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, fmt.Errorf("parse evaluate result: %w", err)
	}

	return wrapper.Result, nil
}

// webViewWaitForLoadState blocks until the webview reaches the given load state.
// timeoutMs of 0 uses the agent's default.
func webViewWaitForLoadState(port int, webviewID, state string, timeoutMs int) error {
	waitMs := webViewAgentDefaultTimeoutMs
	if timeoutMs > 0 {
		waitMs = timeoutMs
	}

	params := map[string]any{"id": webviewID, "timeout": waitMs}
	if state != "" {
		params["state"] = state
	}

	// give the http call a margin over the agent-side wait, so the agent is the
	// one that times out and reports why
	httpTimeout := time.Duration(waitMs)*time.Millisecond + 5*time.Second
	_, err := agentRequestWithTimeout(port, "device.webview.waitForLoadState", params, httpTimeout)
	return err
}
