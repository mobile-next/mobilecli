package utils

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleInfoPlist is a minimal XML Info.plist; howett.net/plist parses both XML
// and binary plists, so XML keeps the test fixtures readable.
const sampleInfoPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>com.mobilenext.playground</string>
	<key>CFBundleShortVersionString</key>
	<string>1.4.0</string>
	<key>CFBundleVersion</key>
	<string>42</string>
</dict>
</plist>`

// writeZip writes a zip file with the given entries to a temp file and returns its path.
func writeZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.zip")

	f, err := os.Create(path)
	require.NoError(t, err)

	w := zip.NewWriter(f)
	for name, content := range entries {
		entry, err := w.Create(name)
		require.NoError(t, err)
		_, err = entry.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	require.NoError(t, f.Close())

	return path
}

func TestParseAppMetadataFromIpaLayout(t *testing.T) {
	ipa := writeZip(t, map[string]string{
		"Payload/Playground.app/Info.plist": sampleInfoPlist,
	})
	// rename to .ipa so extension dispatch picks the iOS path
	ipaPath := ipa + ".ipa"
	require.NoError(t, os.Rename(ipa, ipaPath))

	meta, err := ParseAppMetadata(ipaPath)
	require.NoError(t, err)
	assert.Equal(t, "com.mobilenext.playground", meta.PackageName)
	assert.Equal(t, "1.4.0", meta.Version)
	assert.Equal(t, "42", meta.VersionCode)
}

func TestParseAppMetadataFromSimulatorZipLayout(t *testing.T) {
	// simulator .zip has the .app at the archive root, not under Payload/
	zipPath := writeZip(t, map[string]string{
		"Playground.app/Info.plist": sampleInfoPlist,
	})

	meta, err := ParseAppMetadata(zipPath)
	require.NoError(t, err)
	assert.Equal(t, "com.mobilenext.playground", meta.PackageName)
	assert.Equal(t, "1.4.0", meta.Version)
	assert.Equal(t, "42", meta.VersionCode)
}

func TestParseAppMetadataIgnoresNestedBundlePlists(t *testing.T) {
	// a framework's Info.plist must not shadow the top-level app's
	ipa := writeZip(t, map[string]string{
		"Payload/Playground.app/Frameworks/Other.framework/Info.plist": `<plist><dict><key>CFBundleIdentifier</key><string>com.other.framework</string></dict></plist>`,
		"Payload/Playground.app/Info.plist":                            sampleInfoPlist,
	})
	ipaPath := ipa + ".ipa"
	require.NoError(t, os.Rename(ipa, ipaPath))

	meta, err := ParseAppMetadata(ipaPath)
	require.NoError(t, err)
	assert.Equal(t, "com.mobilenext.playground", meta.PackageName)
}

func TestParseAppMetadataFromAppDirectory(t *testing.T) {
	appDir := filepath.Join(t.TempDir(), "Playground.app")
	require.NoError(t, os.Mkdir(appDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(appDir, "Info.plist"), []byte(sampleInfoPlist), 0o600))

	meta, err := ParseAppMetadata(appDir)
	require.NoError(t, err)
	assert.Equal(t, "com.mobilenext.playground", meta.PackageName)
	assert.Equal(t, "1.4.0", meta.Version)
	assert.Equal(t, "42", meta.VersionCode)
}

func TestParseAppMetadataFromApk(t *testing.T) {
	// sample.apk is a stripped-down fixture: just the binary AndroidManifest.xml
	// plus an empty resources.arsc (the parser requires the latter to exist).
	// package/version are literal manifest attributes, so no resource table is needed.
	meta, err := ParseAppMetadata("testdata/sample.apk")
	require.NoError(t, err)
	assert.Equal(t, "com.example.helloworld", meta.PackageName)
	assert.Equal(t, "1.0", meta.Version)
	assert.Equal(t, "1", meta.VersionCode)
}

func TestParseAppMetadataRejectsUnknownExtension(t *testing.T) {
	_, err := ParseAppMetadata("/tmp/whatever.txt")
	assert.Error(t, err)
}
