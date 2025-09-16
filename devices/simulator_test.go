package devices

import (
	"testing"
)

func TestExtractEnvValue(t *testing.T) {
	// This represents the output from ps -o pid,command -E -ww -e after the PID has been separated
	processInfo := `/Users/john/Library/Developer/CoreSimulator/Devices/D635CBF3-87C8-46AC-9F23-808E3ED2A6A0/data/Containers/Bundle/Application/61312706-4FCF-48D7-8F7F-BF6AC04B2070/WebDriverAgentRunner-Runner.app/WebDriverAgentRunner-Runner IOS_SIMULATOR_SYSLOG_SOCKET=/private/var/tmp/com.apple.CoreSimulator.SimDevice.D635CBF3-87C8-46AC-9F23-808E3ED2A6A0/syslogsock SIMULATOR_SHARED_RESOURCES_DIRECTORY=/Users/john/Library/Developer/CoreSimulator/Devices/D635CBF3-87C8-46AC-9F23-808E3ED2A6A0/data XPC_SIMULATOR_LAUNCHD_NAME=com.apple.CoreSimulator.SimDevice.D635CBF3-87C8-46AC-9F23-808E3ED2A6A0 DYLD_SHARED_CACHE_DIR=/Library/Developer/CoreSimulator/Caches/dyld/24G90/com.apple.CoreSimulator.SimRuntime.iOS-18-6.22G86/ IPHONE_SIMULATOR_ROOT=/Library/Developer/CoreSimulator/Volumes/iOS_22G86/Library/Developer/CoreSimulator/Profiles/Runtimes/iOS 18.6.simruntime/Contents/Resources/RuntimeRoot LS_ENABLE_BUNDLE_LOCALIZATION_CACHING=1 SIMULATOR_HID_SYSTEM_MANAGER=/Library/Developer/PrivateFrameworks/CoreSimulator.framework/Resources/Platforms/iphoneos/Library/Frameworks/SimulatorHID.framework SIMULATOR_MAINSCREEN_HEIGHT=2556 SIMULATOR_MEMORY_WARNINGS=/Users/john/Library/Developer/CoreSimulator/Devices/D635CBF3-87C8-46AC-9F23-808E3ED2A6A0/data/var/run/memory_warning_simulation SIMULATOR_AUDIO_DEVICES_PLIST_PATH=/Users/john/Library/Developer/CoreSimulator/Devices/D635CBF3-87C8-46AC-9F23-808E3ED2A6A0/data/var/run/com.apple.coresimulator.audio.plist SIMULATOR_LOG_ROOT=/Users/john/Library/Logs/CoreSimulator/D635CBF3-87C8-46AC-9F23-808E3ED2A6A0 SIMULATOR_RUNTIME_BUILD_VERSION=22G86 DYLD_ROOT_PATH=/Library/Developer/CoreSimulator/Volumes/iOS_22G86/Library/Developer/CoreSimulator/Profiles/Runtimes/iOS 18.6.simruntime/Contents/Resources/RuntimeRoot PATH=/Library/Developer/CoreSimulator/Volumes/iOS_22G86/Library/Developer/CoreSimulator/Profiles/Runtimes/iOS 18.6.simruntime/Contents/Resources/RuntimeRoot/usr/bin:/Library/Developer/CoreSimulator/Volumes/iOS_22G86/Library/Developer/CoreSimulator/Profiles/Runtimes/iOS 18.6.simruntime/Contents/Resources/RuntimeRoot/bin:/Library/Developer/CoreSimulator/Volumes/iOS_22G86/Library/Developer/CoreSimulator/Profiles/Runtimes/iOS 18.6.simruntime/Contents/Resources/RuntimeRoot/usr/sbin:/Library/Developer/CoreSimulator/Volumes/iOS_22G86/Library/Developer/CoreSimulator/Profiles/Runtimes/iOS 18.6.simruntime/Contents/Resources/RuntimeRoot/sbin:/Library/Developer/CoreSimulator/Volumes/iOS_22G86/Library/Developer/CoreSimulator/Profiles/Runtimes/iOS 18.6.simruntime/Contents/Resources/RuntimeRoot/usr/local/bin SIMULATOR_HOST_HOME=/Users/john SIMULATOR_ROOT=/Library/Developer/CoreSimulator/Volumes/iOS_22G86/Library/Developer/CoreSimulator/Profiles/Runtimes/iOS 18.6.simruntime/Contents/Resources/RuntimeRoot IPHONE_SHARED_RESOURCES_DIRECTORY=/Users/john/Library/Developer/CoreSimulator/Devices/D635CBF3-87C8-46AC-9F23-808E3ED2A6A0/data IPHONE_TVOUT_EXTENDED_PROPERTIES=/Users/john/Library/Developer/CoreSimulator/Devices/D635CBF3-87C8-46AC-9F23-808E3ED2A6A0/data/Library/Application Support/Simulator/extended_display.plist SIMULATOR_BOOT_TIME=1757347070 SIMULATOR_CAPABILITIES=/Library/Developer/CoreSimulator/Profiles/DeviceTypes/iPhone 16.simdevicetype/Contents/Resources/capabilities.plist SIMULATOR_FRAMEBUFFER_FRAMEWORK=/Library/Developer/PrivateFrameworks/CoreSimulator.framework/Resources/Platforms/iphoneos/Library/PrivateFrameworks/SimFramebuffer.framework/SimFramebuffer SIMULATOR_LEGACY_ASSET_SUFFIX=iphone SIMULATOR_MAINSCREEN_PITCH=460.000000 SIMULATOR_MAINSCREEN_WIDTH=1179 SIMULATOR_MODEL_IDENTIFIER=iPhone17,3 SIMULATOR_PRODUCT_CLASS=D47 SIMULATOR_RUNTIME_VERSION=18.6 SIMULATOR_ARCHS=arm64 x86_64 SIMULATOR_AUDIO_SETTINGS_PATH=/Users/john/Library/Developer/CoreSimulator/Devices/D635CBF3-87C8-46AC-9F23-808E3ED2A6A0/data/var/run/simulatoraudio/audiosettings.plist SIMULATOR_DEVICE_NAME=iPhone 16 SIMULATOR_EXTENDED_DISPLAY_PROPERTIES=/Users/john/Library/Developer/CoreSimulator/Devices/D635CBF3-87C8-46AC-9F23-808E3ED2A6A0/data/Library/Application Support/Simulator/extended_display.plist SIMULATOR_MAINSCREEN_SCALE=3.000000 SIMULATOR_UDID=D635CBF3-87C8-46AC-9F23-808E3ED2A6A0 SIMULATOR_VERSION_INFO=CoreSimulator 1010.15 - Device: iPhone 16 (D635CBF3-87C8-46AC-9F23-808E3ED2A6A0) - Runtime: iOS 18.6 (22G86) - DeviceType: iPhone 16 VM_KERNEL_PAGE_SIZE_4K=1 MJPEG_SERVER_PORT=10666 TERM=xterm-256color USE_PORT=10500 CFFIXED_USER_HOME=/Users/john/Library/Developer/CoreSimulator/Devices/D635CBF3-87C8-46AC-9F23-808E3ED2A6A0/data/Containers/Data/Application/D211AE61-BCAB-4E9B-B271-76002C7A0313 HOME=/Users/john/Library/Developer/CoreSimulator/Devices/D635CBF3-87C8-46AC-9F23-808E3ED2A6A0/data/Containers/Data/Application/D211AE61-BCAB-4E9B-B271-76002C7A0313 TMPDIR=/Users/john/Library/Developer/CoreSimulator/Devices/D635CBF3-87C8-46AC-9F23-808E3ED2A6A0/data/Containers/Data/Application/D211AE61-BCAB-4E9B-B271-76002C7A0313/tmp XPC_SERVICE_NAME=UIKitApplication:com.facebook.WebDriverAgentRunner.xctrunner[097a][rb-legacy] SIMULATOR_CAPABILITIES_HIDE_HOME_INDICATOR=1 TESTMANAGERD_REMOTE_AUTOMATION_SIM_SOCK=/private/tmp/com.apple.launchd.zo3BwJ1B3K/com.apple.testmanagerd.remote-automation.unix-domain.socket TESTMANAGERD_SIM_SOCK=/private/tmp/com.apple.launchd.qkIQ5Kp9kk/com.apple.testmanagerd.unix-domain.socket RWI_LISTEN_SOCKET=/private/tmp/com.apple.launchd.9wJeg2LKr8/com.apple.webinspectord_sim.socket DYLD_FALLBACK_FRAMEWORK_PATH=/Library/Developer/CoreSimulator/Volumes/iOS_22G86/Library/Developer/CoreSimulator/Profiles/Runtimes/iOS 18.6.simruntime/Contents/Resources/RuntimeRoot/System/Library/Frameworks DYLD_FALLBACK_LIBRARY_PATH=/Library/Developer/CoreSimulator/Volumes/iOS_22G86/Library/Developer/CoreSimulator/Profiles/Runtimes/iOS 18.6.simruntime/Contents/Resources/RuntimeRoot/usr/lib XPC_FLAGS=1`

	tests := []struct {
		name      string
		envVar    string
		expected  string
		shouldErr bool
	}{
		{
			name:     "extract USE_PORT",
			envVar:   "USE_PORT",
			expected: "10500",
		},
		{
			name:     "extract MJPEG_SERVER_PORT",
			envVar:   "MJPEG_SERVER_PORT",
			expected: "10666",
		},
		{
			name:     "extract SIMULATOR_DEVICE_NAME with spaces",
			envVar:   "SIMULATOR_DEVICE_NAME",
			expected: "iPhone 16",
		},
		{
			name:     "extract SIMULATOR_MAINSCREEN_SCALE",
			envVar:   "SIMULATOR_MAINSCREEN_SCALE",
			expected: "3.000000",
		},
		{
			name:     "extract TERM",
			envVar:   "TERM",
			expected: "xterm-256color",
		},
		{
			name:     "extract XPC_FLAGS at end",
			envVar:   "XPC_FLAGS",
			expected: "1",
		},
		{
			name:     "extract SIMULATOR_VERSION_INFO with complex value",
			envVar:   "SIMULATOR_VERSION_INFO",
			expected: "CoreSimulator 1010.15 - Device: iPhone 16 (D635CBF3-87C8-46AC-9F23-808E3ED2A6A0) - Runtime: iOS 18.6 (22G86) - DeviceType: iPhone 16",
		},
		{
			name:      "nonexistent variable",
			envVar:    "NONEXISTENT_VAR",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractEnvValue(processInfo, tt.envVar)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("expected error for %s, but got none", tt.envVar)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for %s: %v", tt.envVar, err)
				return
			}

			if result != tt.expected {
				t.Errorf("extractEnvValue(%s) = %q, want %q", tt.envVar, result, tt.expected)
			}
		})
	}
}
