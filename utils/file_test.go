package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "source.txt")
	dstPath := filepath.Join(tmpDir, "dest.txt")

	content := "hello world\nline two\n"
	if err := os.WriteFile(srcPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("CopyFile() error: %v", err)
	}

	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read dest: %v", err)
	}
	if string(got) != content {
		t.Errorf("dest content = %q, want %q", string(got), content)
	}
}

func TestCopyFile_PreservesPermissions(t *testing.T) {
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "script.sh")
	dstPath := filepath.Join(tmpDir, "script_copy.sh")

	if err := os.WriteFile(srcPath, []byte("#!/bin/sh\necho hi\n"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("CopyFile() error: %v", err)
	}

	srcInfo, _ := os.Stat(srcPath)
	dstInfo, _ := os.Stat(dstPath)

	if srcInfo.Mode() != dstInfo.Mode() {
		t.Errorf("dest mode = %v, want %v", dstInfo.Mode(), srcInfo.Mode())
	}
}

func TestCopyFile_SourceNotFound(t *testing.T) {
	err := CopyFile("/nonexistent/file", filepath.Join(t.TempDir(), "dest"))
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
}

func TestGetProjectFile_NotImplemented(t *testing.T) {
	_, err := GetProjectFile("anything")
	if err == nil {
		t.Error("expected error from unimplemented function")
	}
}
