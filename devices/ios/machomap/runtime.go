package machomap

// Helpers for driving the target after a map: calling an exported function, and
// checking whether a class is already registered (used to keep injection
// idempotent — mapping the same image twice fails on duplicate class pairs).

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

// ClassIsRegistered reports whether objc_getClass(name) is non-nil in the target.
func ClassIsRegistered(c *RSP, res Resolver, name string) (bool, error) {
	h, err := res.Resolve([]string{"mmap", "objc_getClass"})
	if err != nil {
		return false, err
	}
	t, err := NewTarget(c)
	if err != nil {
		return false, err
	}
	defer func() { _ = t.Restore() }()
	scratch, err := t.Call(h["mmap"], 0, 0x1000, protRead|protWrite, mapAnon|mapPrivate, neg1, 0)
	if err != nil {
		return false, err
	}
	if err := c.WriteMem(scratch, append([]byte(name), 0), 0); err != nil {
		return false, err
	}
	// objc_getClass can wait on a lock until every thread runs, and the default
	// return breakpoint sits in shared code that another thread could then hit
	landingPage, err := t.Call(h["mmap"], 0, 0x1000, protRead|protExec, mapAnon|mapPrivate, neg1, 0)
	if err != nil {
		return false, err
	}
	if err := t.SetLanding(landingPage); err != nil {
		return false, err
	}
	cls, err := t.Call(h["objc_getClass"], scratch)
	if err != nil {
		return false, err
	}
	return cls != 0, nil
}
