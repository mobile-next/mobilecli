package devices

import (
	"errors"
	"testing"
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
