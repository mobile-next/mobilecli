package devices

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	goios "github.com/danielpaulus/go-ios/ios"
)

// Safari cannot take the injected agent: it is not a debuggable build. A device
// does run a web inspector service, the one Safari on a Mac connects to, and it
// reports the tabs of Safari and relays the WebKit inspector protocol to them.
// The tabs are driven through that instead, behind the same webview commands.
//
// The service only reports Safari when Settings > Apps > Safari > Advanced >
// Web Inspector is turned on.

const (
	webInspectorService = "com.apple.webinspector"
	safariBundleID      = "com.apple.mobilesafari"

	webInspectorWebPageType = "WIRTypeWebPage"

	safariFindTimeout    = 5 * time.Second
	safariCommandTimeout = 30 * time.Second
	// a tab that does not answer is reported as hidden instead of holding up the list
	safariVisibilityTimeout  = 1 * time.Second
	safariLoadStatePollEvery = 200 * time.Millisecond
)

var errSafariTabNotFound = errors.New("webview not found")

// safariWebViews drives the tabs of Safari as webviews, through the web
// inspector service that dial connects to.
type safariWebViews struct {
	dial func() (io.ReadWriteCloser, error)
	// zero means the default; tests shorten them
	findSafariTimeout time.Duration
	commandTimeout    time.Duration
}

// dialWebInspector connects to the web inspector service of a real device.
func dialWebInspector(udid string) (io.ReadWriteCloser, error) {
	device, err := goios.GetDevice(udid)
	if err != nil {
		return nil, fmt.Errorf("device not found: %s: %w", udid, err)
	}
	conn, err := goios.ConnectToService(device, webInspectorService)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", webInspectorService, err)
	}
	return deviceServiceConn{conn}, nil
}

// deviceServiceConn makes a go-ios service connection an io.ReadWriteCloser.
type deviceServiceConn struct {
	conn goios.DeviceConnectionInterface
}

func (c deviceServiceConn) Read(p []byte) (int, error)  { return c.conn.Reader().Read(p) }
func (c deviceServiceConn) Write(p []byte) (int, error) { return c.conn.Writer().Write(p) }
func (c deviceServiceConn) Close() error                { return c.conn.Close() }

type webInspectorMessage struct {
	selector string
	argument map[string]any
}

// webInspectorSession is one connection to the web inspector service.
type webInspectorSession struct {
	conn         io.ReadWriteCloser
	plist        goios.PlistCodecReadWriter
	connectionID string
	// messages is fed by a reader of its own, so that waiting for one can time out
	messages  chan webInspectorMessage
	closeOnce sync.Once
}

func newInspectorID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate web inspector id: %w", err)
	}
	return strings.ToUpper(fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])), nil
}

func (s safariWebViews) openSession() (*webInspectorSession, error) {
	connectionID, err := newInspectorID()
	if err != nil {
		return nil, err
	}
	conn, err := s.dial()
	if err != nil {
		return nil, err
	}

	session := &webInspectorSession{
		conn:         conn,
		plist:        goios.NewPlistCodecReadWriter(conn, conn),
		connectionID: connectionID,
		messages:     make(chan webInspectorMessage, 64),
	}
	go session.readMessages()

	if err := session.send("_rpc_reportIdentifier:", map[string]any{}); err != nil {
		session.close()
		return nil, err
	}
	return session, nil
}

func (session *webInspectorSession) readMessages() {
	defer close(session.messages)
	for {
		var raw map[string]any
		if err := session.plist.Read(&raw); err != nil {
			return
		}
		selector, _ := raw["__selector"].(string)
		argument, _ := raw["__argument"].(map[string]any)
		session.messages <- webInspectorMessage{selector: selector, argument: argument}
	}
}

func (session *webInspectorSession) close() {
	session.closeOnce.Do(func() {
		_ = session.conn.Close()
		// let the reader finish instead of leaving it blocked on a full channel
		go func() {
			for range session.messages {
			}
		}()
	})
}

func (session *webInspectorSession) send(selector string, argument map[string]any) error {
	argument["WIRConnectionIdentifierKey"] = session.connectionID
	if err := session.plist.Write(map[string]any{"__selector": selector, "__argument": argument}); err != nil {
		return fmt.Errorf("send %s: %w", selector, err)
	}
	return nil
}

var errInspectorTimedOut = errors.New("web inspector did not answer in time")

// nextMessage waits for the next message from the device, until deadline.
func (session *webInspectorSession) nextMessage(deadline time.Time) (webInspectorMessage, error) {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case message, ok := <-session.messages:
		if !ok {
			return webInspectorMessage{}, errors.New("web inspector connection closed")
		}
		return message, nil
	case <-timer.C:
		return webInspectorMessage{}, errInspectorTimedOut
	}
}

type inspectorApp struct {
	id   string
	name string
}

func inspectorAppWithBundleID(description map[string]any, bundleID string) (inspectorApp, bool) {
	if description["WIRApplicationBundleIdentifierKey"] != bundleID {
		return inspectorApp{}, false
	}
	id, _ := description["WIRApplicationIdentifierKey"].(string)
	name, _ := description["WIRApplicationNameKey"].(string)
	return inspectorApp{id: id, name: name}, id != ""
}

// findSafari waits for the device to report Safari among its inspectable apps.
// The device lists the apps it knows on connect, and announces others after.
func (session *webInspectorSession) findSafari(timeout time.Duration) (inspectorApp, error) {
	deadline := time.Now().Add(timeout)
	for {
		message, err := session.nextMessage(deadline)
		if errors.Is(err, errInspectorTimedOut) {
			return inspectorApp{}, errors.New("safari is not inspectable: open it, and turn on Settings > Apps > Safari > Advanced > Web Inspector on the device")
		}
		if err != nil {
			return inspectorApp{}, err
		}

		switch message.selector {
		case "_rpc_reportConnectedApplicationList:":
			apps, _ := message.argument["WIRApplicationDictionaryKey"].(map[string]any)
			for _, description := range apps {
				if app, found := inspectorAppWithBundleID(asStringMap(description), safariBundleID); found {
					return app, nil
				}
			}
		case "_rpc_applicationConnected:", "_rpc_applicationUpdated:":
			if app, found := inspectorAppWithBundleID(message.argument, safariBundleID); found {
				return app, nil
			}
		}
	}
}

func asStringMap(value any) map[string]any {
	asMap, _ := value.(map[string]any)
	return asMap
}

type inspectorPage struct {
	id    uint64
	url   string
	title string
}

// webPages asks an app for what it has that can be inspected, and returns the
// web pages among it, ordered by id.
func (session *webInspectorSession) webPages(app inspectorApp, timeout time.Duration) ([]inspectorPage, error) {
	if err := session.send("_rpc_forwardGetListing:", map[string]any{"WIRApplicationIdentifierKey": app.id}); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	for {
		message, err := session.nextMessage(deadline)
		if err != nil {
			return nil, fmt.Errorf("list the tabs of safari: %w", err)
		}
		if message.selector != "_rpc_applicationSentListing:" || message.argument["WIRApplicationIdentifierKey"] != app.id {
			continue
		}

		pages := []inspectorPage{}
		listing, _ := message.argument["WIRListingKey"].(map[string]any)
		for _, entry := range listing {
			description := asStringMap(entry)
			if description["WIRTypeKey"] != webInspectorWebPageType {
				continue
			}
			id, ok := description["WIRPageIdentifierKey"].(uint64)
			if !ok {
				continue
			}
			url, _ := description["WIRURLKey"].(string)
			title, _ := description["WIRTitleKey"].(string)
			pages = append(pages, inspectorPage{id: id, url: url, title: title})
		}
		sort.Slice(pages, func(i, j int) bool { return pages[i].id < pages[j].id })
		return pages, nil
	}
}

// inspectorTab is a session attached to one tab, ready to take commands.
type inspectorTab struct {
	session   *webInspectorSession
	app       inspectorApp
	webviewID string
	pageID    uint64
	senderID  string
	// targetID names the page inside the tab: commands are not sent to the tab
	// itself but wrapped in a message to this target
	targetID      string
	lastCommandID int
	timeout       time.Duration
}

// attach opens a session and attaches it to the tab with the given webview id.
// The caller closes the session of the returned tab.
func (s safariWebViews) attach(webviewID string, timeout time.Duration) (*inspectorTab, error) {
	session, err := s.openSession()
	if err != nil {
		return nil, err
	}
	tab, err := s.attachOn(session, webviewID, timeout)
	if err != nil {
		session.close()
		return nil, err
	}
	return tab, nil
}

func (s safariWebViews) attachOn(session *webInspectorSession, webviewID string, timeout time.Duration) (*inspectorTab, error) {
	app, err := session.findSafari(s.findTimeout())
	if err != nil {
		return nil, err
	}
	pages, err := session.webPages(app, timeout)
	if err != nil {
		return nil, err
	}
	pageID, err := strconv.ParseUint(webviewID, 10, 64)
	if err != nil || !hasInspectorPage(pages, pageID) {
		return nil, fmt.Errorf("%w: %s", errSafariTabNotFound, webviewID)
	}

	senderID, err := newInspectorID()
	if err != nil {
		return nil, err
	}
	tab := &inspectorTab{session: session, app: app, webviewID: webviewID, pageID: pageID, senderID: senderID, timeout: timeout}
	if err := session.send("_rpc_forwardSocketSetup:", tab.addressed(map[string]any{"WIRAutomaticallyPause": false})); err != nil {
		return nil, err
	}

	// the tab introduces its targets once attached; the page is the one to talk to
	deadline := time.Now().Add(timeout)
	for {
		event, err := tab.nextProtocolMessage(deadline, "attach")
		if err != nil {
			return nil, err
		}
		if event.Method == "Target.targetCreated" && event.Params.TargetInfo.Type == "page" {
			tab.targetID = event.Params.TargetInfo.TargetID
			return tab, nil
		}
	}
}

func hasInspectorPage(pages []inspectorPage, id uint64) bool {
	for _, page := range pages {
		if page.id == id {
			return true
		}
	}
	return false
}

func (tab *inspectorTab) addressed(argument map[string]any) map[string]any {
	argument["WIRApplicationIdentifierKey"] = tab.app.id
	argument["WIRPageIdentifierKey"] = tab.pageID
	argument["WIRSenderKey"] = tab.senderID
	return argument
}

type inspectorProtocolError struct {
	Message string `json:"message"`
}

// inspectorProtocolMessage is a message of the WebKit inspector protocol: the
// reply to a command when it has an id, an event otherwise.
type inspectorProtocolMessage struct {
	ID     int                     `json:"id"`
	Method string                  `json:"method"`
	Result json.RawMessage         `json:"result"`
	Error  *inspectorProtocolError `json:"error"`
	Params struct {
		TargetInfo struct {
			TargetID string `json:"targetId"`
			Type     string `json:"type"`
		} `json:"targetInfo"`
		// set on Target.dispatchMessageFromTarget: a protocol message of the target
		Message string `json:"message"`
	} `json:"params"`
}

// nextProtocolMessage waits for the next protocol message the tab sends.
func (tab *inspectorTab) nextProtocolMessage(deadline time.Time, doing string) (inspectorProtocolMessage, error) {
	for {
		message, err := tab.session.nextMessage(deadline)
		if errors.Is(err, errInspectorTimedOut) {
			return inspectorProtocolMessage{}, fmt.Errorf("webview %s did not answer %s within %s", tab.webviewID, doing, tab.timeout)
		}
		if err != nil {
			return inspectorProtocolMessage{}, err
		}
		if message.selector != "_rpc_applicationSentData:" || message.argument["WIRDestinationKey"] != tab.senderID {
			continue
		}
		data, _ := message.argument["WIRMessageDataKey"].([]byte)
		var protocolMessage inspectorProtocolMessage
		if err := json.Unmarshal(data, &protocolMessage); err != nil {
			return inspectorProtocolMessage{}, fmt.Errorf("parse web inspector message: %w", err)
		}
		return protocolMessage, nil
	}
}

// command sends one protocol command to the page of the tab and returns its result.
func (tab *inspectorTab) command(method string, params map[string]any) (json.RawMessage, error) {
	tab.lastCommandID++
	commandID := tab.lastCommandID
	command, err := json.Marshal(map[string]any{"id": commandID, "method": method, "params": params})
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", method, err)
	}

	tab.lastCommandID++
	envelopeID := tab.lastCommandID
	envelope, err := json.Marshal(map[string]any{
		"id":     envelopeID,
		"method": "Target.sendMessageToTarget",
		"params": map[string]any{"targetId": tab.targetID, "message": string(command)},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", method, err)
	}
	if err := tab.session.send("_rpc_forwardSocketData:", tab.addressed(map[string]any{"WIRSocketDataKey": envelope})); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(tab.timeout)
	for {
		message, err := tab.nextProtocolMessage(deadline, method)
		if err != nil {
			return nil, err
		}
		// the envelope is only answered with an error when it could not be delivered
		if message.ID == envelopeID && message.Error != nil {
			return nil, fmt.Errorf("%s: %s", method, message.Error.Message)
		}
		if message.Method != "Target.dispatchMessageFromTarget" {
			continue
		}

		var reply inspectorProtocolMessage
		if err := json.Unmarshal([]byte(message.Params.Message), &reply); err != nil {
			return nil, fmt.Errorf("parse %s reply: %w", method, err)
		}
		if reply.ID != commandID {
			continue
		}
		if reply.Error != nil {
			return nil, fmt.Errorf("%s: %s", method, reply.Error.Message)
		}
		return reply.Result, nil
	}
}

// inspectorRemoteObject is how the protocol hands back a javascript value: by
// value, or as a reference to an object that stays in the page.
type inspectorRemoteObject struct {
	Result struct {
		Value       any    `json:"value"`
		ObjectID    string `json:"objectId"`
		Description string `json:"description"`
	} `json:"result"`
	WasThrown bool `json:"wasThrown"`
}

// evaluateOnly runs a function body with args in the page, without waiting for
// a promise it returns, and hands back what it returned.
func (tab *inspectorTab) evaluateOnly(expression string) (inspectorRemoteObject, error) {
	raw, err := tab.command("Runtime.evaluate", map[string]any{"expression": expression})
	if err != nil {
		return inspectorRemoteObject{}, err
	}
	return thrownOrReturned(raw)
}

func thrownOrReturned(raw json.RawMessage) (inspectorRemoteObject, error) {
	var object inspectorRemoteObject
	if err := json.Unmarshal(raw, &object); err != nil {
		return inspectorRemoteObject{}, fmt.Errorf("parse evaluate result: %w", err)
	}
	if object.WasThrown {
		return inspectorRemoteObject{}, errors.New(object.Result.Description)
	}
	return object, nil
}

// functionCall is the javascript that runs body as a function called with args.
func functionCall(body string, args []any) (string, error) {
	if args == nil {
		args = []any{}
	}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("marshal evaluate args: %w", err)
	}
	return "(function(){\n" + body + "\n}).apply(null, " + string(argsJSON) + ")", nil
}

func (s safariWebViews) findTimeout() time.Duration {
	if s.findSafariTimeout > 0 {
		return s.findSafariTimeout
	}
	return safariFindTimeout
}

func (s safariWebViews) timeout() time.Duration {
	if s.commandTimeout > 0 {
		return s.commandTimeout
	}
	return safariCommandTimeout
}

func (s safariWebViews) ListWebViews() ([]WebViewInfo, error) {
	session, err := s.openSession()
	if err != nil {
		return nil, err
	}
	defer session.close()

	safari, err := session.findSafari(s.findTimeout())
	if err != nil {
		return nil, err
	}
	pages, err := session.webPages(safari, s.timeout())
	if err != nil {
		return nil, err
	}

	webviews := make([]WebViewInfo, len(pages))
	for i, page := range pages {
		webviews[i] = WebViewInfo{
			ID:          strconv.FormatUint(page.id, 10),
			URL:         page.url,
			Title:       page.title,
			BundleID:    safariBundleID,
			ProcessName: safari.name,
		}
	}

	// the listing does not say which tab is in front, so ask each one
	var wg sync.WaitGroup
	for i := range webviews {
		wg.Add(1)
		go func(webview *WebViewInfo) {
			defer wg.Done()
			webview.IsVisible = s.isVisible(webview.ID)
		}(&webviews[i])
	}
	wg.Wait()

	return webviews, nil
}

func (s safariWebViews) isVisible(webviewID string) bool {
	visible, err := s.evaluate(webviewID, "return document.visibilityState === 'visible'", nil, safariVisibilityTimeout)
	return err == nil && visible == true
}

// navigate runs a function body that makes the tab leave its page. The page
// that answers is torn down by the navigation, so nothing is awaited from it.
func (s safariWebViews) navigate(webviewID, body string, args []any) error {
	tab, err := s.attach(webviewID, s.timeout())
	if err != nil {
		return err
	}
	defer tab.session.close()

	expression, err := functionCall(body, args)
	if err != nil {
		return err
	}
	_, err = tab.evaluateOnly(expression)
	return err
}

// WebKit has no command to navigate a page, so the page is told to do it.
func (s safariWebViews) WebViewGoto(webviewID, url string) error {
	return s.navigate(webviewID, "window.location.href = arguments[0]", []any{url})
}

func (s safariWebViews) WebViewGoBack(webviewID string) error {
	return s.navigate(webviewID, "history.back()", nil)
}

func (s safariWebViews) WebViewGoForward(webviewID string) error {
	return s.navigate(webviewID, "history.forward()", nil)
}

func (s safariWebViews) WebViewReload(webviewID string) error {
	tab, err := s.attach(webviewID, s.timeout())
	if err != nil {
		return err
	}
	defer tab.session.close()

	_, err = tab.command("Page.reload", map[string]any{})
	return err
}

func (s safariWebViews) WebViewContent(webviewID string) (string, error) {
	result, err := s.WebViewEvaluate(webviewID, "return document.documentElement.outerHTML", nil)
	if err != nil {
		return "", err
	}
	content, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("unexpected content type %T", result)
	}
	return content, nil
}

func (s safariWebViews) WebViewEvaluate(webviewID, expression string, args []any) (any, error) {
	return s.evaluate(webviewID, expression, args, s.timeout())
}

// evaluate runs expression the way the injected agent does: as the body of a
// function called with args, whose returned promise is awaited.
func (s safariWebViews) evaluate(webviewID, expression string, args []any, timeout time.Duration) (any, error) {
	tab, err := s.attach(webviewID, timeout)
	if err != nil {
		return nil, err
	}
	defer tab.session.close()

	call, err := functionCall(ensureReturnExpression(expression), args)
	if err != nil {
		return nil, err
	}
	// WebKit cannot await while evaluating. Whatever the function returns is
	// made a promise, which is handed back by reference and awaited separately.
	promise, err := tab.evaluateOnly("Promise.resolve(" + call + ")")
	if err != nil {
		return nil, err
	}
	raw, err := tab.command("Runtime.awaitPromise", map[string]any{
		"promiseObjectId": promise.Result.ObjectID,
		"returnByValue":   true,
	})
	if err != nil {
		return nil, err
	}
	settled, err := thrownOrReturned(raw)
	if err != nil {
		return nil, err
	}
	return settled.Result.Value, nil
}

func (s safariWebViews) WebViewWaitForLoadState(webviewID, state string, timeoutMs int) error {
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
		// an evaluation fails while the tab swaps pages mid-navigation, so only
		// an unknown tab ends the wait early
		loaded, err := s.evaluate(webviewID, hasLoaded, nil, s.timeout())
		if errors.Is(err, errSafariTabNotFound) {
			return err
		}
		if err == nil && loaded == true {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("waitForLoadState timed out waiting for '%s'", state)
		}
		time.Sleep(safariLoadStatePollEvery)
	}
}
