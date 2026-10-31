package devices

import "testing"

func TestPrimaryCPUABIReadsThePackageABI(t *testing.T) {
	cases := map[string]string{
		// trimmed from `dumpsys package` on an x86_64 emulator
		"    codePath=/data/app/~~x==/it.example-y==\n    resourcePath=/data/app/~~x==/it.example-y==\n    legacyNativeLibraryDir=/data/app/~~x==/it.example-y==/lib\n    extractNativeLibs=false\n    primaryCpuAbi=x86_64\n    secondaryCpuAbi=null\n": "x86_64",
		"    primaryCpuAbi=arm64-v8a\n    secondaryCpuAbi=null\n": "arm64-v8a",
		// an app without native code has no ABI of its own
		"    primaryCpuAbi=null\n    secondaryCpuAbi=null\n": "",
		"Unable to find package: it.example\n":               "",
	}
	for dumpsys, want := range cases {
		if got := primaryCPUABI(dumpsys); got != want {
			t.Errorf("primaryCPUABI(%q) = %q, want %q", dumpsys, got, want)
		}
	}
}

func TestAgentSOForABIPicksTheMatchingBuild(t *testing.T) {
	for _, abi := range []string{"arm64-v8a", "x86_64"} {
		if _, err := agentSOForABI(abi); err != nil {
			t.Errorf("agentSOForABI(%q): %v", abi, err)
		}
	}
	for _, abi := range []string{"armeabi-v7a", "x86", ""} {
		if _, err := agentSOForABI(abi); err == nil {
			t.Errorf("agentSOForABI(%q) = nil error, want an error", abi)
		}
	}
}
