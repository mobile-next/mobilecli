package machomap

import "errors"

// Helpers for driving the target after a map: calling an exported function, and
// asking a class that may already be registered (used to keep injection
// idempotent — mapping the same image twice fails on duplicate class pairs).

// ErrClassMethodNotFound means the class is registered but lacks the class method.
var ErrClassMethodNotFound = errors.New("class method not found")

var classMethodHelpers = []string{
	"mmap", "objc_getClass", "sel_registerName", "class_getClassMethod", "method_getImplementation",
}

// CallFunction borrows a thread and calls fn(args...) once, returning x0. Suitable
// for simple exported getters/setters; it uses the default return breakpoint, so
// callers should not use it for re-entrant functions (Map sets up a trampoline
// for those).
func CallFunction(c *RSP, fn uint64, args ...uint64) (uint64, error) {
	t, err := NewTarget(c)
	if err != nil {
		return 0, err
	}
	defer func() { _ = t.Restore() }()
	return t.Call(fn, args...)
}

// CallClassMethod calls +[class selector] in the target and returns x0. The second
// result is false when the class is not registered.
func CallClassMethod(c *RSP, res Resolver, class, selector string) (uint64, bool, error) {
	h, err := res.Resolve(classMethodHelpers)
	if err != nil {
		return 0, false, err
	}
	t, err := NewTarget(c)
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = t.Restore() }()
	scratch, err := t.Call(h["mmap"], 0, 0x1000, protRead|protWrite, mapAnon|mapPrivate, neg1, 0)
	if err != nil {
		return 0, false, err
	}
	names := append(append([]byte(class), 0), append([]byte(selector), 0)...)
	if err := c.WriteMem(scratch, names, 0); err != nil {
		return 0, false, err
	}
	// the objc runtime can wait on a lock until every thread runs, and the default
	// return breakpoint sits in shared code that another thread could then hit
	landingPage, err := t.Call(h["mmap"], 0, 0x1000, protRead|protExec, mapAnon|mapPrivate, neg1, 0)
	if err != nil {
		return 0, false, err
	}
	if err := t.SetLanding(landingPage); err != nil {
		return 0, false, err
	}
	return callClassMethod(t, h, scratch, scratch+uint64(len(class))+1)
}

// callClassMethod looks the method up before calling it, because sending an
// unknown selector would crash the app.
func callClassMethod(t *Target, h map[string]uint64, className, selectorName uint64) (uint64, bool, error) {
	cls, err := t.Call(h["objc_getClass"], className)
	if err != nil || cls == 0 {
		return 0, false, err
	}
	sel, err := t.Call(h["sel_registerName"], selectorName)
	if err != nil {
		return 0, false, err
	}
	method, err := t.Call(h["class_getClassMethod"], cls, sel)
	if err != nil {
		return 0, false, err
	}
	if method == 0 {
		return 0, true, ErrClassMethodNotFound
	}
	implementation, err := t.Call(h["method_getImplementation"], method)
	if err != nil {
		return 0, false, err
	}
	value, err := t.Call(implementation, cls, sel)
	if err != nil {
		return 0, false, err
	}
	return value, true, nil
}
