package devices

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/mobile-next/mobilecli/types"
)

// A dropped VM-service connection must surface as an error rather than as an
// empty subtree, otherwise a half-walked tree gets reported as a complete dump
// and the accessibility fallback is skipped exactly when it is needed.
func TestVisitFailsOnceTheConnectionIsGone(t *testing.T) {
	vm := newVMWithDeadConnection()

	if _, err := vm.visit("someRenderObject", "RenderParagraph", nil, ""); err == nil {
		t.Error("expected visit to fail after the connection died, got nil error")
	}
}

// Calls made after the read loop exits can never be answered, so they must fail
// immediately instead of blocking for the full per-call timeout.
func TestCallFailsFastAfterTheConnectionIsGone(t *testing.T) {
	vm := newVMWithDeadConnection()

	// A nil websocket connection would panic if call() got as far as writing.
	if _, err := vm.call("getVM", nil); err == nil {
		t.Error("expected call to fail after the connection died, got nil error")
	}
}

func newVMWithDeadConnection() *flutterVM {
	vm := &flutterVM{
		pending: make(map[int]chan vmResp),
		callSem: make(chan struct{}, maxConcurrentVMCalls),
	}
	vm.failAll(errors.New("websocket: close 1006 (abnormal closure)"))
	return vm
}

func TestFriendlyRenderTypeStripsRenderAndUnderscorePrefixes(t *testing.T) {
	cases := map[string]string{
		"RenderParagraph":   "Text",
		"RenderCustomPaint": "CustomPaint",
		"RenderImage":       "Image",
		"_RenderColoredBox": "ColoredBox",
		"RenderEditable":    "Editable",
		"":                  "FlutterWidget",
		"Render":            "FlutterWidget",
	}
	for in, want := range cases {
		if got := friendlyRenderType(in); got != want {
			t.Errorf("friendlyRenderType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVMServiceURIPatternExtractsPortAndToken(t *testing.T) {
	m := vmServiceURIPattern.FindStringSubmatch("http://127.0.0.1:46809/fEuKPZ9fvrk=/")
	if m == nil {
		t.Fatal("expected the Dart VM service URI to match")
	}
	if m[1] != "46809" {
		t.Errorf("port = %q, want 46809", m[1])
	}
	if m[2] != "fEuKPZ9fvrk=" {
		t.Errorf("token = %q, want fEuKPZ9fvrk=", m[2])
	}
}

func TestVMServiceURIPatternRejectsNonLoopback(t *testing.T) {
	if vmServiceURIPattern.MatchString("http://10.0.0.5:46809/token/") {
		t.Error("expected a non-loopback VM service URI to be rejected")
	}
}

// a debug (JIT) build reports a double field as kind Double with its decimal
// text; a profile (AOT) build keeps the field unboxed and reports its raw
// IEEE-754 bits as kind Int. these are real values from an iPhone profile build.
func TestDoubleValueReadsBothJITAndAOTEncodings(t *testing.T) {
	cases := []struct {
		name string
		ref  vmInstanceRef
		want float64
	}{
		{"debug build double", vmInstanceRef{Kind: "Double", ValueAsStr: "207.0"}, 207.0},
		{"debug build zero", vmInstanceRef{Kind: "Double", ValueAsStr: "0.0"}, 0},
		{"profile build unboxed double", vmInstanceRef{Kind: "Int", ValueAsStr: "4641487181586628608"}, 207.0},
		{"profile build unboxed zero", vmInstanceRef{Kind: "Int", ValueAsStr: "0"}, 0},
		{"profile build negative", vmInstanceRef{Kind: "Int", ValueAsStr: "-4598175219545276416"}, -16.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := c.ref.doubleValue()
			if !ok || got != c.want {
				t.Fatalf("doubleValue() = %v, %v; want %v", got, ok, c.want)
			}
		})
	}

	if _, ok := (&vmInstanceRef{Kind: "String", ValueAsStr: "hello"}).doubleValue(); ok {
		t.Error("a non-numeric value must not be read as a double")
	}
}

func TestInnermostStringKeyReadsTheNearestKeyInTheCreatorChain(t *testing.T) {
	cases := map[string]string{
		"Semantics ← _ButtonStyleState ← ElevatedButton-[<'login-button'>] ← Column-[<'form'>] ← ⋯": "login-button",
		"RenderParagraph ← Text ← Padding ← ⋯":                                                      "",
		"Padding-[<42>] ← Row-[GlobalKey#1a2b3] ← ListTile-[<'row 1'>] ← ⋯":                         "row 1",
		"Text-[<'it'>]": "it",
		"Semantics ← Chip-[<'tags ← nested'>] ← Column-[<'form'>] ← ⋯": "tags ← nested",
	}
	for chain, want := range cases {
		if got := innermostStringKey(chain); got != want {
			t.Errorf("innermostStringKey(%q) = %q, want %q", chain, got, want)
		}
	}
}

// A large walk pushes the Offset.zero id out of the isolate's VM-service id
// ring, after which the VM rejects it as an invalid argument. globalRect must
// re-resolve Offset.zero and retry instead of silently dropping the node.
func TestGlobalRectReResolvesAnExpiredOffsetZero(t *testing.T) {
	expectGlobalRectToReResolveOffsetZero(t, func(args []any) (any, *vmRPCError) {
		return nil, &vmRPCError{Code: vmInvalidParams, Message: "Invalid params", Details: fmt.Sprintf("invoke: invalid 'argumentIds' parameter: %v", args)}
	})
}

// The VM service may also answer an expired argument id with the Expired
// sentinel as the result, instead of an error.
func TestGlobalRectReResolvesOffsetZeroOnAnExpiredSentinel(t *testing.T) {
	expectGlobalRectToReResolveOffsetZero(t, func(args []any) (any, *vmRPCError) {
		return map[string]any{"type": "Sentinel", "kind": "Expired", "valueAsString": "<expired>"}, nil
	})
}

// expectGlobalRectToReResolveOffsetZero runs globalRect against a fake VM that
// answers localToGlobal on the stale Offset.zero id with expired.
func expectGlobalRectToReResolveOffsetZero(t *testing.T, expired func(args []any) (any, *vmRPCError)) {
	t.Helper()
	var lookups atomic.Int32
	vm := dialFakeVM(t, func(method string, params map[string]any) (any, *vmRPCError) {
		switch method {
		case "invoke":
			switch params["selector"] {
			case "localToGlobal":
				if args, _ := params["argumentIds"].([]any); fmt.Sprint(args) != "[objects/zero-new]" {
					return expired(args)
				}
				return map[string]any{"id": "objects/global"}, nil
			case "get:size":
				return map[string]any{"id": "objects/size"}, nil
			}
		case "getInstances":
			lookups.Add(1)
			return map[string]any{"instances": []any{map[string]any{"id": "objects/zero-new"}}}, nil
		case "getObject":
			switch params["objectId"] {
			case "objects/zero-new":
				return fakeOffset("0.0", "0.0"), nil
			case "objects/global":
				return fakeOffset("10.0", "20.0"), nil
			case "objects/size":
				return fakeOffset("30.0", "40.0"), nil
			}
		}
		return nil, &vmRPCError{Code: -32000, Message: "unexpected call " + method}
	})
	vm.zeroOffsetID = "objects/zero-old"
	vm.dpr = 2

	rect, ok := vm.globalRect("objects/node")
	if !ok {
		t.Fatal("expected globalRect to succeed after re-resolving Offset.zero")
	}
	want := types.ScreenElementRect{X: 20, Y: 40, Width: 60, Height: 80}
	if rect != want {
		t.Errorf("rect = %+v, want %+v", rect, want)
	}
	if n := lookups.Load(); n != 1 {
		t.Errorf("Offset.zero was looked up %d times, want 1", n)
	}

	// Later nodes use the re-resolved id without another lookup.
	if _, ok := vm.globalRect("objects/other"); !ok {
		t.Fatal("expected globalRect to succeed with the re-resolved Offset.zero")
	}
	if n := lookups.Load(); n != 1 {
		t.Errorf("Offset.zero was looked up %d times, want 1", n)
	}
}

// The binding, its render view and Offset.zero all hang off statics, so finding
// them must not take the full class list or a scan of the heap: on a large app
// those two cost seconds on every dump, before the walk has even started.
func TestWalkRootsAreResolvedThroughStatics(t *testing.T) {
	var scans atomic.Int32
	vm := dialFakeVM(t, func(method string, params map[string]any) (any, *vmRPCError) {
		switch method {
		case "getIsolate":
			return fakeFlutterLibraries(), nil
		case "getObject":
			if lib := fakeFlutterLibrary(params["objectId"]); lib != nil {
				return lib, nil
			}
		case "invoke":
			if result := fakeFlutterStatics(params); result != nil {
				return result, nil
			}
		case "getClassList", "getInstances":
			scans.Add(1)
		}
		return nil, &vmRPCError{Code: -32000, Message: "unexpected call " + method}
	})

	roots, err := vm.resolveWalkRoots()
	if err != nil {
		t.Fatalf("resolveWalkRoots failed: %v", err)
	}
	want := walkRoots{
		rootID:        "objects/root",
		rootClass:     "_ReusableRenderView",
		bindingID:     "objects/binding",
		zeroID:        "objects/zero",
		offsetClassID: "classes/offset",
	}
	if *roots != want {
		t.Errorf("roots = %+v, want %+v", *roots, want)
	}
	if n := scans.Load(); n != 0 {
		t.Errorf("the class list or the heap was scanned %d times, want 0", n)
	}
}

// A VM that cannot be reached through statics still has to produce a dump, so
// the class list and the heap remain the fallback.
func TestWalkRootsFallBackToScanningTheHeap(t *testing.T) {
	vm := dialFakeVM(t, func(method string, params map[string]any) (any, *vmRPCError) {
		switch method {
		case "getIsolate":
			return fakeFlutterLibraries(), nil
		case "getObject":
			if lib := fakeFlutterLibrary(params["objectId"]); lib != nil {
				return lib, nil
			}
			if params["objectId"] == "objects/zero-on-heap" {
				return fakeOffset("0.0", "0.0"), nil
			}
		case "invoke":
			// a static getter that throws comes back as an @Error result
			return map[string]any{"type": "@Error", "kind": "UnhandledException", "id": "objects/error"}, nil
		case "getClassList":
			return map[string]any{"classes": []any{
				fakeClass("classes/root", rootRenderClass),
				fakeClass("classes/offset", offsetClass),
				fakeClass("classes/binding", bindingClass),
			}}, nil
		case "getInstances":
			instance := map[any]string{
				"classes/root":    "objects/root-on-heap",
				"classes/offset":  "objects/zero-on-heap",
				"classes/binding": "objects/binding-on-heap",
			}[params["objectId"]]
			return map[string]any{"instances": []any{map[string]any{"id": instance}}}, nil
		}
		return nil, &vmRPCError{Code: -32000, Message: "unexpected call " + method}
	})

	roots, err := vm.resolveWalkRoots()
	if err != nil {
		t.Fatalf("resolveWalkRoots failed: %v", err)
	}
	want := walkRoots{
		rootID:        "objects/root-on-heap",
		rootClass:     rootRenderClass,
		bindingID:     "objects/binding-on-heap",
		zeroID:        "objects/zero-on-heap",
		offsetClassID: "classes/offset",
	}
	if *roots != want {
		t.Errorf("roots = %+v, want %+v", *roots, want)
	}
}

// Re-resolving an expired Offset.zero happens in the middle of a walk, possibly
// more than once, so it reads the static again instead of scanning the heap.
func TestExpiredOffsetZeroIsReadFromTheStaticAgain(t *testing.T) {
	var scans atomic.Int32
	vm := dialFakeVM(t, func(method string, params map[string]any) (any, *vmRPCError) {
		switch method {
		case "invoke":
			if params["targetId"] == "classes/offset" && params["selector"] == "get:zero" {
				return fakeInstance("objects/zero-new", offsetClass), nil
			}
		case "getInstances":
			scans.Add(1)
		}
		return nil, &vmRPCError{Code: -32000, Message: "unexpected call " + method}
	})
	vm.zeroOffsetID = "objects/zero-old"
	vm.offsetClassID = "classes/offset"

	id, err := vm.refreshZeroOffsetID("objects/zero-old")
	if err != nil {
		t.Fatalf("refreshZeroOffsetID failed: %v", err)
	}
	if id != "objects/zero-new" {
		t.Errorf("id = %q, want objects/zero-new", id)
	}
	if n := scans.Load(); n != 0 {
		t.Errorf("the heap was scanned %d times, want 0", n)
	}
}

func TestIsExpiredArgumentErrorOnlyMatchesRejectedArgumentIds(t *testing.T) {
	cases := map[error]bool{
		&vmRPCError{Code: vmInvalidParams, Message: "Invalid params", Details: "invoke: invalid 'argumentIds' parameter: [objects/1/0]"}: true,
		&vmRPCError{Code: vmInvalidParams, Message: "Invalid params", Details: "invoke: invalid 'targetId' parameter: objects/1/0"}:      false,
		&vmRPCError{Code: 113, Message: "Expression compilation error"}:                                                                  false,
		errors.New("vm service call \"invoke\" timed out"):                                                                               false,
		nil: false,
	}
	for err, want := range cases {
		if got := isExpiredArgumentError(err); got != want {
			t.Errorf("isExpiredArgumentError(%v) = %v, want %v", err, got, want)
		}
	}
}

func fakeOffset(dx, dy string) map[string]any {
	return map[string]any{"fields": []any{
		map[string]any{"name": "_dx", "value": map[string]any{"kind": "Double", "valueAsString": dx}},
		map[string]any{"name": "_dy", "value": map[string]any{"kind": "Double", "valueAsString": dy}},
	}}
}

func fakeClass(id, name string) map[string]any {
	return map[string]any{"type": "@Class", "id": id, "name": name}
}

func fakeInstance(id, className string) map[string]any {
	return map[string]any{"type": "@Instance", "kind": "PlainInstance", "id": id, "class": map[string]any{"name": className}}
}

// fakeFlutterLibraries is the getIsolate answer of an isolate that has loaded
// the two libraries a dump is bootstrapped from, among others.
func fakeFlutterLibraries() map[string]any {
	return map[string]any{"libraries": []any{
		map[string]any{"id": "libraries/core", "uri": "dart:core"},
		map[string]any{"id": "libraries/ui", "uri": dartUILibrary},
		map[string]any{"id": "libraries/widgets-binding", "uri": widgetsBindingLibrary},
	}}
}

// fakeFlutterLibrary is the getObject answer for one of those libraries, or nil
// for any other object id.
func fakeFlutterLibrary(objectID any) map[string]any {
	switch objectID {
	case "libraries/ui":
		return map[string]any{"classes": []any{fakeClass("classes/size", "Size"), fakeClass("classes/offset", offsetClass)}}
	case "libraries/widgets-binding":
		return map[string]any{"classes": []any{fakeClass("classes/widgets-binding", widgetsBindingMixin), fakeClass("classes/binding", bindingClass)}}
	}
	return nil
}

// fakeFlutterStatics answers the invokes that lead from the two classes to the
// walk roots, or nil for any other invoke.
func fakeFlutterStatics(params map[string]any) map[string]any {
	switch fmt.Sprint(params["targetId"], " ", params["selector"]) {
	case "classes/widgets-binding get:instance":
		return fakeInstance("objects/binding", bindingClass)
	case "objects/binding get:renderViews":
		return fakeInstance("objects/views", "_CompactValuesIterable")
	case "objects/views get:first":
		return fakeInstance("objects/root", rootRenderClass)
	case "classes/offset get:zero":
		return fakeInstance("objects/zero", offsetClass)
	}
	return nil
}

// dialFakeVM connects a flutterVM to a fake VM service that answers each
// JSON-RPC call with handle's result or error.
func dialFakeVM(t *testing.T, handle func(method string, params map[string]any) (any, *vmRPCError)) *flutterVM {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		for {
			var req struct {
				ID     int            `json:"id"`
				Method string         `json:"method"`
				Params map[string]any `json:"params"`
			}
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			resp := map[string]any{"jsonrpc": "2.0", "id": req.ID}
			result, rpcErr := handle(req.Method, req.Params)
			if rpcErr != nil {
				resp["error"] = map[string]any{"code": rpcErr.Code, "message": rpcErr.Message, "data": map[string]any{"details": rpcErr.Details}}
			} else {
				resp["result"] = result
			}
			payload, _ := json.Marshal(resp)
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)

	vm, err := dialFlutterVM("ws" + strings.TrimPrefix(srv.URL, "http"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vm.close)
	return vm
}
