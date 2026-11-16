package devices

import (
	"os"
	"path/filepath"
	"testing"
)

// The simulator's device directory is the only thing confining fs operations to
// the simulator instead of the whole Mac, so these tests pin down that boundary:
// legitimate paths are allowed, lexical traversal is rejected, and — the part that
// used to be bypassable — a symlink inside the sandbox cannot be used to reach the
// host filesystem.

func TestValidatePathWithinRootAllowsPathsInsideRoot(t *testing.T) {
	root := t.TempDir()
	documents := filepath.Join(root, "data", "Containers", "Documents")
	if err := os.MkdirAll(documents, 0o750); err != nil {
		t.Fatal(err)
	}

	allowed := []string{
		root,
		documents,
		filepath.Join(documents, "does-not-exist-yet.txt"), // fs push / mkdir target
		filepath.Join(documents, "a", "b", "c"),            // nested not-yet-created
	}
	for _, p := range allowed {
		if err := validatePathWithinRoot(root, p); err != nil {
			t.Errorf("expected %q to be allowed, got error: %v", p, err)
		}
	}
}

func TestValidatePathWithinRootRejectsLexicalEscape(t *testing.T) {
	root := t.TempDir()
	rejected := []string{
		filepath.Join(root, "..", "other-device"),
		filepath.Join(root, "Documents", "..", "..", "..", "..", "etc", "hosts"),
		"/etc/hosts",
		"/tmp",
	}
	for _, p := range rejected {
		if err := validatePathWithinRoot(root, p); err == nil {
			t.Errorf("expected %q to be rejected, but it was allowed", p)
		}
	}
}

// A symlink whose target is outside the root must not let an fs path escape, even
// though the textual path is still under the root. This is the case the old
// lexical-only check let through.
func TestValidatePathWithinRootRejectsSymlinkLeafEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("host data"), 0o600); err != nil {
		t.Fatal(err)
	}

	// A symlink sitting inside the sandbox, pointing at a file outside it.
	link := filepath.Join(root, "escape")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), link); err != nil {
		t.Fatal(err)
	}

	if err := validatePathWithinRoot(root, link); err == nil {
		t.Errorf("expected a symlink pointing outside the root to be rejected, but it was allowed")
	}
}

func TestValidatePathWithinRootRejectsSymlinkParentEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	// A symlinked directory inside the sandbox, pointing at a directory outside it.
	// Reading or writing a child of it (even one that does not exist yet, as with
	// fs push) must be rejected.
	link := filepath.Join(root, "tmplink")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	cases := []string{
		filepath.Join(link, "existing-or-not.txt"),
		filepath.Join(link, "nested", "deep.txt"),
	}
	for _, p := range cases {
		if err := validatePathWithinRoot(root, p); err == nil {
			t.Errorf("expected a path through a symlinked parent (%q) to be rejected, but it was allowed", p)
		}
	}
}

// A symlink that stays inside the sandbox is fine and must keep working.
func TestValidatePathWithinRootAllowsSymlinkThatStaysInside(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real")
	if err := os.MkdirAll(target, 0o750); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "alias")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := validatePathWithinRoot(root, filepath.Join(link, "file.txt")); err != nil {
		t.Errorf("expected a symlink that stays inside the root to be allowed, got error: %v", err)
	}
}
