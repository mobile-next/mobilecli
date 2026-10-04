package devices

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
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
// A real device only reports Safari when Settings > Apps > Safari > Advanced >
// Web Inspector is turned on. A simulator always does.
//
// One connection per device is kept open and shared by every command. The
// service answers a connection that stays open within milliseconds, but stalls
// the next one for seconds after a connection was closed on it.

const (
	webInspectorService = "com.apple.webinspector"
	safariBundleID      = "com.apple.mobilesafari"

	webInspectorWebPageType = "WIRTypeWebPage"

	safariFindTimeout    = 5 * time.Second
	safariCommandTimeout = 30 * time.Second
	// a tab that runs answers within milliseconds. Safari does not keep the tabs
	// it is not showing running, and such a tab accepts a command without ever
	// answering it, so a tab is asked for a sign of life before anything else.
	safariLivenessTimeout   = 2 * time.Second
	safariVisibilityTimeout = 1 * time.Second

	safariLoadStatePollEvery = 200 * time.Millisecond
)

var (
	errSafariTabNotFound = errors.New("webview not found")
	errInspectorTimedOut = errors.New("web inspector did not answer in time")
	errInspectorClosed   = errors.New("web inspector connection closed")
)

type webInspectorDialer func() (io.ReadWriteCloser, error)

// dialDeviceWebInspector connects to the web inspector service of a real device.
func dialDeviceWebInspector(udid string) (io.ReadWriteCloser, error) {
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

// dialSimulatorWebInspector connects to the web inspector of a simulator, which
// listens on a unix socket on this machine that its launchd names.
func dialSimulatorWebInspector(udid string) (io.ReadWriteCloser, error) {
	output, err := runSimctl("spawn", udid, "launchctl", "getenv", "RWI_LISTEN_SOCKET")
	if err != nil {
		return nil, fmt.Errorf("find the web inspector socket of simulator %s: %w", udid, err)
	}
	socketPath := strings.TrimSpace(string(output))
	if socketPath == "" {
		return nil, fmt.Errorf("simulator %s has no web inspector socket", udid)
	}
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("connect to the web inspector of simulator %s: %w", udid, err)
	}
	return conn, nil
}

// deviceServiceConn makes a go-ios service connection an io.ReadWriteCloser.
type deviceServiceConn struct {
	conn goios.DeviceConnectionInterface
}

func (c deviceServiceConn) Read(p []byte) (int, error)  { return c.conn.Reader().Read(p) }
func (c deviceServiceConn) Write(p []byte) (int, error) { return c.conn.Writer().Write(p) }
func (c deviceServiceConn) Close() error                { return c.conn.Close() }

// webInspectorClient keeps the one connection to the web inspector of a device,
// and connects again when it was lost.
type webInspectorClient struct {
	dial webInspectorDialer

	mu      sync.Mutex
	session *webInspectorSession
}

var (
	webInspectorClients   = map[string]*webInspectorClient{}
	webInspectorClientsMu sync.Mutex
)

// webInspectorOf returns the client of the device with the given udid, shared
// by all commands of this process.
func webInspectorOf(udid string, dial webInspectorDialer) *webInspectorClient {
	webInspectorClientsMu.Lock()
	defer webInspectorClientsMu.Unlock()
	client, ok := webInspectorClients[udid]
	if !ok {
		client = &webInspectorClient{dial: dial}
		webInspectorClients[udid] = client
	}
	return client
}

func (c *webInspectorClient) connected() (*webInspectorSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != nil && !c.session.isClosed() {
		return c.session, nil
	}

	connectionID, err := newInspectorID()
	if err != nil {
		return nil, err
	}
	conn, err := c.dial()
	if err != nil {
		return nil, err
	}
	session := newWebInspectorSession(conn, connectionID)
	if err := session.send("_rpc_reportIdentifier:", map[string]any{}); err != nil {
		session.close()
		return nil, err
	}
	c.session = session
	return session, nil
}

func (c *webInspectorClient) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != nil {
		c.session.close()
	}
}

func newInspectorID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate web inspector id: %w", err)
	}
	return strings.ToUpper(fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])), nil
}

type inspectorApp struct {
	id       string
	bundleID string
	name     string
}

type inspectorPage struct {
	id    uint64
	url   string
	title string
}

type inspectorListing struct {
	// revision counts the listings the app has sent, to tell a new one from the last
	revision int
	pages    []inspectorPage
}

// webInspectorSession is one connection to the web inspector service. The
// device keeps reporting on it: which apps can be inspected, what they have
// open, and what the attached tabs say. A reader keeps that state current, and
// commands wait for the state they need.
type webInspectorSession struct {
	conn         io.ReadWriteCloser
	plist        goios.PlistCodecReadWriter
	connectionID string
	writeMu      sync.Mutex

	mu       sync.Mutex
	apps     map[string]inspectorApp
	listings map[string]inspectorListing
	tabs     map[string]*inspectorTab
	closed   bool
	// changed is closed, and replaced, whenever anything above changes
	changed chan struct{}
}

func newWebInspectorSession(conn io.ReadWriteCloser, connectionID string) *webInspectorSession {
	session := &webInspectorSession{
		conn:         conn,
		plist:        goios.NewPlistCodecReadWriter(conn, conn),
		connectionID: connectionID,
		apps:         map[string]inspectorApp{},
		listings:     map[string]inspectorListing{},
		tabs:         map[string]*inspectorTab{},
		changed:      make(chan struct{}),
	}
	go session.readMessages()
	return session
}

func (session *webInspectorSession) isClosed() bool {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.closed
}

func (session *webInspectorSession) close() {
	_ = session.conn.Close()
	session.mu.Lock()
	defer session.mu.Unlock()
	session.closed = true
	session.notifyChange()
}

// notifyChange wakes everyone waiting for the state to change. Called with mu held.
func (session *webInspectorSession) notifyChange() {
	close(session.changed)
	session.changed = make(chan struct{})
}

func (session *webInspectorSession) send(selector string, argument map[string]any) error {
	argument["WIRConnectionIdentifierKey"] = session.connectionID
	session.writeMu.Lock()
	defer session.writeMu.Unlock()
	if err := session.plist.Write(map[string]any{"__selector": selector, "__argument": argument}); err != nil {
		session.close()
		return fmt.Errorf("send %s: %w", selector, err)
	}
	return nil
}

// await waits until reached reports true, which it is asked with mu held every
// time the state changes.
func (session *webInspectorSession) await(deadline time.Time, reached func() bool) error {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	for {
		session.mu.Lock()
		if reached() {
			session.mu.Unlock()
			return nil
		}
		if session.closed {
			session.mu.Unlock()
			return errInspectorClosed
		}
		changed := session.changed
		session.mu.Unlock()

		select {
		case <-changed:
		case <-timer.C:
			return errInspectorTimedOut
		}
	}
}

func (session *webInspectorSession) readMessages() {
	for {
		var message map[string]any
		if err := session.plist.Read(&message); err != nil {
			session.close()
			return
		}
		selector, _ := message["__selector"].(string)
		session.handle(selector, asStringMap(message["__argument"]))
	}
}

func asStringMap(value any) map[string]any {
	asMap, _ := value.(map[string]any)
	return asMap
}

func (session *webInspectorSession) handle(selector string, argument map[string]any) {
	session.mu.Lock()
	defer session.mu.Unlock()

	switch selector {
	case "_rpc_reportConnectedApplicationList:":
		session.apps = map[string]inspectorApp{}
		for _, description := range asStringMap(argument["WIRApplicationDictionaryKey"]) {
			session.rememberApp(asStringMap(description))
		}
	case "_rpc_applicationConnected:", "_rpc_applicationUpdated:":
		session.rememberApp(argument)
	case "_rpc_applicationDisconnected:":
		appID, _ := argument["WIRApplicationIdentifierKey"].(string)
		delete(session.apps, appID)
	case "_rpc_applicationSentListing:":
		appID, _ := argument["WIRApplicationIdentifierKey"].(string)
		session.listings[appID] = inspectorListing{
			revision: session.listings[appID].revision + 1,
			pages:    webPagesOf(asStringMap(argument["WIRListingKey"])),
		}
	case "_rpc_applicationSentData:":
		destination, _ := argument["WIRDestinationKey"].(string)
		data, _ := argument["WIRMessageDataKey"].([]byte)
		if tab, attached := session.tabs[destination]; attached {
			tab.handle(data)
		}
	default:
		return
	}
	session.notifyChange()
}

func (session *webInspectorSession) rememberApp(description map[string]any) {
	id, _ := description["WIRApplicationIdentifierKey"].(string)
	if id == "" {
		return
	}
	bundleID, _ := description["WIRApplicationBundleIdentifierKey"].(string)
	name, _ := description["WIRApplicationNameKey"].(string)
	session.apps[id] = inspectorApp{id: id, bundleID: bundleID, name: name}
}

// webPagesOf returns the web pages among what an app lists, ordered by id.
func webPagesOf(listing map[string]any) []inspectorPage {
	pages := []inspectorPage{}
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
	return pages
}

// findSafari waits for the device to report Safari among its inspectable apps.
// The device lists the apps it knows on connect, and announces others after.
func (session *webInspectorSession) findSafari(timeout time.Duration) (inspectorApp, error) {
	var safari inspectorApp
	err := session.await(time.Now().Add(timeout), func() bool {
		for _, app := range session.apps {
			if app.bundleID == safariBundleID {
				safari = app
				return true
			}
		}
		return false
	})
	if errors.Is(err, errInspectorTimedOut) {
		return inspectorApp{}, errors.New("safari is not inspectable: open it, and on a real device turn on Settings > Apps > Safari > Advanced > Web Inspector")
	}
	return safari, err
}

// webPages asks an app for what it has open, and returns the web pages among it.
func (session *webInspectorSession) webPages(app inspectorApp, timeout time.Duration) ([]inspectorPage, error) {
	session.mu.Lock()
	lastRevision := session.listings[app.id].revision
	session.mu.Unlock()

	if err := session.send("_rpc_forwardGetListing:", map[string]any{"WIRApplicationIdentifierKey": app.id}); err != nil {
		return nil, err
	}
	var pages []inspectorPage
	err := session.await(time.Now().Add(timeout), func() bool {
		listing := session.listings[app.id]
		pages = listing.pages
		return listing.revision > lastRevision
	})
	if err != nil {
		return nil, fmt.Errorf("list the tabs of safari: %w", err)
	}
	return pages, nil
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
			TargetID      string `json:"targetId"`
			Type          string `json:"type"`
			IsProvisional bool   `json:"isProvisional"`
		} `json:"targetInfo"`
		OldTargetID string `json:"oldTargetId"`
		NewTargetID string `json:"newTargetId"`
		// set on Target.dispatchMessageFromTarget: a protocol message of the target
		Message string `json:"message"`
	} `json:"params"`
}

// inspectorTab is the attachment of a session to one tab.
type inspectorTab struct {
	session   *webInspectorSession
	app       inspectorApp
	webviewID string
	pageID    uint64
	senderID  string

	// the fields below are guarded by session.mu

	// targetID names the page inside the tab: commands are not sent to the tab
	// itself but wrapped in a message to this target. A navigation can replace it.
	targetID      string
	lastCommandID int
	replies       map[int]inspectorProtocolMessage
}

// handle takes in a protocol message the tab sent. Called with session.mu held.
func (tab *inspectorTab) handle(data []byte) {
	var message inspectorProtocolMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return
	}

	switch message.Method {
	case "Target.targetCreated":
		if message.Params.TargetInfo.Type == "page" && !message.Params.TargetInfo.IsProvisional {
			tab.targetID = message.Params.TargetInfo.TargetID
		}
	case "Target.didCommitProvisionalTarget":
		if message.Params.OldTargetID == tab.targetID {
			tab.targetID = message.Params.NewTargetID
		}
	case "Target.dispatchMessageFromTarget":
		var reply inspectorProtocolMessage
		if err := json.Unmarshal([]byte(message.Params.Message), &reply); err == nil && reply.ID != 0 {
			tab.replies[reply.ID] = reply
		}
	case "":
		// the envelope a command travels in is only answered with an error when
		// it could not be delivered. It was numbered right after its command.
		if message.Error != nil {
			tab.replies[message.ID-1] = message
		}
	}
}

func (tab *inspectorTab) addressed(argument map[string]any) map[string]any {
	argument["WIRApplicationIdentifierKey"] = tab.app.id
	argument["WIRPageIdentifierKey"] = tab.pageID
	argument["WIRSenderKey"] = tab.senderID
	return argument
}

// detach stops the session from inspecting the tab.
func (tab *inspectorTab) detach() {
	_ = tab.session.send("_rpc_forwardDidClose:", tab.addressed(map[string]any{}))
	tab.session.mu.Lock()
	defer tab.session.mu.Unlock()
	delete(tab.session.tabs, tab.senderID)
}

// command sends one protocol command to the page of the tab and returns its result.
func (tab *inspectorTab) command(method string, params map[string]any, timeout time.Duration) (json.RawMessage, error) {
	tab.session.mu.Lock()
	tab.lastCommandID += 2
	commandID, envelopeID := tab.lastCommandID-1, tab.lastCommandID
	targetID := tab.targetID
	tab.session.mu.Unlock()

	command, err := json.Marshal(map[string]any{"id": commandID, "method": method, "params": params})
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", method, err)
	}
	envelope, err := json.Marshal(map[string]any{
		"id":     envelopeID,
		"method": "Target.sendMessageToTarget",
		"params": map[string]any{"targetId": targetID, "message": string(command)},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", method, err)
	}
	if err := tab.session.send("_rpc_forwardSocketData:", tab.addressed(map[string]any{"WIRSocketDataKey": envelope})); err != nil {
		return nil, err
	}

	var reply inspectorProtocolMessage
	err = tab.session.await(time.Now().Add(timeout), func() bool {
		answer, answered := tab.replies[commandID]
		if answered {
			reply = answer
			delete(tab.replies, commandID)
		}
		return answered
	})
	if errors.Is(err, errInspectorTimedOut) {
		return nil, fmt.Errorf("webview %s did not answer %s within %s", tab.webviewID, method, timeout)
	}
	if err != nil {
		return nil, err
	}
	if reply.Error != nil {
		return nil, fmt.Errorf("%s: %s", method, reply.Error.Message)
	}
	return reply.Result, nil
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

// evaluateOnly runs javascript in the page, without waiting for a promise it
// returns, and hands back what it returned.
func (tab *inspectorTab) evaluateOnly(expression string, timeout time.Duration) (inspectorRemoteObject, error) {
	raw, err := tab.command("Runtime.evaluate", map[string]any{"expression": expression}, timeout)
	if err != nil {
		return inspectorRemoteObject{}, err
	}
	return thrownOrReturned(raw)
}

// safariSignOfLife is evaluated to find out whether a tab answers at all.
const safariSignOfLife = "1"

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

// safariWebViews drives the tabs of Safari as webviews, through the web
// inspector of the device.
type safariWebViews struct {
	inspector *webInspectorClient
	// zero means the default; tests shorten them
	findSafariTimeout time.Duration
	livenessTimeout   time.Duration
}

func (s safariWebViews) findTimeout() time.Duration {
	if s.findSafariTimeout > 0 {
		return s.findSafariTimeout
	}
	return safariFindTimeout
}

func (s safariWebViews) liveness() time.Duration {
	if s.livenessTimeout > 0 {
		return s.livenessTimeout
	}
	return safariLivenessTimeout
}

// safariTabs returns Safari and the tabs it has open.
func (s safariWebViews) safariTabs() (*webInspectorSession, inspectorApp, []inspectorPage, error) {
	session, err := s.inspector.connected()
	if err != nil {
		return nil, inspectorApp{}, nil, err
	}
	safari, err := session.findSafari(s.findTimeout())
	if err != nil {
		return nil, inspectorApp{}, nil, err
	}
	pages, err := session.webPages(safari, safariCommandTimeout)
	if err != nil {
		return nil, inspectorApp{}, nil, err
	}
	return session, safari, pages, nil
}

// attach attaches to the tab with the given webview id, and makes sure it
// answers within livenessTimeout. The caller detaches the returned tab.
func (s safariWebViews) attach(webviewID string, livenessTimeout time.Duration) (*inspectorTab, error) {
	session, safari, pages, err := s.safariTabs()
	if err != nil {
		return nil, err
	}
	pageID, err := strconv.ParseUint(webviewID, 10, 64)
	if err != nil || !hasInspectorPage(pages, pageID) {
		return nil, fmt.Errorf("%w: %s", errSafariTabNotFound, webviewID)
	}
	return session.attach(safari, webviewID, pageID, livenessTimeout)
}

func hasInspectorPage(pages []inspectorPage, id uint64) bool {
	for _, page := range pages {
		if page.id == id {
			return true
		}
	}
	return false
}

func (session *webInspectorSession) attach(app inspectorApp, webviewID string, pageID uint64, livenessTimeout time.Duration) (*inspectorTab, error) {
	senderID, err := newInspectorID()
	if err != nil {
		return nil, err
	}
	tab := &inspectorTab{
		session:   session,
		app:       app,
		webviewID: webviewID,
		pageID:    pageID,
		senderID:  senderID,
		replies:   map[int]inspectorProtocolMessage{},
	}
	session.mu.Lock()
	session.tabs[senderID] = tab
	session.mu.Unlock()

	notRunning := fmt.Errorf("webview %s did not answer within %s; it is probably a background tab, which safari does not keep running", webviewID, livenessTimeout.Round(time.Millisecond))

	if err := session.send("_rpc_forwardSocketSetup:", tab.addressed(map[string]any{"WIRAutomaticallyPause": false})); err != nil {
		tab.detach()
		return nil, err
	}
	// the tab introduces its targets once attached; the page is the one to talk to
	deadline := time.Now().Add(livenessTimeout)
	err = session.await(deadline, func() bool { return tab.targetID != "" })
	if err == nil {
		_, err = tab.command("Runtime.evaluate", map[string]any{"expression": safariSignOfLife}, time.Until(deadline))
	}
	if err != nil {
		tab.detach()
		if errors.Is(err, errInspectorClosed) {
			return nil, err
		}
		return nil, notRunning
	}
	return tab, nil
}

func (s safariWebViews) ListWebViews() ([]WebViewInfo, error) {
	session, safari, pages, err := s.safariTabs()
	if err != nil {
		return nil, err
	}

	webviews := make([]WebViewInfo, len(pages))
	// the listing does not say which tab is in front, so ask each one
	var wg sync.WaitGroup
	for i, page := range pages {
		webviews[i] = WebViewInfo{
			ID:          strconv.FormatUint(page.id, 10),
			URL:         page.url,
			Title:       page.title,
			BundleID:    safariBundleID,
			ProcessName: safari.name,
		}
		wg.Add(1)
		go func(webview *WebViewInfo, pageID uint64) {
			defer wg.Done()
			webview.IsVisible = session.isVisible(safari, webview.ID, pageID)
		}(&webviews[i], page.id)
	}
	wg.Wait()

	return webviews, nil
}

// isVisible asks a tab whether it is the one in front. A tab that does not
// answer is not running, so it is not the one in front either.
func (session *webInspectorSession) isVisible(app inspectorApp, webviewID string, pageID uint64) bool {
	tab, err := session.attach(app, webviewID, pageID, safariVisibilityTimeout)
	if err != nil {
		return false
	}
	defer tab.detach()

	visible, err := tab.evaluate("return document.visibilityState === 'visible'", nil, safariVisibilityTimeout)
	return err == nil && visible == true
}

// navigate runs a function body that makes the tab leave its page. The page
// that answers is torn down by the navigation, so nothing is awaited from it.
func (s safariWebViews) navigate(webviewID, body string, args []any) error {
	tab, err := s.attach(webviewID, s.liveness())
	if err != nil {
		return err
	}
	defer tab.detach()

	expression, err := functionCall(body, args)
	if err != nil {
		return err
	}
	_, err = tab.evaluateOnly(expression, safariCommandTimeout)
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
	tab, err := s.attach(webviewID, s.liveness())
	if err != nil {
		return err
	}
	defer tab.detach()

	_, err = tab.command("Page.reload", map[string]any{}, safariCommandTimeout)
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
	tab, err := s.attach(webviewID, s.liveness())
	if err != nil {
		return nil, err
	}
	defer tab.detach()

	return tab.evaluate(expression, args, safariCommandTimeout)
}

// evaluate runs expression the way the injected agent does: as the body of a
// function called with args, whose returned promise is awaited.
func (tab *inspectorTab) evaluate(expression string, args []any, timeout time.Duration) (any, error) {
	call, err := functionCall(ensureReturnExpression(expression), args)
	if err != nil {
		return nil, err
	}
	// WebKit cannot await while evaluating. Whatever the function returns is
	// made a promise, which is handed back by reference and awaited separately.
	promise, err := tab.evaluateOnly("Promise.resolve("+call+")", timeout)
	if err != nil {
		return nil, err
	}
	raw, err := tab.command("Runtime.awaitPromise", map[string]any{
		"promiseObjectId": promise.Result.ObjectID,
		"returnByValue":   true,
	}, timeout)
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
		// a poll on a tab that does not answer must not outlive the wait
		pollTimeout := min(time.Until(deadline), s.liveness())
		if pollTimeout <= 0 {
			return fmt.Errorf("waitForLoadState timed out waiting for '%s'", state)
		}
		// the tab does not answer while it swaps pages mid-navigation, so only
		// an unknown tab ends the wait early
		loaded, err := s.hasLoaded(webviewID, hasLoaded, pollTimeout)
		if errors.Is(err, errSafariTabNotFound) {
			return err
		}
		if err == nil && loaded {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("waitForLoadState timed out waiting for '%s': %w", state, err)
			}
			return fmt.Errorf("waitForLoadState timed out waiting for '%s'", state)
		}
		time.Sleep(safariLoadStatePollEvery)
	}
}

func (s safariWebViews) hasLoaded(webviewID, hasLoaded string, timeout time.Duration) (bool, error) {
	tab, err := s.attach(webviewID, timeout)
	if err != nil {
		return false, err
	}
	defer tab.detach()

	loaded, err := tab.evaluate(hasLoaded, nil, timeout)
	return loaded == true, err
}
