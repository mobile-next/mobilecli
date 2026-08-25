package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shogo82148/androidbinary/apk"
	"howett.net/plist"
)

// AppMetadata holds identity and version information parsed from an app file
// (.apk, .ipa, .zip, or .app). Field names are unified across platforms:
// PackageName is the Android package / iOS CFBundleIdentifier, Version is the
// Android versionName / iOS CFBundleShortVersionString (marketing version), and
// VersionCode is the Android versionCode / iOS CFBundleVersion (build number).
type AppMetadata struct {
	PackageName string `json:"packageName"`
	Version     string `json:"version,omitempty"`
	VersionCode string `json:"versionCode,omitempty"`
}

// ParseAppMetadata reads identity and version metadata from an app file,
// dispatching by extension. It parses the file locally and does not touch any
// device.
func ParseAppMetadata(path string) (*AppMetadata, error) {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".apk"):
		return parseApkMetadata(path)
	case strings.HasSuffix(lower, ".ipa"), strings.HasSuffix(lower, ".zip"):
		return parseIpaMetadata(path)
	case strings.HasSuffix(lower, ".app"):
		return parseAppDirMetadata(path)
	default:
		return nil, fmt.Errorf("unsupported app file type: %s", path)
	}
}

func parseApkMetadata(path string) (*AppMetadata, error) {
	pkg, err := apk.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open apk: %w", err)
	}
	defer func() { _ = pkg.Close() }()

	meta := &AppMetadata{
		PackageName: pkg.PackageName(),
	}

	manifest := pkg.Manifest()
	if versionName, err := manifest.VersionName.String(); err == nil {
		meta.Version = versionName
	}
	if versionCode, err := manifest.VersionCode.Int32(); err == nil {
		meta.VersionCode = strconv.FormatInt(int64(versionCode), 10)
	}

	return meta, nil
}

// parseIpaMetadata reads the top-level app's Info.plist from an .ipa or .zip
// archive. It works for both .ipa (Payload/Foo.app/Info.plist) and simulator
// .zip (Foo.app/Info.plist) layouts.
func parseIpaMetadata(path string) (*AppMetadata, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open archive: %w", err)
	}
	defer func() { _ = reader.Close() }()

	for _, file := range reader.File {
		if !isAppInfoPlist(file.Name) {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open Info.plist: %w", err)
		}

		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read Info.plist: %w", err)
		}

		return decodeInfoPlist(data)
	}

	return nil, fmt.Errorf("no app Info.plist found in %s", path)
}

func parseAppDirMetadata(path string) (*AppMetadata, error) {
	data, err := os.ReadFile(filepath.Join(path, "Info.plist"))
	if err != nil {
		return nil, fmt.Errorf("failed to read Info.plist: %w", err)
	}

	return decodeInfoPlist(data)
}

// isAppInfoPlist reports whether a zip entry name is the top-level app bundle's
// Info.plist, e.g. "Payload/Foo.app/Info.plist" or "Foo.app/Info.plist". A
// nested bundle's plist (frameworks, plugins) has more path segments and is
// excluded.
func isAppInfoPlist(name string) bool {
	name = strings.TrimPrefix(name, "Payload/")
	parts := strings.Split(name, "/")
	return len(parts) == 2 && strings.HasSuffix(parts[0], ".app") && parts[1] == "Info.plist"
}

func decodeInfoPlist(data []byte) (*AppMetadata, error) {
	// infoPlist mirrors the keys we extract from an iOS Info.plist.
	type infoPlist struct {
		CFBundleIdentifier         string `plist:"CFBundleIdentifier"`
		CFBundleShortVersionString string `plist:"CFBundleShortVersionString"`
		CFBundleVersion            string `plist:"CFBundleVersion"`
	}

	var info infoPlist
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse Info.plist: %w", err)
	}

	return &AppMetadata{
		PackageName: info.CFBundleIdentifier,
		Version:     info.CFBundleShortVersionString,
		VersionCode: info.CFBundleVersion,
	}, nil
}
