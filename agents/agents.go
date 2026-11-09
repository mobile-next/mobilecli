package agents

import _ "embed"

//go:embed android/mobilecli-arm64-v8a.so
var androidMobilecliSOArm64 []byte

//go:embed android/mobilecli-x86_64.so
var androidMobilecliSOX8664 []byte

// AndroidMobilecliSO is the JVMTI agent loaded into a debuggable app, keyed by
// the Android ABI it is built for. It must match the ABI the app process runs
// as, or dlopen rejects it.
var AndroidMobilecliSO = map[string][]byte{
	"arm64-v8a": androidMobilecliSOArm64,
	"x86_64":    androidMobilecliSOX8664,
}

//go:embed android/mobilecli.dex
var AndroidMobilecliDEX []byte

//go:embed ios/agent-sim.dylib
var IOSAgentSimDylib []byte

// IOSRealDeviceWebViewAgent is the Objective-C expression evaluated via LLDB to
// inject the webview agent into a foreground app on a real iOS device. It is an
// LLDB expression (top-level statements), not a standalone translation unit, so
// it is not compiled — only embedded. See devices/ios_device_webview.go.
//
//go:embed ios-real/agent.m
var IOSRealDeviceWebViewAgent string
