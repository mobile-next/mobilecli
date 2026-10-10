package devices

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// networkPollTimeout bounds one network.poll call. It must exceed the agent's
// own poll hold (~30s) so a quiet window returns empty rather than erroring.
const networkPollTimeout = 35 * time.Second

// agentNetwork relays the NetworkInterceptable surface to the in-app agent over
// its forwarded RPC port, the same channel agentWebViews uses. It holds no
// interception logic itself — the agent does the work; this just forwards JSON.
type agentNetwork struct {
	port int
}

func (a agentNetwork) NetworkEnable(routes []NetworkRoute) error {
	if routes == nil {
		routes = []NetworkRoute{}
	}
	_, err := agentRequest(a.port, "network.enable", map[string]any{"routes": routes})
	return err
}

func (a agentNetwork) NetworkPoll(ctx context.Context) ([]json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result, err := agentRequestContext(ctx, a.port, "network.poll", map[string]any{}, networkPollTimeout)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Events []json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return nil, fmt.Errorf("parse network events: %w", err)
	}
	return payload.Events, nil
}

func (a agentNetwork) NetworkContinue(params map[string]any) error {
	_, err := agentRequest(a.port, "network.continue", params)
	return err
}

func (a agentNetwork) NetworkFulfill(params map[string]any) error {
	_, err := agentRequest(a.port, "network.fulfill", params)
	return err
}

func (a agentNetwork) NetworkAbort(params map[string]any) error {
	_, err := agentRequest(a.port, "network.abort", params)
	return err
}

func (a agentNetwork) NetworkDisable() error {
	_, err := agentRequest(a.port, "network.disable", map[string]any{})
	return err
}

// NetworkInterceptor attaches to the debuggable foreground app on Android and
// returns a relay to its in-app agent. It mirrors webViews()' foreground
// resolution but only takes the agent path (a browser cannot be intercepted this
// way).
func (d *AndroidDevice) NetworkInterceptor() (NetworkInterceptable, error) {
	var foreground *ForegroundAppInfo
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for {
		foreground, err = d.GetForegroundApp()
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("could not determine foreground app: %w", err)
		}
		time.Sleep(150 * time.Millisecond)
	}
	port, err := d.ensureAgentReady(foreground.PackageName)
	if err != nil {
		return nil, err
	}
	return agentNetwork{port: port}, nil
}

// NetworkInterceptor attaches to the debuggable foreground app in the simulator
// and returns a relay to its in-app agent.
func (s *SimulatorDevice) NetworkInterceptor() (NetworkInterceptable, error) {
	port, err := s.ensureIOSAgentReady()
	if err != nil {
		return nil, err
	}
	return agentNetwork{port: port}, nil
}
