package devices

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mobile-next/mobilecli/utils"
)

// A browser cannot take the injected agent: it is not a debuggable build. It
// does serve the Chrome DevTools Protocol on an abstract unix socket, so its
// tabs are driven through that instead, behind the same webview commands.

var errWebViewNotFound = errors.New("webview not found")

const (
	cdpCommandTimeout = 30 * time.Second
	// the discovery endpoints are plain local http, and a socket that is not a
	// browser after all must not hold up the search for the one that is
	cdpDiscoveryTimeout = 5 * time.Second
	// a tab that does not answer is reported as hidden instead of holding up the list
	cdpVisibilityTimeout  = 2 * time.Second
	cdpLoadStatePollEvery = 200 * time.Millisecond
)

// devtoolsSocketPattern matches the abstract sockets Chromium names after its
// DevTools server, e.g. chrome_devtools_remote or webview_devtools_remote_<pid>.
var devtoolsSocketPattern = regexp.MustCompile(`@(\S*_devtools_remote(?:_\d+)?)$`)

// devtoolsSocketNames picks the DevTools sockets out of /proc/net/unix.
func devtoolsSocketNames(procNetUnix string) []string {
	names := []string{}
	for _, line := range strings.Split(procNetUnix, "\n") {
		if match := devtoolsSocketPattern.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
			names = append(names, match[1])
		}
	}
	return names
}

// cdpTarget is one entry of the browser's /json/list.
type cdpTarget struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	URL   string `json:"url"`
	Title string `json:"title"`
}

// cdpBrowser is a DevTools server reachable at addr (host:port), and drives
// its tabs as webviews.
type cdpBrowser struct {
	addr string
}

func (b cdpBrowser) getJSON(path string, into any) error {
	client := &http.Client{Timeout: cdpDiscoveryTimeout}
	resp, err := client.Get("http://" + b.addr + path)
	if err != nil {
		return fmt.Errorf("devtools %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("devtools %s: unexpected status %s", path, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		return fmt.Errorf("parse devtools %s: %w", path, err)
	}
	return nil
}

// packageName returns the Android package the DevTools server belongs to.
func (b cdpBrowser) packageName() (string, error) {
	var version struct {
		AndroidPackage string `json:"Android-Package"`
	}
	if err := b.getJSON("/json/version", &version); err != nil {
		return "", err
	}
	return version.AndroidPackage, nil
}

// call sends one protocol command to a tab and returns its result.
func (b cdpBrowser) call(webviewID, method string, params any, timeout time.Duration) (json.RawMessage, error) {
	dialer := websocket.Dialer{HandshakeTimeout: timeout}
	conn, resp, err := dialer.Dial("ws://"+b.addr+"/devtools/page/"+webviewID, nil)
	if err != nil {
		// the browser answers the handshake with an http error for a tab it does not have
		if resp != nil {
			return nil, fmt.Errorf("%w: %s", errWebViewNotFound, webviewID)
		}
		return nil, fmt.Errorf("connect to webview %s: %w", webviewID, err)
	}
	defer func() { _ = conn.Close() }()

	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	_ = conn.SetWriteDeadline(time.Now().Add(timeout))

	const commandID = 1
	command := map[string]any{"id": commandID, "method": method}
	if params != nil {
		command["params"] = params
	}
	if err := conn.WriteJSON(command); err != nil {
		return nil, fmt.Errorf("send %s: %w", method, err)
	}

	// events arrive on the same connection; skip everything that is not our reply
	for {
		var message struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := conn.ReadJSON(&message); err != nil {
			if netErr := net.Error(nil); errors.As(err, &netErr) && netErr.Timeout() {
				return nil, fmt.Errorf("webview %s did not answer %s within %s; a browser freezes the tabs it is not showing", webviewID, method, timeout.Round(time.Millisecond))
			}
			return nil, fmt.Errorf("read %s reply: %w", method, err)
		}
		if message.ID != commandID {
			continue
		}
		if message.Error != nil {
			return nil, fmt.Errorf("%s: %s", method, message.Error.Message)
		}
		return message.Result, nil
	}
}

func (b cdpBrowser) ListWebViews() ([]WebViewInfo, error) {
	packageName, err := b.packageName()
	if err != nil {
		return nil, err
	}
	var targets []cdpTarget
	if err := b.getJSON("/json/list", &targets); err != nil {
		return nil, err
	}

	webviews := []WebViewInfo{}
	for _, target := range targets {
		if target.Type != "page" {
			continue
		}
		webviews = append(webviews, WebViewInfo{
			ID:          target.ID,
			URL:         target.URL,
			Title:       target.Title,
			BundleID:    packageName,
			ProcessName: packageName,
		})
	}

	// the tab list does not say which tab is in front, so ask each one
	var wg sync.WaitGroup
	for i := range webviews {
		wg.Add(1)
		go func(webview *WebViewInfo) {
			defer wg.Done()
			webview.IsVisible = b.isVisible(webview.ID)
		}(&webviews[i])
	}
	wg.Wait()

	return webviews, nil
}

func (b cdpBrowser) isVisible(webviewID string) bool {
	visible, err := b.evaluate(webviewID, "return document.visibilityState === 'visible'", nil, cdpVisibilityTimeout)
	return err == nil && visible == true
}

func (b cdpBrowser) WebViewGoto(webviewID, url string) error {
	raw, err := b.call(webviewID, "Page.navigate", map[string]any{"url": url}, cdpCommandTimeout)
	if err != nil {
		return err
	}
	var navigation struct {
		ErrorText string `json:"errorText"`
	}
	if err := json.Unmarshal(raw, &navigation); err != nil {
		return fmt.Errorf("parse Page.navigate result: %w", err)
	}
	if navigation.ErrorText != "" {
		return fmt.Errorf("navigate to %s: %s", url, navigation.ErrorText)
	}
	return nil
}

func (b cdpBrowser) WebViewReload(webviewID string) error {
	_, err := b.call(webviewID, "Page.reload", nil, cdpCommandTimeout)
	return err
}

func (b cdpBrowser) WebViewGoBack(webviewID string) error {
	return b.navigateHistory(webviewID, -1)
}

func (b cdpBrowser) WebViewGoForward(webviewID string) error {
	return b.navigateHistory(webviewID, 1)
}

// navigateHistory moves offset entries through the tab's history, and like an
// embedded WebView does nothing when there is no such entry.
func (b cdpBrowser) navigateHistory(webviewID string, offset int) error {
	raw, err := b.call(webviewID, "Page.getNavigationHistory", nil, cdpCommandTimeout)
	if err != nil {
		return err
	}
	var history struct {
		CurrentIndex int `json:"currentIndex"`
		Entries      []struct {
			ID int `json:"id"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(raw, &history); err != nil {
		return fmt.Errorf("parse navigation history: %w", err)
	}

	index := history.CurrentIndex + offset
	if index < 0 || index >= len(history.Entries) {
		return nil
	}
	_, err = b.call(webviewID, "Page.navigateToHistoryEntry", map[string]any{"entryId": history.Entries[index].ID}, cdpCommandTimeout)
	return err
}

func (b cdpBrowser) WebViewContent(webviewID string) (string, error) {
	result, err := b.WebViewEvaluate(webviewID, "return document.documentElement.outerHTML", nil)
	if err != nil {
		return "", err
	}
	content, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("unexpected content type %T", result)
	}
	return content, nil
}

func (b cdpBrowser) WebViewEvaluate(webviewID, expression string, args []any) (any, error) {
	return b.evaluate(webviewID, expression, args, cdpCommandTimeout)
}

// evaluate runs expression the way the injected agent does: as the body of a
// function called with args, whose returned promise is awaited.
func (b cdpBrowser) evaluate(webviewID, expression string, args []any, timeout time.Duration) (any, error) {
	if args == nil {
		args = []any{}
	}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("marshal evaluate args: %w", err)
	}

	raw, err := b.call(webviewID, "Runtime.evaluate", map[string]any{
		"expression":    "(function(){\n" + ensureReturnExpression(expression) + "\n}).apply(null, " + string(argsJSON) + ")",
		"awaitPromise":  true,
		"returnByValue": true,
	}, timeout)
	if err != nil {
		return nil, err
	}

	var evaluation struct {
		Result struct {
			Value any `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text      string `json:"text"`
			Exception struct {
				Description string `json:"description"`
				Value       any    `json:"value"`
			} `json:"exception"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(raw, &evaluation); err != nil {
		return nil, fmt.Errorf("parse evaluate result: %w", err)
	}

	if details := evaluation.ExceptionDetails; details != nil {
		// the description of an Error carries its stack; the first line is the message
		if description, _, _ := strings.Cut(details.Exception.Description, "\n"); description != "" {
			return nil, errors.New(description)
		}
		if details.Exception.Value != nil {
			return nil, fmt.Errorf("%v", details.Exception.Value)
		}
		return nil, errors.New(details.Text)
	}
	return evaluation.Result.Value, nil
}

func (b cdpBrowser) WebViewWaitForLoadState(webviewID, state string, timeoutMs int) error {
	if state == "" {
		state = "load"
	}
	if timeoutMs <= 0 {
		timeoutMs = webViewAgentDefaultTimeoutMs
	}
	hasLoaded := "return document.readyState === 'complete'"
	if state == "domcontentloaded" {
		hasLoaded = "return document.readyState === 'interactive' || document.readyState === 'complete'"
	}

	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	for {
		// a poll on a tab that never answers must not outlive the wait
		pollTimeout := min(time.Until(deadline), cdpCommandTimeout)
		if pollTimeout <= 0 {
			return fmt.Errorf("waitForLoadState timed out waiting for '%s'", state)
		}
		// an evaluation fails while the tab swaps documents mid-navigation, so
		// only an unknown tab ends the wait early
		loaded, err := b.evaluate(webviewID, hasLoaded, nil, pollTimeout)
		if errors.Is(err, errWebViewNotFound) {
			return err
		}
		if err == nil && loaded == true {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("waitForLoadState timed out waiting for '%s': %w", state, err)
			}
			return fmt.Errorf("waitForLoadState timed out waiting for '%s'", state)
		}
		time.Sleep(cdpLoadStatePollEvery)
	}
}

// findDevtoolsBrowser returns the DevTools server that belongs to pkg. The
// socket names do not say which app serves them, so each is connected and
// asked; connections to other apps are released again.
func findDevtoolsBrowser(pkg string, sockets []string, connect func(socket string) (cdpBrowser, func(), error)) (cdpBrowser, bool) {
	for _, socket := range sockets {
		browser, release, err := connect(socket)
		if err != nil {
			utils.Verbose("failed to connect to devtools socket %s: %v", socket, err)
			continue
		}
		if owner, err := browser.packageName(); err == nil && owner == pkg {
			return browser, true
		}
		release()
	}
	return cdpBrowser{}, false
}
