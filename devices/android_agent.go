package devices

import (
	"bufio"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mobile-next/mobilecli/devices/wda"
	"github.com/mobile-next/mobilecli/types"
	"github.com/mobile-next/mobilecli/utils"
)

// mwAgentDex is the compiled on-device automation server (app_process server).
// It holds a UiAutomation connection open for the process lifetime and serves
// hierarchy dumps, screenshots, and input injection over a localabstract
// socket — eliminating the per-command process/instrumentation spawn that makes
// the adb paths slow.
//
//go:embed assets/mw-agent.dex
var mwAgentDex []byte

const (
	agentSocketName = "mobilewright-agent"
	agentMainClass  = "dev.mobilewright.agent.Agent"
)

// agentRemotePath is the on-device dex path, content-addressed so a new agent
// build is pushed automatically without a manual version bump.
var agentRemotePath = fmt.Sprintf("/data/local/tmp/mw-agent-%s.dex", agentDexHash())

func agentDexHash() string {
	sum := sha256.Sum256(mwAgentDex)
	return fmt.Sprintf("%x", sum[:4])
}

// androidAgent is a persistent client to the on-device agent for one device.
type androidAgent struct {
	device *AndroidDevice
	mu     sync.Mutex
	conn   net.Conn
	reader *bufio.Reader
	nextID int
	port   int
}

// agentEnabled reports whether the fast on-device agent should be used.
// Opt out with MOBILECLI_DISABLE_AGENT=1.
func agentEnabled() bool {
	return os.Getenv("MOBILECLI_DISABLE_AGENT") == ""
}

// getAgent returns a connected agent for the device, performing one-time setup
// (push dex, launch app_process, adb forward, dial) on first use. Any failure
// is sticky for the session so callers fall back to adb without re-probing.
func (d *AndroidDevice) getAgent() (*androidAgent, error) {
	if !agentEnabled() {
		return nil, fmt.Errorf("agent disabled")
	}

	d.agentOnce.Do(func() {
		a := &androidAgent{device: d}
		if err := a.setup(); err != nil {
			utils.Verbose("agent setup failed, falling back to adb: %v", err)
			d.agentErr = err
			return
		}
		d.agent = a
	})

	if d.agent == nil {
		return nil, d.agentErr
	}
	return d.agent, nil
}

func (a *androidAgent) setup() error {
	if err := a.ensureDexPushed(); err != nil {
		return err
	}
	if err := a.ensureRunning(); err != nil {
		return err
	}
	if err := a.ensureForwarded(); err != nil {
		return err
	}
	return a.connect()
}

func (a *androidAgent) ensureDexPushed() error {
	// `ls` is cheap; only push when the content-addressed dex is absent.
	// Note: a missing file makes `ls` print an error that *contains* the path,
	// so presence must be detected by exit status, not substring match.
	if out, err := a.device.runAdbCommand("shell", "ls", agentRemotePath); err == nil &&
		!strings.Contains(string(out), "No such file") {
		return nil
	}

	tmp, err := os.CreateTemp("", "mw-agent-*.dex")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(mwAgentDex); err != nil {
		return err
	}
	_ = tmp.Close()

	if out, err := a.device.runAdbCommand("push", tmp.Name(), agentRemotePath); err != nil {
		return fmt.Errorf("failed to push agent dex: %w\n%s", err, string(out))
	}
	utils.Verbose("pushed agent dex to %s", agentRemotePath)
	return nil
}

func (a *androidAgent) ensureRunning() error {
	// already serving?
	if a.socketPresent() {
		return nil
	}

	launch := fmt.Sprintf(
		"CLASSPATH=%s nohup app_process /system/bin %s > /data/local/tmp/mw-agent.log 2>&1 &",
		agentRemotePath, agentMainClass,
	)
	if out, err := a.device.runAdbCommand("shell", launch); err != nil {
		return fmt.Errorf("failed to launch agent: %w\n%s", err, string(out))
	}

	// wait for the localabstract socket to appear
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if a.socketPresent() {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	log, _ := a.device.runAdbCommand("shell", "cat", "/data/local/tmp/mw-agent.log")
	return fmt.Errorf("agent did not start; log:\n%s", string(log))
}

func (a *androidAgent) socketPresent() bool {
	out, err := a.device.runAdbCommand("shell", "cat", "/proc/net/unix")
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "@"+agentSocketName)
}

func (a *androidAgent) ensureForwarded() error {
	port, err := freeLocalPort()
	if err != nil {
		return err
	}
	if out, err := a.device.runAdbCommand("forward",
		fmt.Sprintf("tcp:%d", port),
		"localabstract:"+agentSocketName); err != nil {
		return fmt.Errorf("adb forward failed: %w\n%s", err, string(out))
	}
	a.port = port
	return nil
}

func (a *androidAgent) connect() error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", a.port), 5*time.Second)
	if err != nil {
		return err
	}
	a.conn = conn
	a.reader = bufio.NewReader(conn)

	// verify with a ping. connect() runs either during one-time setup (single
	// goroutine, via sync.Once) or from reconnectLocked() with a.mu already
	// held, so the ping is written directly rather than through rawCall — which
	// would deadlock on a.mu during reconnect.
	if err := a.pingDirect(); err != nil {
		a.dropConn()
		return fmt.Errorf("agent ping failed: %w", err)
	}
	utils.Verbose("agent connected on tcp:%d", a.port)
	return nil
}

func (a *androidAgent) pingDirect() error {
	_ = a.conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := a.conn.Write([]byte("0 ping\n")); err != nil {
		return err
	}
	if _, err := a.reader.ReadBytes('\n'); err != nil {
		return err
	}
	return nil
}

type agentResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error"`
}

// rawCall sends one command and returns its result payload. It serializes
// access so concurrent RPC handlers share the single agent connection safely.
// On a transport failure (agent killed, device rebooted) it transparently
// re-establishes the agent once and retries, so a single hiccup does not
// poison the rest of the session.
func (a *androidAgent) rawCall(method string, args ...string) (json.RawMessage, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	res, err := a.send(method, args)
	if err == nil {
		return res, nil
	}
	if !isTransportError(err) {
		return nil, err
	}

	utils.Verbose("agent transport error (%v), reconnecting", err)
	if rerr := a.reconnectLocked(); rerr != nil {
		return nil, fmt.Errorf("agent reconnect failed: %w (original: %v)", rerr, err)
	}
	return a.send(method, args)
}

// send performs one request/response cycle. The caller must hold a.mu.
func (a *androidAgent) send(method string, args []string) (json.RawMessage, error) {
	if a.conn == nil {
		return nil, errAgentDisconnected
	}

	a.nextID++
	id := a.nextID
	line := fmt.Sprintf("%d %s", id, method)
	if len(args) > 0 {
		line += " " + strings.Join(args, " ")
	}
	line += "\n"

	_ = a.conn.SetDeadline(time.Now().Add(30 * time.Second))
	if _, err := a.conn.Write([]byte(line)); err != nil {
		a.dropConn()
		return nil, err
	}

	respLine, err := a.reader.ReadBytes('\n')
	if err != nil {
		a.dropConn()
		return nil, err
	}

	var resp agentResponse
	if err := json.Unmarshal(respLine, &resp); err != nil {
		return nil, fmt.Errorf("bad agent response: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("agent error: %s", resp.Error)
	}
	return resp.Result, nil
}

func (a *androidAgent) dropConn() {
	if a.conn != nil {
		_ = a.conn.Close()
		a.conn = nil
	}
}

// reconnectLocked re-runs the launch/forward/connect steps. The caller must
// hold a.mu.
func (a *androidAgent) reconnectLocked() error {
	a.dropConn()
	if err := a.ensureRunning(); err != nil {
		return err
	}
	if err := a.ensureForwarded(); err != nil {
		return err
	}
	return a.connect()
}

var errAgentDisconnected = fmt.Errorf("agent not connected")

// isTransportError reports whether an error reflects a broken connection
// (as opposed to a method-level error reported by a healthy agent).
func isTransportError(err error) bool {
	if err == nil {
		return false
	}
	if err == errAgentDisconnected {
		return true
	}
	if strings.HasPrefix(err.Error(), "agent error:") || strings.HasPrefix(err.Error(), "bad agent response") {
		return false
	}
	return true
}

// ─── Agent-backed device operations ────────────────────────────────

func (a *androidAgent) tap(x, y int) error {
	_, err := a.rawCall("tap", itoa(x), itoa(y))
	return err
}

func (a *androidAgent) swipe(x1, y1, x2, y2, durationMs int) error {
	_, err := a.rawCall("swipe", itoa(x1), itoa(y1), itoa(x2), itoa(y2), itoa(durationMs))
	return err
}

func (a *androidAgent) longPress(x, y, durationMs int) error {
	_, err := a.rawCall("longpress", itoa(x), itoa(y), itoa(durationMs))
	return err
}

// key presses a single key by Android keycode name (e.g. "KEYCODE_HOME").
func (a *androidAgent) key(keycodeName string) error {
	_, err := a.rawCall("key", keycodeName)
	return err
}

// typeText injects text via the on-device KeyCharacterMap. The payload is
// base64-encoded so spaces and unicode survive the whitespace-delimited
// protocol. Returns an error (so callers fall back to adb/clipboard) when the
// device cannot map the characters — e.g. non-ASCII / emoji.
func (a *androidAgent) typeText(text string) error {
	enc := base64.StdEncoding.EncodeToString([]byte(text))
	_, err := a.rawCall("text", enc)
	return err
}

// gesture runs a pointer action sequence. Actions are JSON-encoded then
// base64-wrapped to pass through the protocol as a single token.
func (a *androidAgent) gesture(actions []wda.TapAction) error {
	payload, err := json.Marshal(actions)
	if err != nil {
		return err
	}
	enc := base64.StdEncoding.EncodeToString(payload)
	_, err = a.rawCall("gesture", enc)
	return err
}

func (a *androidAgent) screenshot() ([]byte, error) {
	// Return PNG to honor the TakeScreenshot() contract; the command layer
	// converts to JPEG host-side when requested.
	res, err := a.rawCall("screenshot", "png", "100")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(res, &payload); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(payload.Data)
}

// agentNode mirrors the JSON node shape produced by the on-device agent.
type agentNode struct {
	Type       string        `json:"type"`
	Text       string        `json:"text"`
	Label      string        `json:"label"`
	Identifier string        `json:"identifier"`
	Enabled    bool          `json:"enabled"`
	Visible    bool          `json:"visible"`
	Rect       deviceKitRect `json:"rect"`
	Children   []agentNode   `json:"children"`
}

func (a *androidAgent) dump() ([]agentNode, error) {
	res, err := a.rawCall("dump")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Elements []agentNode `json:"elements"`
	}
	if err := json.Unmarshal(res, &payload); err != nil {
		return nil, err
	}
	return payload.Elements, nil
}

func (a *androidAgent) dumpRaw() (string, error) {
	res, err := a.rawCall("dump")
	if err != nil {
		return "", err
	}
	return string(res), nil
}

// collectAgentElements flattens the agent node tree into ScreenElements,
// matching the filtering used for devicekit/uiautomator dumps.
func collectAgentElements(nodes []agentNode) []types.ScreenElement {
	var elements []types.ScreenElement
	for _, node := range nodes {
		elements = append(elements, collectAgentElements(node.Children)...)

		if node.Text == "" && node.Label == "" && node.Identifier == "" {
			continue
		}
		if node.Rect.Width <= 0 || node.Rect.Height <= 0 {
			continue
		}

		text := node.Text
		element := types.ScreenElement{
			Type: node.Type,
			Text: &text,
			Rect: types.ScreenElementRect{
				X:      node.Rect.X,
				Y:      node.Rect.Y,
				Width:  node.Rect.Width,
				Height: node.Rect.Height,
			},
		}
		if node.Label != "" {
			label := node.Label
			element.Label = &label
		}
		if node.Identifier != "" {
			id := node.Identifier
			element.Identifier = &id
		}
		if element.Type == "" {
			element.Type = "text"
		}
		elements = append(elements, element)
	}
	return elements
}

// ─── helpers ───────────────────────────────────────────────────────

func itoa(n int) string { return fmt.Sprintf("%d", n) }
