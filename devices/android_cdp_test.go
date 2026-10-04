package devices

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type fakeChromeCall struct {
	TargetID string
	Method   string
	Params   map[string]any
}

// fakeChrome serves the DevTools endpoints of a browser on a device: the http
// discovery ones, and one websocket per tab that answers protocol commands.
type fakeChrome struct {
	t           *testing.T
	server      *httptest.Server
	packageName string
	targets     []cdpTarget
	// answer returns the result of a protocol command, or a protocol error message
	answer func(call fakeChromeCall) (result any, protocolError string)

	mu    sync.Mutex
	calls []fakeChromeCall
}

func startFakeChrome(t *testing.T, targets ...cdpTarget) *fakeChrome {
	t.Helper()
	chrome := &fakeChrome{t: t, packageName: "com.android.chrome", targets: targets}
	chrome.answer = func(fakeChromeCall) (any, string) { return map[string]any{}, "" }

	mux := http.NewServeMux()
	mux.HandleFunc("/json/version", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"Android-Package": chrome.packageName, "Browser": "Chrome/154.0"})
	})
	mux.HandleFunc("/json/list", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(chrome.targets)
	})
	mux.HandleFunc("/devtools/page/", chrome.serveTab)

	chrome.server = httptest.NewServer(mux)
	t.Cleanup(chrome.server.Close)
	return chrome
}

func (c *fakeChrome) serveTab(w http.ResponseWriter, r *http.Request) {
	targetID := strings.TrimPrefix(r.URL.Path, "/devtools/page/")
	if !c.hasTarget(targetID) {
		http.Error(w, "No such target id: "+targetID, http.StatusNotFound)
		return
	}

	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	for {
		var command struct {
			ID     int            `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if err := conn.ReadJSON(&command); err != nil {
			return
		}

		call := fakeChromeCall{TargetID: targetID, Method: command.Method, Params: command.Params}
		c.mu.Lock()
		c.calls = append(c.calls, call)
		c.mu.Unlock()

		// a real browser interleaves events with replies, so send one first
		_ = conn.WriteJSON(map[string]any{"method": "Page.frameNavigated", "params": map[string]any{}})

		result, protocolError := c.answer(call)
		reply := map[string]any{"id": command.ID, "result": result}
		if protocolError != "" {
			reply = map[string]any{"id": command.ID, "error": map[string]any{"code": -32000, "message": protocolError}}
		}
		_ = conn.WriteJSON(reply)
	}
}

func (c *fakeChrome) hasTarget(id string) bool {
	for _, target := range c.targets {
		if target.ID == id {
			return true
		}
	}
	return false
}

func (c *fakeChrome) browser() cdpBrowser {
	return cdpBrowser{addr: strings.TrimPrefix(c.server.URL, "http://")}
}

func (c *fakeChrome) callsOf(method string) []fakeChromeCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	var matching []fakeChromeCall
	for _, call := range c.calls {
		if call.Method == method {
			matching = append(matching, call)
		}
	}
	return matching
}

func (c *fakeChrome) onlyCallOf(method string) fakeChromeCall {
	c.t.Helper()
	calls := c.callsOf(method)
	if len(calls) != 1 {
		c.t.Fatalf("expected exactly one %s call, got %d", method, len(calls))
	}
	return calls[0]
}

// evaluatesTo makes every Runtime.evaluate return the given javascript value
func (c *fakeChrome) evaluatesTo(value any) {
	c.answer = func(fakeChromeCall) (any, string) {
		return map[string]any{"result": map[string]any{"type": "object", "value": value}}, ""
	}
}

func (c *fakeChrome) hasHistory(currentIndex int, entryIDs ...int) {
	entries := []map[string]any{}
	for _, id := range entryIDs {
		entries = append(entries, map[string]any{"id": id})
	}
	c.answer = func(call fakeChromeCall) (any, string) {
		if call.Method == "Page.getNavigationHistory" {
			return map[string]any{"currentIndex": currentIndex, "entries": entries}, ""
		}
		return map[string]any{}, ""
	}
}

var examplePage = cdpTarget{ID: "TAB1", Type: "page", URL: "https://example.com/", Title: "Example Domain"}

func TestDevtoolsSocketsAreFoundAmongTheUnixSocketsOfTheDevice(t *testing.T) {
	procNetUnix := `Num       RefCount Protocol Flags    Type St Inode Path
0000000000000000: 00000002 00000000 00010000 0001 01 452744 @chrome_devtools_remote
0000000000000000: 00000002 00000000 00010000 0001 01 383406 @stetho_com.example_devtools_remote
0000000000000000: 00000002 00000000 00010000 0001 01 11111 @webview_devtools_remote_4242
0000000000000000: 00000003 00000000 00000000 0001 03 22222 /dev/socket/logdw
0000000000000000: 00000002 00000000 00010000 0001 01 33333 @jdwp-control
`

	sockets := devtoolsSocketNames(procNetUnix)

	expected := []string{"chrome_devtools_remote", "stetho_com.example_devtools_remote", "webview_devtools_remote_4242"}
	if !reflect.DeepEqual(sockets, expected) {
		t.Fatalf("expected %v, got %v", expected, sockets)
	}
}

func TestBrowserReportsTheAndroidPackageItBelongsTo(t *testing.T) {
	chrome := startFakeChrome(t)

	packageName, err := chrome.browser().packageName()

	if err != nil {
		t.Fatalf("packageName: %v", err)
	}
	if packageName != "com.android.chrome" {
		t.Fatalf("unexpected package %q", packageName)
	}
}

func TestListingWebViewsReturnsTheTabsOfTheBrowser(t *testing.T) {
	serviceWorker := cdpTarget{ID: "SW1", Type: "service_worker", URL: "https://example.com/sw.js"}
	chrome := startFakeChrome(t, examplePage, serviceWorker)
	chrome.evaluatesTo(true)

	webviews, err := chrome.browser().ListWebViews()

	if err != nil {
		t.Fatalf("ListWebViews: %v", err)
	}
	expected := []WebViewInfo{{
		ID:          "TAB1",
		URL:         "https://example.com/",
		Title:       "Example Domain",
		BundleID:    "com.android.chrome",
		ProcessName: "com.android.chrome",
		IsVisible:   true,
	}}
	if !reflect.DeepEqual(webviews, expected) {
		t.Fatalf("expected %+v, got %+v", expected, webviews)
	}
}

func TestBackgroundTabIsListedAsNotVisible(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)
	chrome.evaluatesTo(false)

	webviews, err := chrome.browser().ListWebViews()

	if err != nil {
		t.Fatalf("ListWebViews: %v", err)
	}
	if len(webviews) != 1 || webviews[0].IsVisible {
		t.Fatalf("expected one hidden tab, got %+v", webviews)
	}
}

func TestBrowserWithoutTabsListsNoWebViews(t *testing.T) {
	chrome := startFakeChrome(t)

	webviews, err := chrome.browser().ListWebViews()

	if err != nil {
		t.Fatalf("ListWebViews: %v", err)
	}
	if webviews == nil || len(webviews) != 0 {
		t.Fatalf("expected an empty list, got %#v", webviews)
	}
}

func TestEvaluateReturnsTheJavascriptValue(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)
	chrome.evaluatesTo(map[string]any{"title": "Example Domain"})

	value, err := chrome.browser().WebViewEvaluate("TAB1", "return {title: document.title}", nil)

	if err != nil {
		t.Fatalf("WebViewEvaluate: %v", err)
	}
	if !reflect.DeepEqual(value, map[string]any{"title": "Example Domain"}) {
		t.Fatalf("unexpected value %#v", value)
	}
}

// The agent runs the expression as a function body and awaits what it returns,
// so the same expression has to mean the same thing in a browser tab.
func TestEvaluateRunsTheExpressionAsAFunctionBodyAndAwaitsIt(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)

	_, err := chrome.browser().WebViewEvaluate("TAB1", "document.title", []any{"first", 2})

	if err != nil {
		t.Fatalf("WebViewEvaluate: %v", err)
	}
	params := chrome.onlyCallOf("Runtime.evaluate").Params
	expectedExpression := "(function(){\nreturn (document.title)\n}).apply(null, [\"first\",2])"
	if params["expression"] != expectedExpression {
		t.Fatalf("unexpected expression %q", params["expression"])
	}
	if params["awaitPromise"] != true || params["returnByValue"] != true {
		t.Fatalf("expected the promise to be awaited and returned by value, got %v", params)
	}
}

func TestEvaluateOfUndefinedReturnsNil(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)
	chrome.answer = func(fakeChromeCall) (any, string) {
		return map[string]any{"result": map[string]any{"type": "undefined"}}, ""
	}

	value, err := chrome.browser().WebViewEvaluate("TAB1", "return undefined", nil)

	if err != nil || value != nil {
		t.Fatalf("expected nil without error, got %#v, %v", value, err)
	}
}

func TestEvaluateFailsWithTheMessageOfTheThrownError(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)
	chrome.answer = func(fakeChromeCall) (any, string) {
		return map[string]any{
			"result": map[string]any{"type": "object", "subtype": "error"},
			"exceptionDetails": map[string]any{
				"text":      "Uncaught",
				"exception": map[string]any{"description": "Error: boom\n    at <anonymous>:2:7"},
			},
		}, ""
	}

	_, err := chrome.browser().WebViewEvaluate("TAB1", "throw new Error('boom')", nil)

	if err == nil || err.Error() != "Error: boom" {
		t.Fatalf("expected the thrown error, got %v", err)
	}
}

func TestCommandOnAnUnknownWebViewFailsAsNotFound(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)

	err := chrome.browser().WebViewReload("NO-SUCH-TAB")

	if !errors.Is(err, errWebViewNotFound) || !strings.Contains(err.Error(), "NO-SUCH-TAB") {
		t.Fatalf("expected a not found error naming the webview, got %v", err)
	}
}

func TestProtocolErrorFailsTheCommand(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)
	chrome.answer = func(fakeChromeCall) (any, string) { return nil, "Not allowed" }

	err := chrome.browser().WebViewReload("TAB1")

	if err == nil || !strings.Contains(err.Error(), "Not allowed") {
		t.Fatalf("expected the protocol error, got %v", err)
	}
}

func TestGotoNavigatesTheTabToTheUrl(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)

	err := chrome.browser().WebViewGoto("TAB1", "https://mobilewright.dev/")

	if err != nil {
		t.Fatalf("WebViewGoto: %v", err)
	}
	if url := chrome.onlyCallOf("Page.navigate").Params["url"]; url != "https://mobilewright.dev/" {
		t.Fatalf("navigated to %v", url)
	}
}

func TestGotoFailsWhenTheBrowserCannotStartTheNavigation(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)
	chrome.answer = func(fakeChromeCall) (any, string) {
		return map[string]any{"frameId": "F1", "errorText": "net::ERR_NAME_NOT_RESOLVED"}, ""
	}

	err := chrome.browser().WebViewGoto("TAB1", "https://no-such-host.invalid/")

	if err == nil || !strings.Contains(err.Error(), "net::ERR_NAME_NOT_RESOLVED") {
		t.Fatalf("expected the navigation error, got %v", err)
	}
}

func TestReloadReloadsTheTab(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)

	err := chrome.browser().WebViewReload("TAB1")

	if err != nil {
		t.Fatalf("WebViewReload: %v", err)
	}
	chrome.onlyCallOf("Page.reload")
}

func TestGoBackNavigatesToThePreviousHistoryEntry(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)
	chrome.hasHistory(1, 10, 20, 30)

	err := chrome.browser().WebViewGoBack("TAB1")

	if err != nil {
		t.Fatalf("WebViewGoBack: %v", err)
	}
	if entry := chrome.onlyCallOf("Page.navigateToHistoryEntry").Params["entryId"]; entry != float64(10) {
		t.Fatalf("navigated to history entry %v", entry)
	}
}

func TestGoForwardNavigatesToTheNextHistoryEntry(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)
	chrome.hasHistory(1, 10, 20, 30)

	err := chrome.browser().WebViewGoForward("TAB1")

	if err != nil {
		t.Fatalf("WebViewGoForward: %v", err)
	}
	if entry := chrome.onlyCallOf("Page.navigateToHistoryEntry").Params["entryId"]; entry != float64(30) {
		t.Fatalf("navigated to history entry %v", entry)
	}
}

// An embedded WebView ignores goBack when there is nowhere to go back to.
func TestGoBackOnTheFirstHistoryEntryDoesNothing(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)
	chrome.hasHistory(0, 10, 20)

	err := chrome.browser().WebViewGoBack("TAB1")

	if err != nil {
		t.Fatalf("WebViewGoBack: %v", err)
	}
	if navigations := chrome.callsOf("Page.navigateToHistoryEntry"); len(navigations) != 0 {
		t.Fatalf("expected no navigation, got %v", navigations)
	}
}

func TestGoForwardOnTheLastHistoryEntryDoesNothing(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)
	chrome.hasHistory(1, 10, 20)

	err := chrome.browser().WebViewGoForward("TAB1")

	if err != nil {
		t.Fatalf("WebViewGoForward: %v", err)
	}
	if navigations := chrome.callsOf("Page.navigateToHistoryEntry"); len(navigations) != 0 {
		t.Fatalf("expected no navigation, got %v", navigations)
	}
}

func TestContentReturnsTheHtmlOfTheDocument(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)
	chrome.evaluatesTo("<html><body>hi</body></html>")

	content, err := chrome.browser().WebViewContent("TAB1")

	if err != nil {
		t.Fatalf("WebViewContent: %v", err)
	}
	if content != "<html><body>hi</body></html>" {
		t.Fatalf("unexpected content %q", content)
	}
}

func TestWaitReturnsOnceTheDocumentHasLoaded(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)
	polls := 0
	chrome.answer = func(fakeChromeCall) (any, string) {
		polls++
		return map[string]any{"result": map[string]any{"type": "boolean", "value": polls >= 3}}, ""
	}

	err := chrome.browser().WebViewWaitForLoadState("TAB1", "load", 5000)

	if err != nil {
		t.Fatalf("WebViewWaitForLoadState: %v", err)
	}
	if polls != 3 {
		t.Fatalf("expected to stop polling once loaded, polled %d times", polls)
	}
}

func TestWaitForDomContentLoadedAlsoAcceptsAnInteractiveDocument(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)
	chrome.evaluatesTo(true)

	err := chrome.browser().WebViewWaitForLoadState("TAB1", "domcontentloaded", 5000)

	if err != nil {
		t.Fatalf("WebViewWaitForLoadState: %v", err)
	}
	expression := chrome.onlyCallOf("Runtime.evaluate").Params["expression"].(string)
	if !strings.Contains(expression, "'interactive'") {
		t.Fatalf("expected the interactive state to be accepted, evaluated %q", expression)
	}
}

func TestWaitTimesOutWhenTheDocumentNeverLoads(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)
	chrome.evaluatesTo(false)

	err := chrome.browser().WebViewWaitForLoadState("TAB1", "load", 300)

	if err == nil || err.Error() != "waitForLoadState timed out waiting for 'load'" {
		t.Fatalf("expected a timeout, got %v", err)
	}
}

func TestWaitOnAnUnknownWebViewFailsWithoutWaitingForTheTimeout(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)

	err := chrome.browser().WebViewWaitForLoadState("NO-SUCH-TAB", "load", 60_000)

	if !errors.Is(err, errWebViewNotFound) {
		t.Fatalf("expected a not found error, got %v", err)
	}
}

// Several apps on a device can serve DevTools at once, and the socket names do
// not say which app each belongs to, so the browser itself is asked.
func TestTheDevtoolsSocketOfTheForegroundAppIsChosen(t *testing.T) {
	messages := startFakeChrome(t)
	messages.packageName = "com.google.android.apps.messaging"
	chrome := startFakeChrome(t, examplePage)
	servers := map[string]*fakeChrome{"stetho_messaging_devtools_remote": messages, "chrome_devtools_remote": chrome}
	var released []string
	connect := func(socket string) (cdpBrowser, func(), error) {
		return servers[socket].browser(), func() { released = append(released, socket) }, nil
	}

	browser, found := findDevtoolsBrowser("com.android.chrome", []string{"stetho_messaging_devtools_remote", "chrome_devtools_remote"}, connect)

	if !found || browser != chrome.browser() {
		t.Fatalf("expected the chrome socket to be chosen, got %+v (found=%v)", browser, found)
	}
	if !reflect.DeepEqual(released, []string{"stetho_messaging_devtools_remote"}) {
		t.Fatalf("expected only the other app's socket to be released, got %v", released)
	}
}

func TestNoDevtoolsBrowserIsFoundWhenNoSocketBelongsToTheApp(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)
	connect := func(socket string) (cdpBrowser, func(), error) {
		return chrome.browser(), func() {}, nil
	}

	_, found := findDevtoolsBrowser("com.example.release", []string{"chrome_devtools_remote"}, connect)

	if found {
		t.Fatal("expected no browser for an app that serves no devtools")
	}
}

// Chrome freezes the tabs it is not showing, and a frozen tab accepts a
// command without ever answering it.
func TestCommandOnATabThatNeverAnswersFailsSayingSo(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)
	tabIsFrozen := make(chan struct{})
	t.Cleanup(func() { close(tabIsFrozen) })
	chrome.answer = func(fakeChromeCall) (any, string) {
		<-tabIsFrozen
		return map[string]any{}, ""
	}

	_, err := chrome.browser().evaluate("TAB1", "return 1", nil, 200*time.Millisecond)

	if err == nil || err.Error() != "webview TAB1 did not answer Runtime.evaluate within 200ms; a browser freezes the tabs it is not showing" {
		t.Fatalf("expected an explanation of the timeout, got %v", err)
	}
}

// A frozen tab never answers, and a poll on it must not outlive the wait.
func TestWaitOnATabThatNeverAnswersEndsAtTheTimeout(t *testing.T) {
	chrome := startFakeChrome(t, examplePage)
	tabIsFrozen := make(chan struct{})
	t.Cleanup(func() { close(tabIsFrozen) })
	chrome.answer = func(fakeChromeCall) (any, string) {
		<-tabIsFrozen
		return map[string]any{}, ""
	}

	start := time.Now()
	err := chrome.browser().WebViewWaitForLoadState("TAB1", "load", 300)

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("the wait took %s for a 300ms timeout", elapsed)
	}
	expected := "waitForLoadState timed out waiting for 'load': webview TAB1 did not answer Runtime.evaluate within 300ms; a browser freezes the tabs it is not showing"
	if err == nil || err.Error() != expected {
		t.Fatalf("expected the timeout to say why, got %v", err)
	}
}
