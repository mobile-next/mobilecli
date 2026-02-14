package utils

import (
	"os/exec"
	"runtime"
	"testing"
)

func hasPLUtil() bool {
	_, err := exec.LookPath("plutil")
	return err == nil
}

func TestConvertPlistToJSON(t *testing.T) {
	if runtime.GOOS != "darwin" || !hasPLUtil() {
		t.Skip("plutil only available on macOS")
	}

	// a minimal binary plist in XML format (plutil can read XML plist)
	plistData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>TestApp</string>
	<key>CFBundleVersion</key>
	<string>1.0</string>
</dict>
</plist>`)

	var result map[string]interface{}
	err := ConvertPlistToJSON(plistData, &result)
	if err != nil {
		t.Fatalf("ConvertPlistToJSON() error: %v", err)
	}

	if result["CFBundleName"] != "TestApp" {
		t.Errorf("CFBundleName = %v, want %q", result["CFBundleName"], "TestApp")
	}
	if result["CFBundleVersion"] != "1.0" {
		t.Errorf("CFBundleVersion = %v, want %q", result["CFBundleVersion"], "1.0")
	}
}

func TestConvertPlistToJSON_InvalidInput(t *testing.T) {
	if runtime.GOOS != "darwin" || !hasPLUtil() {
		t.Skip("plutil only available on macOS")
	}

	var result map[string]interface{}
	err := ConvertPlistToJSON([]byte("not a plist"), &result)
	if err == nil {
		t.Error("expected error for invalid plist data")
	}
}
