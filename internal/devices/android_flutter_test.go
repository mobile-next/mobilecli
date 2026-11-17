package devices

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/mobile-next/mobilecli/internal/types"
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

// A parent keeps some children in the render tree without laying them out or
// painting them: the routes under an opaque one, the hidden children of an
// IndexedStack, the child of an Offstage. It marks them offstage in
// debugDescribeChildren, and they must be left out along with everything below
// them: they are not on screen, and a deep navigation stack is mostly them.
func TestChildrenOfLeavesOutOffstageChildren(t *testing.T) {
	vm := dialFakeVM(t, fakeRenderParent(map[string]map[string]any{
		"objects/describes-onstage":  fakeDescribedChild("objects/onstage", "RenderPadding", ""),
		"objects/describes-offstage": fakeDescribedChild("objects/offstage", "RenderSemanticsAnnotations", "DiagnosticsTreeStyle.offstage"),
		"objects/describes-sparse":   fakeDescribedChild("objects/sparse", "RenderParagraph", "DiagnosticsTreeStyle.sparse"),
	}))

	kids, _ := vm.childrenOf("objects/parent")

	want := []renderChild{
		{id: "objects/onstage", class: "RenderPadding"},
		{id: "objects/sparse", class: "RenderParagraph"},
	}
	if fmt.Sprint(kids) != fmt.Sprint(want) {
		t.Errorf("children = %v, want %v", kids, want)
	}
}

// Nothing below an offstage child may be asked for either: skipping the subtree
// is what makes a dump of a deep screen cheaper.
func TestVisitDoesNotWalkAnOffstageSubtree(t *testing.T) {
	var offstageCalls atomic.Int32
	parent := fakeRenderParent(map[string]map[string]any{
		"objects/describes-offstage": fakeDescribedChild("objects/offstage", "RenderSemanticsAnnotations", "DiagnosticsTreeStyle.offstage"),
	})
	vm := dialFakeVM(t, func(method string, params map[string]any) (any, *vmRPCError) {
		if params["targetId"] == "objects/offstage" || params["objectId"] == "objects/offstage" {
			offstageCalls.Add(1)
		}
		return parent(method, params)
	})

	if _, err := vm.visit("objects/parent", "RenderIndexedStack", nil, ""); err != nil {
		t.Fatalf("visit failed: %v", err)
	}
	if n := offstageCalls.Load(); n != 0 {
		t.Errorf("the offstage child was asked %d times, want 0", n)
	}
}

// A parent whose children are all offstage paints nothing, so it is not a leaf
// to report: an Offstage sized by tight constraints, or an IndexedStack with no
// index, still has a size and a position, and would come out as an element
// nobody can see.
func TestVisitReportsNothingForAParentWhoseChildrenAreAllOffstage(t *testing.T) {
	parent := fakeRenderParent(map[string]map[string]any{
		"objects/describes-offstage": fakeDescribedChild("objects/offstage", "RenderParagraph", "DiagnosticsTreeStyle.offstage"),
	})
	vm := dialFakeVM(t, func(method string, params map[string]any) (any, *vmRPCError) {
		if answer, ok := fakeRectOfParent(method, params); ok {
			return answer, nil
		}
		return parent(method, params)
	})
	vm.dpr = 1

	els, err := vm.visit("objects/parent", "RenderOffstage", nil, "")

	if err != nil {
		t.Fatalf("visit failed: %v", err)
	}
	if len(els) != 0 {
		t.Errorf("elements = %+v, want none", els)
	}
}

// A render object that describes no children at all is still a leaf, and is
// reported with its rect.
func TestVisitReportsALeafThatHasNoChildren(t *testing.T) {
	parent := fakeRenderParent(map[string]map[string]any{})
	vm := dialFakeVM(t, func(method string, params map[string]any) (any, *vmRPCError) {
		if answer, ok := fakeRectOfParent(method, params); ok {
			return answer, nil
		}
		return parent(method, params)
	})
	vm.dpr = 1

	els, err := vm.visit("objects/parent", "RenderDecoratedBox", nil, "")

	if err != nil {
		t.Fatalf("visit failed: %v", err)
	}
	if len(els) != 1 || els[0].Rect.Width != 120 || els[0].Rect.Height != 40 {
		t.Errorf("elements = %+v, want one 120x40 element", els)
	}
}

// A diagnostics node that keeps its value behind a getter instead of in a field
// (anything that is not a DiagnosticableNode) is still asked through the getter.
func TestChildrenOfReadsAValueThatIsNotAField(t *testing.T) {
	parent := fakeRenderParent(map[string]map[string]any{
		"objects/describes-property": {"fields": []any{}},
	})
	vm := dialFakeVM(t, func(method string, params map[string]any) (any, *vmRPCError) {
		if method == "invoke" && params["targetId"] == "objects/describes-property" && params["selector"] == "get:value" {
			return map[string]any{"type": "@Instance", "id": "objects/property-child", "class": map[string]any{"name": "RenderBox"}}, nil
		}
		return parent(method, params)
	})

	kids, _ := vm.childrenOf("objects/parent")

	want := []renderChild{{id: "objects/property-child", class: "RenderBox"}}
	if fmt.Sprint(kids) != fmt.Sprint(want) {
		t.Errorf("children = %v, want %v", kids, want)
	}
}

// style is a field of every DiagnosticsNode, so a node that keeps its value
// behind a getter can be marked offstage just the same. It is left out too, and
// without being asked for the value.
func TestChildrenOfLeavesOutAnOffstageChildWhoseValueIsNotAField(t *testing.T) {
	var valueReads atomic.Int32
	offstageProperty := map[string]any{"fields": []any{
		map[string]any{"name": "style", "value": map[string]any{"type": "@Instance", "kind": "PlainInstance", "id": "styles/DiagnosticsTreeStyle.offstage"}},
	}}
	parent := fakeRenderParent(map[string]map[string]any{"objects/describes-property": offstageProperty})
	vm := dialFakeVM(t, func(method string, params map[string]any) (any, *vmRPCError) {
		if method == "invoke" && params["targetId"] == "objects/describes-property" && params["selector"] == "get:value" {
			valueReads.Add(1)
			return map[string]any{"type": "@Instance", "id": "objects/property-child", "class": map[string]any{"name": "RenderBox"}}, nil
		}
		return parent(method, params)
	})

	if kids, _ := vm.childrenOf("objects/parent"); len(kids) != 0 {
		t.Errorf("children = %v, want none", kids)
	}
	if n := valueReads.Load(); n != 0 {
		t.Errorf("the offstage node's value was read %d times, want 0", n)
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

// fakeDescribedChild is the getObject answer for a DiagnosticableTreeNode that
// describes a child render object. style is the toString of the style the
// parent passed, or "" for a node created without one.
func fakeDescribedChild(childID, childClass, style string) map[string]any {
	styleRef := map[string]any{"type": "@Instance", "kind": "Null", "id": "objects/null"}
	if style != "" {
		styleRef = map[string]any{"type": "@Instance", "kind": "PlainInstance", "id": "styles/" + style}
	}
	return map[string]any{"fields": []any{
		map[string]any{"name": "name", "value": map[string]any{"kind": "String", "valueAsString": "child"}},
		map[string]any{"name": "style", "value": styleRef},
		map[string]any{"name": "value", "value": map[string]any{"type": "@Instance", "id": childID, "class": map[string]any{"name": childClass}}},
	}}
}

// fakeFieldValue returns the value of a field in a fake getObject answer, or nil.
func fakeFieldValue(object map[string]any, name string) any {
	fields, _ := object["fields"].([]any)
	for _, f := range fields {
		if field, ok := f.(map[string]any); ok && field["name"] == name {
			return field["value"]
		}
	}
	return nil
}

// fakeRenderParent is a fake VM whose objects/parent describes the given
// diagnostics nodes as its children, in id order, and answers both ways of
// reading the child out of one: its `value` field and its `value` getter. Every
// other render object has no children, no semantics, no creator and no size.
func fakeRenderParent(describes map[string]map[string]any) func(method string, params map[string]any) (any, *vmRPCError) {
	ids := make([]string, 0, len(describes))
	for id := range describes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	elements := make([]any, 0, len(ids))
	for _, id := range ids {
		elements = append(elements, map[string]any{"type": "@Instance", "id": id})
	}

	return func(method string, params map[string]any) (any, *vmRPCError) {
		switch method {
		case "invoke":
			target, _ := params["targetId"].(string)
			switch {
			case params["selector"] == "debugDescribeChildren" && target == "objects/parent":
				return map[string]any{"type": "@Instance", "id": "objects/children"}, nil
			case params["selector"] == "toString" && strings.HasPrefix(target, "styles/"):
				return map[string]any{"type": "@Instance", "kind": "String", "valueAsString": strings.TrimPrefix(target, "styles/")}, nil
			case params["selector"] == "get:value":
				if value := fakeFieldValue(describes[target], "value"); value != nil {
					return value, nil
				}
			}
			return map[string]any{"type": "@Instance", "kind": "Null", "id": "objects/null"}, nil
		case "getObject":
			id, _ := params["objectId"].(string)
			if id == "objects/children" {
				return map[string]any{"elements": elements}, nil
			}
			if node, ok := describes[id]; ok {
				return node, nil
			}
		}
		return nil, &vmRPCError{Code: -32000, Message: "unexpected call " + method}
	}
}

// fakeRectOfParent answers the calls that read the global rect of objects/parent:
// a 120x40 box at 10,20.
func fakeRectOfParent(method string, params map[string]any) (any, bool) {
	switch {
	case method == "invoke" && params["targetId"] == "objects/parent" && params["selector"] == "localToGlobal":
		return map[string]any{"type": "@Instance", "id": "objects/parent-offset"}, true
	case method == "invoke" && params["targetId"] == "objects/parent" && params["selector"] == "get:size":
		return map[string]any{"type": "@Instance", "id": "objects/parent-size"}, true
	case method == "getObject" && params["objectId"] == "objects/parent-offset":
		return fakeOffset("10.0", "20.0"), true
	case method == "getObject" && params["objectId"] == "objects/parent-size":
		return fakeOffset("120.0", "40.0"), true
	}
	return nil, false
}
