package agents

import _ "embed"

//go:embed android/mobilecli.so
var AndroidMobilecliSO []byte

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
