package devices

import (
	"regexp"
)

// A widget Key (`key: Key('login-button')`) lives only in the Dart widget tree —
// it is never published to the semantics or platform accessibility trees — yet
// Flutter's own tooling (find.byKey, Patrol, flutter_driver's byValueKey) teaches
// it as the way to identify a widget. In a debug build every render object keeps
// a debugCreator whose toString() is the creator chain of the element that made
// it, innermost first, each widget printed via toStringShort():
//
//	Semantics ← _ButtonStyleState ← ElevatedButton-[<'login-button'>] ← Column ← …
//
// Reading that one string costs two invokes per render object. Walking the
// elements with getObject instead fans out an object id per field, which evicts
// ids we hold (Offset.zero) from the VM service's id ring mid-walk.

// stringValueKeyPattern matches a widget printed with a ValueKey<String>:
// `ElevatedButton-[<'login-button'>]`, followed by the ` ← ` separator
// Element.debugGetCreatorChain uses or the end of the chain. The key itself may
// contain that separator, so the chain is matched whole rather than split on it.
// Other keys (GlobalKey, ObjectKey, ValueKey<int>) have no quoted string and are
// ignored.
var stringValueKeyPattern = regexp.MustCompile(`-\[<'(.*?)'>\]( ← |$)`)

// readWidgetKey returns the innermost String ValueKey in the creator chain of a
// render object, or "" when there is none (or the build is not a debug build,
// where debugCreator is null).
func (vm *flutterVM) readWidgetKey(renderNodeID string) string {
	creator, err := vm.invoke(renderNodeID, "get:debugCreator", nil)
	if err != nil || creator == nil || creator.ID == "" || creator.Kind == dartKindNull {
		return ""
	}
	chain, err := vm.invoke(creator.ID, "toString", nil)
	if err != nil || chain == nil {
		return ""
	}
	text := chain.ValueAsStr
	if chain.ValueAsStrTruncated {
		full, err := vm.getObject(chain.ID)
		if err != nil {
			return ""
		}
		text = full.ValueAsStr
	}
	return innermostStringKey(text)
}

// innermostStringKey returns the first String ValueKey in a creator chain.
func innermostStringKey(chain string) string {
	if m := stringValueKeyPattern.FindStringSubmatch(chain); m != nil {
		return m[1]
	}
	return ""
}
