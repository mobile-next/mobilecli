package utils

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func createTestZip(t *testing.T, files map[string]string) string {
	t.Helper()

	zipPath := filepath.Join(t.TempDir(), "test.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	w := zip.NewWriter(f)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	return zipPath
}

func TestUnzip_ExtractsFiles(t *testing.T) {
	zipPath := createTestZip(t, map[string]string{
		"hello.txt":        "hello world",
		"subdir/nested.txt": "nested content",
	})

	destDir, err := Unzip(zipPath)
	if err != nil {
		t.Fatalf("Unzip() error: %v", err)
	}
	defer os.RemoveAll(destDir)

	// verify hello.txt
	content, err := os.ReadFile(filepath.Join(destDir, "hello.txt"))
	if err != nil {
		t.Fatalf("failed to read hello.txt: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("hello.txt content = %q, want %q", string(content), "hello world")
	}

	// verify nested file
	content, err = os.ReadFile(filepath.Join(destDir, "subdir", "nested.txt"))
	if err != nil {
		t.Fatalf("failed to read subdir/nested.txt: %v", err)
	}
	if string(content) != "nested content" {
		t.Errorf("nested.txt content = %q, want %q", string(content), "nested content")
	}
}

func TestUnzip_InvalidZipFile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "notazip.zip")
	if err := os.WriteFile(tmpFile, []byte("this is not a zip file"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Unzip(tmpFile)
	if err == nil {
		t.Error("expected error for invalid zip file")
	}
}

func TestUnzip_NonexistentFile(t *testing.T) {
	_, err := Unzip("/nonexistent/path/file.zip")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestUnzipFile_PathTraversalAbsolute(t *testing.T) {
	// create a zip with an absolute path entry
	zipPath := filepath.Join(t.TempDir(), "evil.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	w := zip.NewWriter(f)
	fw, err := w.Create("/etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("evil"))
	w.Close()
	f.Close()

	destDir := t.TempDir()
	err = unzipFile(zipPath, destDir)
	if err == nil {
		t.Error("expected error for absolute path in zip")
	}
}

func TestUnzipFile_PathTraversalDotDot(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "evil.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	w := zip.NewWriter(f)
	fw, err := w.Create("../../../etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("evil"))
	w.Close()
	f.Close()

	destDir := t.TempDir()
	err = unzipFile(zipPath, destDir)
	if err == nil {
		t.Error("expected error for path traversal in zip")
	}
}
