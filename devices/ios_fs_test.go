package devices

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielpaulus/go-ios/ios/afc"
)

// fakeAfc is an in-memory afcFS.
type fakeAfc struct {
	files      map[string][]byte
	dirs       map[string]bool
	closeErr   error
	readErr    error
	removed    []string
	removedAll []string
}

func newFakeAfc(dirs ...string) *fakeAfc {
	f := &fakeAfc{files: map[string][]byte{}, dirs: map[string]bool{"/": true}}
	for _, d := range dirs {
		f.dirs[d] = true
	}
	return f
}

func (f *fakeAfc) Stat(p string) (afc.FileInfo, error) {
	if f.dirs[p] {
		return afc.FileInfo{Name: path.Base(p), Type: afc.S_IFDIR}, nil
	}
	if b, ok := f.files[p]; ok {
		return afc.FileInfo{Name: path.Base(p), Type: afc.S_IFMT, Size: int64(len(b))}, nil
	}
	return afc.FileInfo{}, errors.New("not found")
}

type fakeWriter struct {
	f *fakeAfc
	p string
}

func (w *fakeWriter) Write(b []byte) (int, error) {
	if len(b) > afcChunkSize {
		return 0, errors.New("oversize packet")
	}
	w.f.files[w.p] = append(w.f.files[w.p], b...)
	return len(b), nil
}
func (w *fakeWriter) Close() error { return w.f.closeErr }

type errReader struct {
	r   io.Reader
	err error
}

func (e *errReader) Read(p []byte) (int, error) {
	n, err := e.r.Read(p)
	if err == io.EOF && e.err != nil {
		return n, e.err
	}
	return n, err
}
func (e *errReader) Close() error { return nil }

func (f *fakeAfc) OpenRead(p string) (io.ReadCloser, error) {
	b, ok := f.files[p]
	if !ok {
		return nil, errors.New("not found")
	}
	return &errReader{r: bytes.NewReader(b), err: f.readErr}, nil
}

func (f *fakeAfc) OpenWrite(p string) (io.WriteCloser, error) {
	f.files[p] = nil
	return &fakeWriter{f: f, p: p}, nil
}

func (f *fakeAfc) MkDir(p string) error {
	if !f.dirs[path.Dir(p)] {
		return errors.New("parent missing")
	}
	f.dirs[p] = true
	return nil
}

func (f *fakeAfc) children(p string) bool {
	prefix := strings.TrimRight(p, "/") + "/"
	for k := range f.files {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	for k := range f.dirs {
		if k != p && strings.HasPrefix(k, prefix) {
			return true
		}
	}
	return false
}

func (f *fakeAfc) Remove(p string) error {
	f.removed = append(f.removed, p)
	if _, ok := f.files[p]; ok {
		delete(f.files, p)
		return nil
	}
	if f.dirs[p] {
		if f.children(p) {
			return errors.New("directory not empty")
		}
		delete(f.dirs, p)
		return nil
	}
	return errors.New("not found")
}

func (f *fakeAfc) RemoveAll(p string) error {
	f.removedAll = append(f.removedAll, p)
	prefix := strings.TrimRight(p, "/") + "/"
	for k := range f.files {
		if k == p || strings.HasPrefix(k, prefix) {
			delete(f.files, k)
		}
	}
	for k := range f.dirs {
		if k == p || strings.HasPrefix(k, prefix) {
			delete(f.dirs, k)
		}
	}
	return nil
}

func writeLocal(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAfcPush(t *testing.T) {
	big := bytes.Repeat([]byte("0123456789abcdef"), 10*1024) // 160 KiB, multi-chunk

	t.Run("new file", func(t *testing.T) {
		fs := newFakeAfc("/Downloads")
		l := writeLocal(t, t.TempDir(), "a.txt", []byte("hello"))
		if err := afcPush(fs, l, "/Downloads/a.txt"); err != nil {
			t.Fatal(err)
		}
		if string(fs.files["/Downloads/a.txt"]) != "hello" {
			t.Fatalf("got %q", fs.files["/Downloads/a.txt"])
		}
	})
	t.Run("overwrite", func(t *testing.T) {
		fs := newFakeAfc("/Downloads")
		fs.files["/Downloads/a.txt"] = []byte("old old old")
		l := writeLocal(t, t.TempDir(), "a.txt", []byte("new"))
		if err := afcPush(fs, l, "/Downloads/a.txt"); err != nil {
			t.Fatal(err)
		}
		if string(fs.files["/Downloads/a.txt"]) != "new" {
			t.Fatalf("got %q", fs.files["/Downloads/a.txt"])
		}
	})
	t.Run("into existing dir appends name", func(t *testing.T) {
		fs := newFakeAfc("/Downloads")
		l := writeLocal(t, t.TempDir(), "b.txt", []byte("x"))
		if err := afcPush(fs, l, "/Downloads"); err != nil {
			t.Fatal(err)
		}
		if _, ok := fs.files["/Downloads/b.txt"]; !ok {
			t.Fatalf("files: %v", fs.files)
		}
	})
	t.Run("missing parent", func(t *testing.T) {
		fs := newFakeAfc()
		l := writeLocal(t, t.TempDir(), "a.txt", []byte("x"))
		if err := afcPush(fs, l, "/nope/a.txt"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("local dir or missing", func(t *testing.T) {
		fs := newFakeAfc("/Downloads")
		dir := t.TempDir()
		if err := afcPush(fs, dir, "/Downloads/x"); err == nil {
			t.Fatal("expected error for directory")
		}
		if err := afcPush(fs, filepath.Join(dir, "missing"), "/Downloads/x"); err == nil {
			t.Fatal("expected error for missing file")
		}
	})
	t.Run("multi chunk intact", func(t *testing.T) {
		fs := newFakeAfc("/Downloads")
		l := writeLocal(t, t.TempDir(), "big.bin", big)
		if err := afcPush(fs, l, "/Downloads/big.bin"); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(fs.files["/Downloads/big.bin"], big) {
			t.Fatal("content mismatch")
		}
	})
	t.Run("close error surfaced", func(t *testing.T) {
		fs := newFakeAfc("/Downloads")
		fs.closeErr = errors.New("boom")
		l := writeLocal(t, t.TempDir(), "a.txt", []byte("x"))
		if err := afcPush(fs, l, "/Downloads/a.txt"); err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("dotdot rejected", func(t *testing.T) {
		fs := newFakeAfc("/Downloads")
		l := writeLocal(t, t.TempDir(), "a.txt", []byte("x"))
		if err := afcPush(fs, l, "/Downloads/../x"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAfcPull(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		fs := newFakeAfc("/Downloads")
		fs.files["/Downloads/a.txt"] = []byte("hello")
		dst := filepath.Join(t.TempDir(), "out.txt")
		if err := afcPull(fs, "/Downloads/a.txt", dst); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(dst)
		if string(b) != "hello" {
			t.Fatalf("got %q", b)
		}
		st, _ := os.Stat(dst)
		if st.Mode().Perm() != 0o600 {
			t.Fatalf("mode %v", st.Mode().Perm())
		}
	})
	t.Run("directory", func(t *testing.T) {
		fs := newFakeAfc("/Downloads")
		if err := afcPull(fs, "/Downloads", filepath.Join(t.TempDir(), "o")); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("missing", func(t *testing.T) {
		fs := newFakeAfc()
		if err := afcPull(fs, "/nope", filepath.Join(t.TempDir(), "o")); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("into existing local dir", func(t *testing.T) {
		fs := newFakeAfc("/Downloads")
		fs.files["/Downloads/a.txt"] = []byte("hi")
		dir := t.TempDir()
		if err := afcPull(fs, "/Downloads/a.txt", dir); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "a.txt")); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("no partial file on read error", func(t *testing.T) {
		fs := newFakeAfc("/Downloads")
		fs.files["/Downloads/a.txt"] = []byte("hi")
		fs.readErr = errors.New("link down")
		dir := t.TempDir()
		if err := afcPull(fs, "/Downloads/a.txt", filepath.Join(dir, "o")); err == nil {
			t.Fatal("expected error")
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 0 {
			t.Fatalf("left behind: %v", entries)
		}
	})
	t.Run("multi chunk intact", func(t *testing.T) {
		big := bytes.Repeat([]byte("xyz"), 50000)
		fs := newFakeAfc("/Downloads")
		fs.files["/Downloads/big"] = big
		dst := filepath.Join(t.TempDir(), "big")
		if err := afcPull(fs, "/Downloads/big", dst); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(dst)
		if !bytes.Equal(b, big) {
			t.Fatal("content mismatch")
		}
	})
}

func TestAfcMkdir(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		fs := newFakeAfc()
		if err := afcMkdir(fs, "/claude", false); err != nil || !fs.dirs["/claude"] {
			t.Fatalf("err=%v dirs=%v", err, fs.dirs)
		}
	})
	t.Run("existing without parents", func(t *testing.T) {
		fs := newFakeAfc("/claude")
		if err := afcMkdir(fs, "/claude", false); err != nil {
			t.Fatal(err) // fake allows re-create, mirrors device behaviour
		}
	})
	t.Run("parents three levels", func(t *testing.T) {
		fs := newFakeAfc()
		if err := afcMkdir(fs, "/a/b/c", true); err != nil {
			t.Fatal(err)
		}
		for _, d := range []string{"/a", "/a/b", "/a/b/c"} {
			if !fs.dirs[d] {
				t.Fatalf("missing %s", d)
			}
		}
	})
	t.Run("parents over existing file", func(t *testing.T) {
		fs := newFakeAfc()
		fs.files["/a"] = []byte("x")
		if err := afcMkdir(fs, "/a/b", true); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("no parents, missing parent", func(t *testing.T) {
		fs := newFakeAfc()
		if err := afcMkdir(fs, "/a/b", false); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("root and dotdot", func(t *testing.T) {
		fs := newFakeAfc()
		if err := afcMkdir(fs, "/", true); err == nil {
			t.Fatal("expected error for root")
		}
		if err := afcMkdir(fs, "/../x", true); err == nil {
			t.Fatal("expected error for ..")
		}
	})
}

func TestAfcRm(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		fs := newFakeAfc("/d")
		fs.files["/d/f"] = []byte("x")
		if err := afcRm(fs, "/d/f", false); err != nil {
			t.Fatal(err)
		}
		if _, ok := fs.files["/d/f"]; ok {
			t.Fatal("still there")
		}
	})
	t.Run("empty dir", func(t *testing.T) {
		fs := newFakeAfc("/d")
		if err := afcRm(fs, "/d", false); err != nil || fs.dirs["/d"] {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("non-empty dir without recursive", func(t *testing.T) {
		fs := newFakeAfc("/d")
		fs.files["/d/f"] = []byte("x")
		if err := afcRm(fs, "/d", false); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("recursive removes tree", func(t *testing.T) {
		fs := newFakeAfc("/d", "/d/e")
		fs.files["/d/e/f"] = []byte("x")
		if err := afcRm(fs, "/d", true); err != nil {
			t.Fatal(err)
		}
		if fs.dirs["/d"] || fs.dirs["/d/e"] || len(fs.files) != 0 {
			t.Fatalf("left: %v %v", fs.dirs, fs.files)
		}
	})
	t.Run("root refused", func(t *testing.T) {
		for _, p := range []string{"/", "", "  ", ".", "//", "/./", "/a/..", "/../", "../"} {
			fs := newFakeAfc("/d")
			for _, rec := range []bool{false, true} {
				if err := afcRm(fs, p, rec); err == nil {
					t.Fatalf("path %q recursive=%v: expected error", p, rec)
				}
			}
			if len(fs.removed)+len(fs.removedAll) != 0 {
				t.Fatalf("path %q reached the filesystem", p)
			}
		}
	})
}

func TestCleanAfcPath(t *testing.T) {
	ok := map[string]string{
		"//a/./b":  "/a/b",
		"a/b":      "/a/b",
		"/":        "/",
		"":         "/",
		"/a/b/":    "/a/b",
		"/a//b///": "/a/b",
	}
	for in, want := range ok {
		got, err := cleanAfcPath(in)
		if err != nil || got != want {
			t.Errorf("cleanAfcPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"..", "../x", "/a/../b", "/a/.."} {
		if _, err := cleanAfcPath(in); err == nil {
			t.Errorf("cleanAfcPath(%q): expected error", in)
		}
	}
}
