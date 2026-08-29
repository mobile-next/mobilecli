package devices

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mobile-next/mobilecli/types"
	"github.com/mobile-next/mobilecli/utils"
)

// Flutter renders its whole UI into a single opaque native view, so uiautomator
// and the accessibility tree only ever see what Flutter publishes to the a11y
// layer — merged, typeless, and missing anything without semantics (e.g. a
// CustomPaint chart). For a debuggable Flutter app we instead read the live
// render tree from the Dart VM service: the JVMTI agent hands us the service
// URI (with its auth token) via FlutterJNI.getVMServiceUri(), and here we walk
// the render objects over the VM-service WebSocket using `invoke` — a reflective
// method call that, unlike `evaluate`, needs no Dart expression compiler and so
// works on a normally-launched app (no `flutter run`). Each render object's
// global position comes from invoking RenderBox.localToGlobal(Offset.zero).

const flutterVMCallTimeout = 15 * time.Second

// vmServiceURIPattern extracts port and token from http://127.0.0.1:PORT/TOKEN/
var vmServiceURIPattern = regexp.MustCompile(`^http://127\.0\.0\.1:(\d+)/([^/]*)/?$`)

// flutterVMServiceURI attaches the agent to pkg and asks it for the running
// Flutter app's Dart VM service URI. An empty string (no error) or an error
// both mean "treat as not-Flutter and fall back to the accessibility dump".
func (d *AndroidDevice) flutterVMServiceURI(pkg string) string {
	port, err := d.ensureAgentReady(pkg)
	if err != nil {
		utils.Verbose("flutter: agent not ready for %s: %v", pkg, err)
		return ""
	}
	raw, err := agentRequest(port, "device.flutter.vmServiceUri", nil)
	if err != nil {
		// "not a flutter app" is the expected negative — the FlutterJNI class is
		// absent from a non-Flutter app's classloader.
		utils.Verbose("flutter: vmServiceUri for %s: %v", pkg, err)
		return ""
	}
	var r struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return ""
	}
	return r.URI
}

// tryDumpFlutterSource returns the Flutter render tree for the foreground app,
// or ok=false to signal the caller should use the accessibility dump. Detection
// requires a debuggable app (the JVMTI agent only attaches to those), which is
// also the only case where a Dart VM service exists.
func (d *AndroidDevice) tryDumpFlutterSource() ([]types.ScreenElement, bool) {
	foreground, err := d.GetForegroundApp()
	if err != nil {
		return nil, false
	}
	pkg := foreground.PackageName
	if !d.isAppDebuggable(pkg) {
		return nil, false
	}
	uri := d.flutterVMServiceURI(pkg)
	if uri == "" {
		return nil, false
	}
	start := time.Now()
	elements, err := d.dumpFlutterSource(uri)
	if err != nil {
		utils.Verbose("flutter: render-tree dump failed, falling back: %v", err)
		return nil, false
	}
	utils.Verbose("flutter: render-tree dump produced %d elements in %s", len(elements), time.Since(start))
	return elements, true
}

// dumpFlutterSource reads the Flutter render tree over the VM service and
// converts it to ScreenElements. uri is the on-device service URI; we forward a
// host port to its device port and speak the VM-service protocol over WebSocket.
func (d *AndroidDevice) dumpFlutterSource(uri string) ([]types.ScreenElement, error) {
	m := vmServiceURIPattern.FindStringSubmatch(strings.TrimSpace(uri))
	if m == nil {
		return nil, fmt.Errorf("unexpected Dart VM service URI: %q", uri)
	}
	devicePort, token := m[1], m[2]

	out, err := d.runAdbCommand("forward", "tcp:0", "tcp:"+devicePort)
	if err != nil {
		return nil, fmt.Errorf("adb forward to VM service: %s: %w", strings.TrimSpace(string(out)), err)
	}
	localPort, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return nil, fmt.Errorf("unexpected adb forward output %q: %w", strings.TrimSpace(string(out)), err)
	}
	defer d.runAdbCommand("forward", "--remove", fmt.Sprintf("tcp:%d", localPort))

	// The device port is now reachable at the forwarded local port.
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/%s/ws", localPort, token)
	return dumpFlutterTreeOverWS(wsURL, d.devicePixelRatio())
}

// devicePixelRatio maps Flutter's logical pixels to the physical pixels that
// uiautomator (and taps) use. `wm density` reports dpi; dpr = dpi / 160.
func (d *AndroidDevice) devicePixelRatio() float64 {
	out, err := d.runAdbCommand("shell", "wm", "density")
	if err == nil {
		if m := regexp.MustCompile(`(\d+)`).FindStringSubmatch(string(out)); m != nil {
			if dpi, err := strconv.Atoi(m[1]); err == nil && dpi > 0 {
				return float64(dpi) / 160.0
			}
		}
	}
	return 1.0
}

// ---------------------------------------------------------------------------
// VM-service WebSocket client (JSON-RPC 2.0, concurrent request/response)
// ---------------------------------------------------------------------------

type flutterVM struct {
	conn         *websocket.Conn
	writeMu      sync.Mutex
	mu           sync.Mutex
	seq          int
	pending      map[int]chan vmResp
	isolateID    string
	dpr          float64
	zeroOffsetID string        // objectId of Offset.zero, the localToGlobal argument
	callSem      chan struct{} // bounds concurrent in-flight VM-service calls
}

// maxConcurrentVMCalls caps in-flight VM-service requests. The walk fans out one
// goroutine per render child (cheap), but the underlying calls are throttled
// here so we pipeline aggressively without flooding the VM service. Structural
// goroutines wait on their children *after* their own calls return, so they
// never hold a call slot across a wait — no deadlock regardless of tree shape.
const maxConcurrentVMCalls = 48

type vmResp struct {
	result json.RawMessage
	err    error
}

func dialFlutterVM(wsURL string) (*flutterVM, error) {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to Dart VM service: %w", err)
	}
	vm := &flutterVM{
		conn:    conn,
		pending: make(map[int]chan vmResp),
		callSem: make(chan struct{}, maxConcurrentVMCalls),
	}
	go vm.readLoop()
	return vm, nil
}

func (vm *flutterVM) close() {
	vm.conn.Close()
}

func (vm *flutterVM) readLoop() {
	for {
		_, data, err := vm.conn.ReadMessage()
		if err != nil {
			vm.failAll(err)
			return
		}
		var msg struct {
			ID     *int            `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &msg); err != nil || msg.ID == nil {
			continue
		}
		vm.mu.Lock()
		ch := vm.pending[*msg.ID]
		delete(vm.pending, *msg.ID)
		vm.mu.Unlock()
		if ch == nil {
			continue
		}
		if msg.Error != nil {
			ch <- vmResp{err: fmt.Errorf("vm error %d: %s", msg.Error.Code, msg.Error.Message)}
		} else {
			ch <- vmResp{result: msg.Result}
		}
	}
}

func (vm *flutterVM) failAll(err error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	for id, ch := range vm.pending {
		ch <- vmResp{err: err}
		delete(vm.pending, id)
	}
}

func (vm *flutterVM) call(method string, params map[string]any) (json.RawMessage, error) {
	vm.callSem <- struct{}{}
	defer func() { <-vm.callSem }()

	vm.mu.Lock()
	vm.seq++
	id := vm.seq
	ch := make(chan vmResp, 1)
	vm.pending[id] = ch
	vm.mu.Unlock()

	req := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		req["params"] = params
	}
	payload, _ := json.Marshal(req)

	vm.writeMu.Lock()
	err := vm.conn.WriteMessage(websocket.TextMessage, payload)
	vm.writeMu.Unlock()
	if err != nil {
		vm.mu.Lock()
		delete(vm.pending, id)
		vm.mu.Unlock()
		return nil, err
	}

	select {
	case r := <-ch:
		return r.result, r.err
	case <-time.After(flutterVMCallTimeout):
		vm.mu.Lock()
		delete(vm.pending, id)
		vm.mu.Unlock()
		return nil, fmt.Errorf("vm service call %q timed out", method)
	}
}

// vmInstanceRef / vmInstance model the subset of the VM-service Instance shape
// we read: an object id, its class name, a scalar string value, and its fields.
type vmInstanceRef struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Name       string `json:"name"` // populated for Class objects (the class name)
	ValueAsStr string `json:"valueAsString"`
	ClassRef   *struct {
		Name string `json:"name"`
	} `json:"class"`
}

func (r *vmInstanceRef) className() string {
	if r != nil && r.ClassRef != nil {
		return r.ClassRef.Name
	}
	return ""
}

type vmInstance struct {
	vmInstanceRef
	Fields []struct {
		Decl *struct {
			Name string `json:"name"`
		} `json:"decl"`
		Name  string         `json:"name"`
		Value *vmInstanceRef `json:"value"`
	} `json:"fields"`
	Elements []vmInstanceRef `json:"elements"` // populated for List objects
}

func (o *vmInstance) field(name string) *vmInstanceRef {
	for _, f := range o.Fields {
		fn := f.Name
		if f.Decl != nil {
			fn = f.Decl.Name
		}
		if fn == name {
			return f.Value
		}
	}
	return nil
}

func (vm *flutterVM) getObject(objectID string) (*vmInstance, error) {
	raw, err := vm.call("getObject", map[string]any{"isolateId": vm.isolateID, "objectId": objectID})
	if err != nil {
		return nil, err
	}
	var o vmInstance
	if err := json.Unmarshal(raw, &o); err != nil {
		return nil, err
	}
	return &o, nil
}

// invoke calls a method reflectively on a live object (no expression compiler).
func (vm *flutterVM) invoke(target, selector string, args []string) (*vmInstanceRef, error) {
	if args == nil {
		args = []string{}
	}
	raw, err := vm.call("invoke", map[string]any{
		"isolateId":   vm.isolateID,
		"targetId":    target,
		"selector":    selector,
		"argumentIds": args,
	})
	if err != nil {
		return nil, err
	}
	var r vmInstanceRef
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (vm *flutterVM) resolveIsolate() error {
	raw, err := vm.call("getVM", nil)
	if err != nil {
		return err
	}
	var v struct {
		Isolates []struct {
			ID string `json:"id"`
		} `json:"isolates"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	if len(v.Isolates) == 0 {
		return fmt.Errorf("no Dart isolate available")
	}
	vm.isolateID = v.Isolates[0].ID
	return nil
}

// findClassIDs resolves class ids by exact name. getClassList already returns
// every class ref with its name inline, so this is a single round trip — no
// per-class getObject.
func (vm *flutterVM) findClassIDs(names map[string]string) error {
	raw, err := vm.call("getClassList", map[string]any{"isolateId": vm.isolateID})
	if err != nil {
		return err
	}
	var cl struct {
		Classes []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"classes"`
	}
	if err := json.Unmarshal(raw, &cl); err != nil {
		return err
	}
	for _, c := range cl.Classes {
		if _, want := names[c.Name]; want && names[c.Name] == "" {
			names[c.Name] = c.ID
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Render-tree walk
// ---------------------------------------------------------------------------

const (
	rootRenderClass = "_ReusableRenderView"
	offsetClass     = "Offset"
)

var offsetSizePattern = regexp.MustCompile(`\(([-\d.]+),\s*([-\d.]+)\)`)

func (vm *flutterVM) dumpRenderTree(dpr float64) ([]types.ScreenElement, error) {
	vm.dpr = dpr
	t0 := time.Now()
	classes := map[string]string{rootRenderClass: "", offsetClass: ""}
	if err := vm.findClassIDs(classes); err != nil {
		return nil, err
	}
	if classes[rootRenderClass] == "" {
		return nil, fmt.Errorf("could not locate the Flutter root render object (%s)", rootRenderClass)
	}

	rootID, err := vm.firstInstance(classes[rootRenderClass])
	if err != nil {
		return nil, fmt.Errorf("no root render object instance: %w", err)
	}
	zeroID, err := vm.offsetZeroID(classes[offsetClass])
	if err != nil {
		return nil, fmt.Errorf("could not obtain Offset.zero: %w", err)
	}
	vm.zeroOffsetID = zeroID
	utils.Verbose("flutter: bootstrap (classes+root+offset.zero) took %s", time.Since(t0))

	t1 := time.Now()
	els, err := vm.visit(rootID, rootRenderClass, nil)
	utils.Verbose("flutter: tree walk took %s", time.Since(t1))
	return els, err
}

// renderChild is a child render object plus its class name (both come from the
// parent's debugDescribeChildren, so no extra getObject is needed per node).
type renderChild struct {
	id    string
	class string
}

// flutterSemantics is the accessibility metadata read from a SemanticsNode. It
// is threaded down the render walk from the nearest enclosing boundary and
// attached to the content leaves it covers.
type flutterSemantics struct {
	label        string
	value        string
	identifier   string
	hint         string
	checked      *bool
	selected     *bool
	focused      *bool
	enabled      *bool
	isButton     bool
	isTextField  bool
	inRadioGroup bool // isInMutuallyExclusiveGroup — a radio, not a checkbox
	// mergesDescendants is set when this boundary folds its whole subtree into one
	// accessible node (e.g. a CheckboxListTile). Such a node is a single control,
	// so it is emitted once and its content leaves are not emitted separately.
	mergesDescendants bool
}

// hasContent reports whether the semantics carry anything worth attaching.
func (s *flutterSemantics) hasContent() bool {
	return s != nil && (s.label != "" || s.identifier != "" || s.value != "" ||
		s.hint != "" || s.isButton || s.isTextField ||
		s.checked != nil || s.selected != nil || s.focused != nil || s.enabled != nil)
}

func (vm *flutterVM) firstInstance(classID string) (string, error) {
	raw, err := vm.call("getInstances", map[string]any{"isolateId": vm.isolateID, "objectId": classID, "limit": 1})
	if err != nil {
		return "", err
	}
	var r struct {
		Instances []struct {
			ID string `json:"id"`
		} `json:"instances"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", err
	}
	if len(r.Instances) == 0 {
		return "", fmt.Errorf("class %s has no live instances", classID)
	}
	return r.Instances[0].ID, nil
}

// offsetZeroID finds the const Offset.zero on the heap (fields _dx/_dy == 0.0),
// used as the point argument to RenderBox.localToGlobal.
func (vm *flutterVM) offsetZeroID(offsetClassID string) (string, error) {
	raw, err := vm.call("getInstances", map[string]any{"isolateId": vm.isolateID, "objectId": offsetClassID, "limit": 500})
	if err != nil {
		return "", err
	}
	var r struct {
		Instances []struct {
			ID string `json:"id"`
		} `json:"instances"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", err
	}
	// Inspect the instances concurrently — call() throttles the fan-out.
	found := make([]string, len(r.Instances))
	var wg sync.WaitGroup
	for i, inst := range r.Instances {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			o, err := vm.getObject(id)
			if err != nil {
				return
			}
			dx, dy := o.field("_dx"), o.field("_dy")
			if dx != nil && dy != nil && dx.ValueAsStr == "0.0" && dy.ValueAsStr == "0.0" {
				found[i] = id
			}
		}(i, inst.ID)
	}
	wg.Wait()
	for _, id := range found {
		if id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("Offset.zero not found among live instances")
}

// visit walks a render object subtree, returning the meaningful ScreenElements
// it contains. Pure-wrapper render objects (those with render children) are
// hoisted — only leaf render objects with a non-zero size are emitted, which is
// exactly the visible content (paragraphs, images, custom-painted widgets like
// charts), each with its real Flutter type and global bounds.
func (vm *flutterVM) visit(nodeID, className string, sem *flutterSemantics) ([]types.ScreenElement, error) {
	// A render object that forms a semantics boundary carries the label/flags on
	// its own debugSemantics (only the owner does — ancestors and merged
	// descendants read empty), regardless of the boundary class
	// (RenderSemanticsAnnotations, RenderIndexedSemantics, RenderMergeSemantics…).
	// Adopt it for this subtree so the content leaves beneath (where Flutter has
	// merged the label away) inherit it.
	if s := vm.readNodeSemantics(nodeID); s.hasContent() {
		sem = s
		// A merging boundary is one accessible control (label + its widgets folded
		// together), so emit it once here and don't descend — otherwise each of its
		// content leaves (e.g. a checkbox's label row and its box) emits separately.
		if s.mergesDescendants {
			return vm.emitLeaf(nodeID, className, sem), nil
		}
	}

	kids := vm.childrenOf(nodeID)

	// Wrapper node (has render children): visit children concurrently and hoist
	// their results to our level, preserving order.
	if len(kids) > 0 {
		results := make([][]types.ScreenElement, len(kids))
		var wg sync.WaitGroup
		for i, k := range kids {
			wg.Add(1)
			go func(i int, k renderChild) {
				defer wg.Done()
				results[i], _ = vm.visit(k.id, k.class, sem)
			}(i, k)
		}
		wg.Wait()
		var children []types.ScreenElement
		for _, r := range results {
			children = append(children, r...)
		}
		return children, nil
	}

	return vm.emitLeaf(nodeID, className, sem), nil
}

// emitLeaf builds the ScreenElement for a single node (a render leaf, or a
// merging semantics boundary), or returns nil if it has no on-screen size.
func (vm *flutterVM) emitLeaf(nodeID, className string, sem *flutterSemantics) []types.ScreenElement {
	rect, ok := vm.globalRect(nodeID)
	if !ok || rect.Width <= 0 || rect.Height <= 0 {
		return nil
	}
	el := types.ScreenElement{
		Type: friendlyRenderType(className),
		Rect: rect,
	}
	if text := vm.extractText(nodeID, className); text != "" {
		el.Text = &text
	}
	applySemantics(&el, sem)
	return []types.ScreenElement{el}
}

// applySemantics attaches the nearest enclosing semantics to a leaf element:
// label/value/identifier/placeholder and the checked/selected/focused/enabled
// booleans, and refines the type for buttons and text fields.
func applySemantics(el *types.ScreenElement, sem *flutterSemantics) {
	if !sem.hasContent() {
		return
	}
	if sem.label != "" {
		label := sem.label
		el.Label = &label
	}
	if sem.value != "" {
		value := sem.value
		el.Value = &value
	}
	if sem.identifier != "" {
		identifier := sem.identifier
		el.Identifier = &identifier
	}
	if sem.hint != "" {
		hint := sem.hint
		el.Placeholder = &hint
	}
	// A node with a checked-state is a checkbox/radio: always emit its state
	// (true or false) so it is unambiguous. selected/focused stay true-only and
	// enabled false-only, per the accessibility-dump conventions.
	if sem.checked != nil {
		c := *sem.checked
		el.Checked = &c
	}
	if sem.selected != nil && *sem.selected {
		t := true
		el.Selected = &t
	}
	if sem.focused != nil && *sem.focused {
		t := true
		el.Focused = &t
	}
	if sem.enabled != nil && !*sem.enabled {
		f := false
		el.Enabled = &f
	}
	// Type refinement. sem.checked != nil means the node has a checked-state (a
	// checkbox/radio) even when unchecked, so it stays identifiable as a control
	// rather than looking like plain text.
	switch {
	case sem.isTextField:
		el.Type = "TextField"
	case sem.isButton:
		el.Type = "Button"
	case sem.checked != nil && sem.inRadioGroup:
		el.Type = "Radio"
	case sem.checked != nil:
		el.Type = "Checkbox"
	}
}

// readNodeSemantics reads the accessibility metadata attached to a render
// object via its debugSemantics SemanticsNode (label/value/identifier/hint and
// flags). Returns an empty (no-content) struct when the node has no semantics.
func (vm *flutterVM) readNodeSemantics(renderNodeID string) *flutterSemantics {
	sem := &flutterSemantics{}
	sn, err := vm.invoke(renderNodeID, "get:debugSemantics", nil)
	if err != nil || sn == nil || sn.ID == "" || sn.Kind == "Null" {
		return sem
	}
	sem.label = vm.invokeString(sn.ID, "get:label")
	sem.value = vm.invokeString(sn.ID, "get:value")
	sem.identifier = vm.invokeString(sn.ID, "get:identifier")
	sem.mergesDescendants = vm.invokeString(sn.ID, "get:mergeAllDescendantsIntoThisNode") == "true"

	data, err := vm.invoke(sn.ID, "getSemanticsData", nil)
	if err != nil || data == nil || data.ID == "" {
		return sem
	}
	obj, err := vm.getObject(data.ID)
	if err != nil {
		return sem
	}
	if ah := obj.field("attributedHint"); ah != nil && ah.ID != "" {
		sem.hint = vm.invokeString(ah.ID, "get:string")
	}
	if fc := obj.field("flagsCollection"); fc != nil && fc.ID != "" {
		if fco, err := vm.getObject(fc.ID); err == nil {
			sem.isButton = fieldIsTrue(fco, "isButton")
			sem.isTextField = fieldIsTrue(fco, "isTextField")
			sem.inRadioGroup = fieldIsTrue(fco, "isInMutuallyExclusiveGroup")
			sem.checked = vm.triState(fco.field("isChecked"))
			sem.selected = vm.triState(fco.field("isSelected"))
			sem.focused = vm.triState(fco.field("isFocused"))
			sem.enabled = vm.triState(fco.field("isEnabled"))
		}
	}
	return sem
}

// invokeString invokes a no-arg selector expected to return a String and returns
// its value (empty on error or non-string).
func (vm *flutterVM) invokeString(target, selector string) string {
	r, err := vm.invoke(target, selector, nil)
	if err != nil || r == nil {
		return ""
	}
	return r.ValueAsStr
}

func fieldIsTrue(o *vmInstance, name string) bool {
	f := o.field(name)
	return f != nil && f.ValueAsStr == "true"
}

// triState resolves a SemanticsFlags tri-state member to a *bool. In this Flutter
// version these are enums (Tristate{true,false,none}, CheckedState{checked,
// unchecked,mixed}); older versions expose plain bools. none/mixed → nil.
func (vm *flutterVM) triState(ref *vmInstanceRef) *bool {
	if ref == nil || ref.ID == "" || ref.Kind == "Null" {
		return nil
	}
	if ref.ValueAsStr == "true" {
		t := true
		return &t
	}
	if ref.ValueAsStr == "false" {
		f := false
		return &f
	}
	// Tristate.{isTrue,isFalse,none} and CheckedState.{isTrue,isFalse,mixed};
	// none/mixed fall through to nil.
	switch s := vm.invokeString(ref.ID, "toString"); {
	case strings.HasSuffix(s, ".isTrue"):
		t := true
		return &t
	case strings.HasSuffix(s, ".isFalse"):
		f := false
		return &f
	}
	return nil
}

// childrenOf returns a render object's render children type-agnostically via
// RenderObject.debugDescribeChildren() — every RenderObject implements it,
// regardless of how it stores children (single-child, container, or sliver).
// Each returned DiagnosticsNode's `value` is the child render object.
func (vm *flutterVM) childrenOf(nodeID string) []renderChild {
	list, err := vm.invoke(nodeID, "debugDescribeChildren", nil)
	if err != nil || list.ID == "" {
		return nil
	}
	obj, err := vm.getObject(list.ID)
	if err != nil {
		return nil
	}
	var kids []renderChild
	for _, node := range obj.Elements {
		if node.ID == "" {
			continue
		}
		val, err := vm.invoke(node.ID, "get:value", nil)
		if err != nil || val.ID == "" || val.Kind == "Null" {
			continue
		}
		// debugDescribeChildren only lists children, but guard anyway.
		if !strings.Contains(val.className(), "Render") {
			continue
		}
		kids = append(kids, renderChild{id: val.ID, class: val.className()})
	}
	return kids
}

// globalRect returns a render object's global bounds in physical pixels via
// RenderBox.localToGlobal(Offset.zero) and RenderBox.size.
func (vm *flutterVM) globalRect(nodeID string) (types.ScreenElementRect, bool) {
	off, err := vm.invoke(nodeID, "localToGlobal", []string{vm.zeroOffsetID})
	if err != nil || off.ID == "" {
		return types.ScreenElementRect{}, false
	}
	ox, oy, ok := vm.offsetPair(off.ID)
	if !ok {
		return types.ScreenElementRect{}, false
	}
	size, err := vm.invoke(nodeID, "get:size", nil)
	if err != nil || size.ID == "" {
		return types.ScreenElementRect{}, false
	}
	sw, sh, ok := vm.offsetPair(size.ID)
	if !ok {
		return types.ScreenElementRect{}, false
	}
	return types.ScreenElementRect{
		X:      int(ox*vm.dpr + 0.5),
		Y:      int(oy*vm.dpr + 0.5),
		Width:  int(sw*vm.dpr + 0.5),
		Height: int(sh*vm.dpr + 0.5),
	}, true
}

// offsetPair reads the two doubles of an Offset or Size (both store _dx/_dy).
func (vm *flutterVM) offsetPair(objectID string) (float64, float64, bool) {
	o, err := vm.getObject(objectID)
	if err != nil {
		return 0, 0, false
	}
	dx, dy := o.field("_dx"), o.field("_dy")
	if dx == nil || dy == nil {
		return 0, 0, false
	}
	x, err1 := strconv.ParseFloat(dx.ValueAsStr, 64)
	y, err2 := strconv.ParseFloat(dy.ValueAsStr, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return x, y, true
}

// extractText reads the plain text of a paragraph/editable render object.
func (vm *flutterVM) extractText(nodeID, className string) string {
	if !strings.Contains(className, "Paragraph") && !strings.Contains(className, "Editable") {
		return ""
	}
	span, err := vm.invoke(nodeID, "get:text", nil)
	if err != nil || span.ID == "" {
		return ""
	}
	plain, err := vm.invoke(span.ID, "toPlainText", nil)
	if err != nil {
		return ""
	}
	return plain.ValueAsStr
}

// friendlyRenderType turns a render class name into a widget-ish type, e.g.
// RenderParagraph -> Paragraph, _RenderColoredBox -> ColoredBox.
func friendlyRenderType(name string) string {
	name = strings.TrimPrefix(name, "_")
	name = strings.TrimPrefix(name, "Render")
	if name == "" {
		return "FlutterWidget"
	}
	return name
}
