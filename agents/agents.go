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

// IOSAgentDevDylib is the real-device build of the compiled agent (server +
// bridge + dispatcher + network). It is mapped into a foreground app over the
// debugserver by devices/ios/machomap — no code signing, no re-signing — and
// supersedes the LLDB-evaluated webview source agent on real devices.
//
//go:embed ios/agent-dev.dylib
var IOSAgentDevDylib []byte
