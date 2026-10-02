package devices

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	goios "github.com/danielpaulus/go-ios/ios"
)

type fakeInspectorApp struct {
	ID       string
	BundleID string
	Name     string
	// announcedLater apps are missing from the first application list and are
	// reported on their own afterwards, as the device does for some of them
	announcedLater bool
}

type fakeInspectorPage struct {
	ID    uint64
	Type  string
	URL   string
	Title string
}

type fakeInspectorCall struct {
	PageID uint64
	Method string
	Params map[string]any
}

// fakeWebInspector plays the web inspector service of a device: it reports the
// inspectable apps and their pages, and answers the protocol commands sent to a page.
type fakeWebInspector struct {
	t     *testing.T
	apps  []fakeInspectorApp
	pages map[string][]fakeInspectorPage
	// answer returns the result of a protocol command, or a protocol error message
	answer func(call fakeInspectorCall) (result any, protocolError string)
	// frozenPages accept commands without ever answering them, as the tabs
	// safari is not showing do
	frozenPages map[uint64]bool

	mu          sync.Mutex
	calls       []fakeInspectorCall
	connections []net.Conn
	client      *webInspectorClient
}

var (
	safariApp   = fakeInspectorApp{ID: "PID:525", BundleID: "com.apple.mobilesafari", Name: "Safari"}
	fitnessApp  = fakeInspectorApp{ID: "PID:1542", BundleID: "com.apple.fitcored", Name: "fitcored"}
	examplePage = fakeInspectorPage{ID: 2, Type: "WIRTypeWebPage", URL: "https://example.com/", Title: "Example Domain"}
)

func startFakeWebInspector(t *testing.T, apps ...fakeInspectorApp) *fakeWebInspector {
	t.Helper()
	inspector := &fakeWebInspector{t: t, apps: apps, pages: map[string][]fakeInspectorPage{}, frozenPages: map[uint64]bool{}}
	inspector.answer = func(fakeInspectorCall) (any, string) { return map[string]any{}, "" }
	inspector.client = &webInspectorClient{dial: inspector.dial}
	t.Cleanup(inspector.client.close)
	return inspector
}

// safariShowing starts an inspector of a device whose Safari has the given tabs open
func safariShowing(t *testing.T, pages ...fakeInspectorPage) *fakeWebInspector {
	t.Helper()
	inspector := startFakeWebInspector(t, fitnessApp, safariApp)
	inspector.pages[safariApp.ID] = pages
	return inspector
}

func (f *fakeWebInspector) safari() safariWebViews {
	return safariWebViews{inspector: f.client}
}

func (f *fakeWebInspector) connectionCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.connections)
}

// loseConnection drops the connection from the device side, as happens when
// the device is unplugged or its web inspector restarts
func (f *fakeWebInspector) loseConnection() {
	f.mu.Lock()
	defer f.mu.Unlock()
	_ = f.connections[len(f.connections)-1].Close()
}

func (f *fakeWebInspector) dial() (io.ReadWriteCloser, error) {
	client, device := net.Pipe()
	f.mu.Lock()
	f.connections = append(f.connections, device)
	f.mu.Unlock()
	go f.serve(device)
	return client, nil
}

func (f *fakeWebInspector) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	plist := goios.NewPlistCodecReadWriter(conn, conn)
	send := func(selector string, argument map[string]any) {
		_ = plist.Write(map[string]any{"__selector": selector, "__argument": argument})
	}

	for {
		var message map[string]any
		if err := plist.Read(&message); err != nil {
			return
		}
		argument, _ := message["__argument"].(map[string]any)
		appID, _ := argument["WIRApplicationIdentifierKey"].(string)
		pageID, _ := argument["WIRPageIdentifierKey"].(uint64)

		switch message["__selector"] {
		case "_rpc_reportIdentifier:":
			send("_rpc_reportCurrentState:", map[string]any{"WIRAutomationAvailabilityKey": "WIRAutomationAvailabilityNotAvailable"})
			known := map[string]any{}
			for _, app := range f.apps {
				if !app.announcedLater {
					known[app.ID] = app.description()
				}
			}
			send("_rpc_reportConnectedApplicationList:", map[string]any{"WIRApplicationDictionaryKey": known})
			for _, app := range f.apps {
				if app.announcedLater {
					send("_rpc_applicationConnected:", app.description())
				}
			}

		case "_rpc_forwardGetListing:":
			listing := map[string]any{}
			for _, page := range f.pages[appID] {
				listing[fmt.Sprint(page.ID)] = map[string]any{
					"WIRPageIdentifierKey": page.ID,
					"WIRTypeKey":           page.Type,
					"WIRURLKey":            page.URL,
					"WIRTitleKey":          page.Title,
				}
			}
			send("_rpc_applicationSentListing:", map[string]any{"WIRApplicationIdentifierKey": appID, "WIRListingKey": listing})

		case "_rpc_forwardSocketSetup:":
			sender := argument["WIRSenderKey"]
			f.sendToInspector(send, sender, map[string]any{
				"method": "Target.targetCreated",
				"params": map[string]any{"targetInfo": map[string]any{"targetId": "frame-1", "type": "frame"}},
			})
			f.sendToInspector(send, sender, map[string]any{
				"method": "Target.targetCreated",
				"params": map[string]any{"targetInfo": map[string]any{"targetId": fmt.Sprintf("page-%d", pageID), "type": "page"}},
			})

		case "_rpc_forwardSocketData:":
			f.answerCommand(send, argument["WIRSenderKey"], pageID, argument["WIRSocketDataKey"].([]byte))
		}
	}
}

func (app fakeInspectorApp) description() map[string]any {
	return map[string]any{
		"WIRApplicationIdentifierKey":       app.ID,
		"WIRApplicationBundleIdentifierKey": app.BundleID,
		"WIRApplicationNameKey":             app.Name,
	}
}

func (f *fakeWebInspector) sendToInspector(send func(string, map[string]any), sender any, message map[string]any) {
	data, _ := json.Marshal(message)
	send("_rpc_applicationSentData:", map[string]any{"WIRDestinationKey": sender, "WIRMessageDataKey": data})
}

// answerCommand unwraps a command addressed to the page target, and sends the
// answer back the way the device does: wrapped in a message from that target
func (f *fakeWebInspector) answerCommand(send func(string, map[string]any), sender any, pageID uint64, data []byte) {
	var outer struct {
		ID     int `json:"id"`
		Params struct {
			TargetID string `json:"targetId"`
			Message  string `json:"message"`
		} `json:"params"`
	}
	_ = json.Unmarshal(data, &outer)
	var command struct {
		ID     int            `json:"id"`
		Method string         `json:"method"`
		Params map[string]any `json:"params"`
	}
	_ = json.Unmarshal([]byte(outer.Params.Message), &command)

	f.sendToInspector(send, sender, map[string]any{"id": outer.ID, "result": map[string]any{}})
	if f.frozenPages[pageID] {
		return
	}
	answerToTarget := func(reply map[string]any) {
		inner, _ := json.Marshal(reply)
		f.sendToInspector(send, sender, map[string]any{
			"method": "Target.dispatchMessageFromTarget",
			"params": map[string]any{"targetId": outer.Params.TargetID, "message": string(inner)},
		})
	}
	// the sign of life a tab is asked for before anything else is not a call a test cares about
	if command.Method == "Runtime.evaluate" && command.Params["expression"] == safariSignOfLife {
		answerToTarget(map[string]any{"id": command.ID, "result": map[string]any{"result": map[string]any{"type": "number", "value": 1}, "wasThrown": false}})
		return
	}

	call := fakeInspectorCall{PageID: pageID, Method: command.Method, Params: command.Params}
	f.mu.Lock()
	f.calls = append(f.calls, call)
	f.mu.Unlock()

	result, protocolError := f.answer(call)
	reply := map[string]any{"id": command.ID, "result": result}
	if protocolError != "" {
		reply = map[string]any{"id": command.ID, "error": map[string]any{"code": -32601, "message": protocolError}}
	}
	answerToTarget(reply)
}

func (f *fakeWebInspector) callsOf(method string) []fakeInspectorCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var matching []fakeInspectorCall
	for _, call := range f.calls {
		if call.Method == method {
			matching = append(matching, call)
		}
	}
	return matching
}

func (f *fakeWebInspector) onlyCallOf(method string) fakeInspectorCall {
	f.t.Helper()
	calls := f.callsOf(method)
	if len(calls) != 1 {
		f.t.Fatalf("expected exactly one %s call, got %d", method, len(calls))
	}
	return calls[0]
}

// webkit has no way to await a promise while evaluating: it hands back a
// reference to the promise, which is then awaited with a second command
var promiseReference = map[string]any{
	"result":    map[string]any{"type": "object", "objectId": "promise-1", "className": "Promise"},
	"wasThrown": false,
}

// evaluatesTo makes every evaluation resolve to the given javascript value
func (f *fakeWebInspector) evaluatesTo(value any) {
	f.answer = func(call fakeInspectorCall) (any, string) {
		if call.Method == "Runtime.awaitPromise" {
			return map[string]any{"result": map[string]any{"type": "object", "value": value}, "wasThrown": false}, ""
		}
		return promiseReference, ""
	}
}

// throws makes the command of the given method fail with a javascript error
func (f *fakeWebInspector) throws(method, description string) {
	f.answer = func(call fakeInspectorCall) (any, string) {
		if call.Method == method {
			return map[string]any{
				"result":    map[string]any{"type": "object", "subtype": "error", "description": description},
				"wasThrown": true,
			}, ""
		}
		return promiseReference, ""
	}
}

func TestListingWebViewsReturnsTheTabsOfSafari(t *testing.T) {
	inspector := safariShowing(t, examplePage)
	inspector.pages[fitnessApp.ID] = []fakeInspectorPage{{ID: 9, Type: "WIRTypeJavaScript", Title: "a script context"}}
	inspector.evaluatesTo(true)

	webviews, err := inspector.safari().ListWebViews()

	if err != nil {
		t.Fatalf("ListWebViews: %v", err)
	}
	expected := []WebViewInfo{{
		ID:          "2",
		URL:         "https://example.com/",
		Title:       "Example Domain",
		BundleID:    "com.apple.mobilesafari",
		ProcessName: "Safari",
		IsVisible:   true,
	}}
	if !reflect.DeepEqual(webviews, expected) {
		t.Fatalf("expected %+v, got %+v", expected, webviews)
	}
}

func TestListingSkipsWhatSafariReportsThatIsNotAWebPage(t *testing.T) {
	serviceWorker := fakeInspectorPage{ID: 7, Type: "WIRTypeServiceWorker", URL: "https://example.com/sw.js"}
	inspector := safariShowing(t, serviceWorker, examplePage)
	inspector.evaluatesTo(true)

	webviews, err := inspector.safari().ListWebViews()

	if err != nil {
		t.Fatalf("ListWebViews: %v", err)
	}
	if len(webviews) != 1 || webviews[0].ID != "2" {
		t.Fatalf("expected only the web page, got %+v", webviews)
	}
}

func TestTabsAreListedInTheOrderOfTheirIds(t *testing.T) {
	inspector := safariShowing(t,
		fakeInspectorPage{ID: 10, Type: "WIRTypeWebPage"},
		fakeInspectorPage{ID: 2, Type: "WIRTypeWebPage"},
		fakeInspectorPage{ID: 1, Type: "WIRTypeWebPage"})

	webviews, err := inspector.safari().ListWebViews()

	if err != nil {
		t.Fatalf("ListWebViews: %v", err)
	}
	ids := []string{}
	for _, webview := range webviews {
		ids = append(ids, webview.ID)
	}
	if !reflect.DeepEqual(ids, []string{"1", "2", "10"}) {
		t.Fatalf("unexpected order %v", ids)
	}
}

func TestBackgroundTabOfSafariIsListedAsNotVisible(t *testing.T) {
	inspector := safariShowing(t, examplePage)
	inspector.evaluatesTo(false)

	webviews, err := inspector.safari().ListWebViews()

	if err != nil {
		t.Fatalf("ListWebViews: %v", err)
	}
	if len(webviews) != 1 || webviews[0].IsVisible {
		t.Fatalf("expected one hidden tab, got %+v", webviews)
	}
}

func TestSafariWithoutTabsListsNoWebViews(t *testing.T) {
	inspector := safariShowing(t)

	webviews, err := inspector.safari().ListWebViews()

	if err != nil {
		t.Fatalf("ListWebViews: %v", err)
	}
	if webviews == nil || len(webviews) != 0 {
		t.Fatalf("expected an empty list, got %#v", webviews)
	}
}

// The device reports some apps only after its first list of them.
func TestSafariIsFoundWhenTheDeviceAnnouncesItLater(t *testing.T) {
	lateSafari := safariApp
	lateSafari.announcedLater = true
	inspector := startFakeWebInspector(t, fitnessApp, lateSafari)
	inspector.pages[safariApp.ID] = []fakeInspectorPage{examplePage}

	webviews, err := inspector.safari().ListWebViews()

	if err != nil || len(webviews) != 1 {
		t.Fatalf("expected the tab of safari, got %+v, %v", webviews, err)
	}
}

// Safari only shows up in the inspector when its Web Inspector setting is on.
func TestSafariThatIsNotInspectableFailsNamingTheSettingToTurnOn(t *testing.T) {
	inspector := startFakeWebInspector(t, fitnessApp)
	safari := inspector.safari()
	safari.findSafariTimeout = 200 * time.Millisecond

	_, err := safari.ListWebViews()

	if err == nil || !strings.Contains(err.Error(), "Settings > Apps > Safari > Advanced > Web Inspector") {
		t.Fatalf("expected an error naming the setting, got %v", err)
	}
}

func TestSafariEvaluateReturnsTheJavascriptValue(t *testing.T) {
	inspector := safariShowing(t, examplePage)
	inspector.evaluatesTo(map[string]any{"title": "Example Domain"})

	value, err := inspector.safari().WebViewEvaluate("2", "return {title: document.title}", nil)

	if err != nil {
		t.Fatalf("WebViewEvaluate: %v", err)
	}
	if !reflect.DeepEqual(value, map[string]any{"title": "Example Domain"}) {
		t.Fatalf("unexpected value %#v", value)
	}
}

// The agent runs the expression as a function body and awaits what it returns,
// so the same expression has to mean the same thing in a safari tab.
func TestSafariEvaluateRunsTheExpressionAsAFunctionBodyAndAwaitsIt(t *testing.T) {
	inspector := safariShowing(t, examplePage)
	inspector.evaluatesTo("Example Domain")

	_, err := inspector.safari().WebViewEvaluate("2", "document.title", []any{"first", 2})

	if err != nil {
		t.Fatalf("WebViewEvaluate: %v", err)
	}
	expectedExpression := "Promise.resolve((function(){\nreturn (document.title)\n}).apply(null, [\"first\",2]))"
	if expression := inspector.onlyCallOf("Runtime.evaluate").Params["expression"]; expression != expectedExpression {
		t.Fatalf("unexpected expression %q", expression)
	}
	awaited := inspector.onlyCallOf("Runtime.awaitPromise").Params
	if awaited["promiseObjectId"] != "promise-1" || awaited["returnByValue"] != true {
		t.Fatalf("expected the returned promise to be awaited by value, got %v", awaited)
	}
}

func TestSafariEvaluateOfUndefinedReturnsNil(t *testing.T) {
	inspector := safariShowing(t, examplePage)
	inspector.answer = func(call fakeInspectorCall) (any, string) {
		if call.Method == "Runtime.awaitPromise" {
			return map[string]any{"result": map[string]any{"type": "undefined"}, "wasThrown": false}, ""
		}
		return promiseReference, ""
	}

	value, err := inspector.safari().WebViewEvaluate("2", "return undefined", nil)

	if err != nil || value != nil {
		t.Fatalf("expected nil without error, got %#v, %v", value, err)
	}
}

func TestSafariEvaluateFailsWithTheErrorTheExpressionThrew(t *testing.T) {
	inspector := safariShowing(t, examplePage)
	inspector.throws("Runtime.evaluate", "Error: boom")

	_, err := inspector.safari().WebViewEvaluate("2", "return (() => { throw new Error('boom') })()", nil)

	if err == nil || err.Error() != "Error: boom" {
		t.Fatalf("expected the thrown error, got %v", err)
	}
}

func TestSafariEvaluateFailsWithTheReasonThePromiseRejected(t *testing.T) {
	inspector := safariShowing(t, examplePage)
	inspector.throws("Runtime.awaitPromise", "Error: nope")

	_, err := inspector.safari().WebViewEvaluate("2", "return Promise.reject(new Error('nope'))", nil)

	if err == nil || err.Error() != "Error: nope" {
		t.Fatalf("expected the rejection, got %v", err)
	}
}

func TestSafariCommandOnAnUnknownWebViewFailsAsNotFound(t *testing.T) {
	inspector := safariShowing(t, examplePage)

	for _, unknownID := range []string{"99", "no-such-webview"} {
		err := inspector.safari().WebViewReload(unknownID)

		if !errors.Is(err, errSafariTabNotFound) || !strings.Contains(err.Error(), unknownID) {
			t.Fatalf("expected a not found error naming %q, got %v", unknownID, err)
		}
	}
}

func TestSafariProtocolErrorFailsTheCommand(t *testing.T) {
	inspector := safariShowing(t, examplePage)
	inspector.answer = func(fakeInspectorCall) (any, string) { return nil, "'Page.reload' was not found" }

	err := inspector.safari().WebViewReload("2")

	if err == nil || !strings.Contains(err.Error(), "'Page.reload' was not found") {
		t.Fatalf("expected the protocol error, got %v", err)
	}
}

// Safari does not keep the tabs it is not showing running, and such a tab
// accepts a command without ever answering it.
func TestSafariCommandOnATabThatDoesNotAnswerFailsQuicklySayingSo(t *testing.T) {
	inspector := safariShowing(t, examplePage)
	inspector.frozenPages[examplePage.ID] = true
	safari := inspector.safari()
	safari.livenessTimeout = 200 * time.Millisecond

	_, err := safari.WebViewEvaluate("2", "return 1", nil)

	expected := "webview 2 did not answer within 200ms; it is probably a background tab, which safari does not keep running"
	if err == nil || err.Error() != expected {
		t.Fatalf("expected an explanation of the timeout, got %v", err)
	}
}

func TestTabThatDoesNotAnswerIsListedAsNotVisible(t *testing.T) {
	backgroundTab := fakeInspectorPage{ID: 1, Type: "WIRTypeWebPage", URL: "https://example.org/"}
	inspector := safariShowing(t, backgroundTab, examplePage)
	inspector.frozenPages[backgroundTab.ID] = true
	inspector.evaluatesTo(true)

	webviews, err := inspector.safari().ListWebViews()

	if err != nil {
		t.Fatalf("ListWebViews: %v", err)
	}
	if len(webviews) != 2 || webviews[0].IsVisible || !webviews[1].IsVisible {
		t.Fatalf("expected only the answering tab to be visible, got %+v", webviews)
	}
}

// The web inspector stalls a new connection for seconds after one was closed,
// so the connection is kept and shared.
func TestCommandsShareOneConnectionToTheWebInspector(t *testing.T) {
	inspector := safariShowing(t, examplePage)
	inspector.evaluatesTo("Example Domain")
	safari := inspector.safari()

	for range 3 {
		if _, err := safari.WebViewEvaluate("2", "document.title", nil); err != nil {
			t.Fatalf("WebViewEvaluate: %v", err)
		}
	}

	if connections := inspector.connectionCount(); connections != 1 {
		t.Fatalf("expected a single connection, got %d", connections)
	}
}

func TestCommandConnectsAgainAfterTheConnectionWasLost(t *testing.T) {
	inspector := safariShowing(t, examplePage)
	inspector.evaluatesTo("Example Domain")
	safari := inspector.safari()
	if _, err := safari.WebViewEvaluate("2", "document.title", nil); err != nil {
		t.Fatalf("WebViewEvaluate: %v", err)
	}

	inspector.loseConnection()
	waitUntilTheClientNoticed(t, inspector.client)
	title, err := safari.WebViewEvaluate("2", "document.title", nil)

	if err != nil || title != "Example Domain" {
		t.Fatalf("expected the command to work on a new connection, got %v, %v", title, err)
	}
}

func waitUntilTheClientNoticed(t *testing.T, client *webInspectorClient) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !client.session.isClosed() {
		if time.Now().After(deadline) {
			t.Fatal("the client never noticed the lost connection")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// WebKit has no command to navigate a page, so the page is told to navigate
// itself. The url travels as an argument, not as javascript source.
func TestSafariGotoNavigatesTheTabToTheUrl(t *testing.T) {
	inspector := safariShowing(t, examplePage)

	err := inspector.safari().WebViewGoto("2", "https://mobilewright.dev/?q='quoted'")

	if err != nil {
		t.Fatalf("WebViewGoto: %v", err)
	}
	expression := inspector.onlyCallOf("Runtime.evaluate").Params["expression"].(string)
	if expression != "(function(){\nwindow.location.href = arguments[0]\n}).apply(null, [\"https://mobilewright.dev/?q='quoted'\"])" {
		t.Fatalf("unexpected expression %q", expression)
	}
}

// The page that answers is torn down by the navigation it starts, so nothing
// may be awaited from it afterwards.
func TestSafariNavigationDoesNotAwaitThePageItLeaves(t *testing.T) {
	inspector := safariShowing(t, examplePage)
	safari := inspector.safari()

	if err := safari.WebViewGoto("2", "https://mobilewright.dev/"); err != nil {
		t.Fatalf("WebViewGoto: %v", err)
	}
	if err := safari.WebViewGoBack("2"); err != nil {
		t.Fatalf("WebViewGoBack: %v", err)
	}
	if err := safari.WebViewGoForward("2"); err != nil {
		t.Fatalf("WebViewGoForward: %v", err)
	}

	if awaited := inspector.callsOf("Runtime.awaitPromise"); len(awaited) != 0 {
		t.Fatalf("expected no promise to be awaited, got %v", awaited)
	}
}

func TestSafariReloadReloadsTheTab(t *testing.T) {
	inspector := safariShowing(t, examplePage)

	err := inspector.safari().WebViewReload("2")

	if err != nil {
		t.Fatalf("WebViewReload: %v", err)
	}
	inspector.onlyCallOf("Page.reload")
}

func TestSafariGoBackAndGoForwardMoveThroughTheHistoryOfTheTab(t *testing.T) {
	inspector := safariShowing(t, examplePage)
	safari := inspector.safari()

	if err := safari.WebViewGoBack("2"); err != nil {
		t.Fatalf("WebViewGoBack: %v", err)
	}
	if err := safari.WebViewGoForward("2"); err != nil {
		t.Fatalf("WebViewGoForward: %v", err)
	}

	evaluations := inspector.callsOf("Runtime.evaluate")
	if len(evaluations) != 2 ||
		!strings.Contains(evaluations[0].Params["expression"].(string), "history.back()") ||
		!strings.Contains(evaluations[1].Params["expression"].(string), "history.forward()") {
		t.Fatalf("expected history.back() then history.forward(), got %v", evaluations)
	}
}

func TestSafariContentReturnsTheHtmlOfTheDocument(t *testing.T) {
	inspector := safariShowing(t, examplePage)
	inspector.evaluatesTo("<html><body>hi</body></html>")

	content, err := inspector.safari().WebViewContent("2")

	if err != nil {
		t.Fatalf("WebViewContent: %v", err)
	}
	if content != "<html><body>hi</body></html>" {
		t.Fatalf("unexpected content %q", content)
	}
}

func TestSafariWaitReturnsOnceTheDocumentHasLoaded(t *testing.T) {
	inspector := safariShowing(t, examplePage)
	polls := 0
	inspector.answer = func(call fakeInspectorCall) (any, string) {
		if call.Method != "Runtime.awaitPromise" {
			return promiseReference, ""
		}
		polls++
		return map[string]any{"result": map[string]any{"type": "boolean", "value": polls >= 3}, "wasThrown": false}, ""
	}

	err := inspector.safari().WebViewWaitForLoadState("2", "load", 5000)

	if err != nil {
		t.Fatalf("WebViewWaitForLoadState: %v", err)
	}
	if polls != 3 {
		t.Fatalf("expected to stop polling once loaded, polled %d times", polls)
	}
}

func TestSafariWaitForDomContentLoadedAlsoAcceptsAnInteractiveDocument(t *testing.T) {
	inspector := safariShowing(t, examplePage)
	inspector.evaluatesTo(true)

	err := inspector.safari().WebViewWaitForLoadState("2", "domcontentloaded", 5000)

	if err != nil {
		t.Fatalf("WebViewWaitForLoadState: %v", err)
	}
	expression := inspector.onlyCallOf("Runtime.evaluate").Params["expression"].(string)
	if !strings.Contains(expression, "'interactive'") {
		t.Fatalf("expected the interactive state to be accepted, evaluated %q", expression)
	}
}

func TestSafariWaitTimesOutWhenTheDocumentNeverLoads(t *testing.T) {
	inspector := safariShowing(t, examplePage)
	inspector.evaluatesTo(false)

	err := inspector.safari().WebViewWaitForLoadState("2", "load", 300)

	if err == nil || err.Error() != "waitForLoadState timed out waiting for 'load'" {
		t.Fatalf("expected a timeout, got %v", err)
	}
}

func TestSafariWaitOnAnUnknownWebViewFailsWithoutWaitingForTheTimeout(t *testing.T) {
	inspector := safariShowing(t, examplePage)

	err := inspector.safari().WebViewWaitForLoadState("99", "load", 60_000)

	if !errors.Is(err, errSafariTabNotFound) {
		t.Fatalf("expected a not found error, got %v", err)
	}
}
