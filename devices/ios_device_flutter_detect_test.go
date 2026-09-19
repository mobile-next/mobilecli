package devices

import (
	"testing"

	"github.com/danielpaulus/go-ios/ios/installationproxy"
)

// shapes as installation_proxy returns them: plist arrays decode to []any.
func appAdvertising(services ...any) installationproxy.AppInfo {
	return installationproxy.AppInfo{"CFBundleIdentifier": "com.example.app", "NSBonjourServices": services}
}

func TestFlutterDebugAndProfileBuildsAdvertiseTheDartVMService(t *testing.T) {
	if !advertisesDartVMService(appAdvertising("_dartVmService._tcp")) {
		t.Error("a current Flutter debug/profile build must be detected")
	}
	if !advertisesDartVMService(appAdvertising("_http._tcp", "_dartobservatory._tcp")) {
		t.Error("an older Flutter build (observatory service name) must be detected among other services")
	}
}

func TestAppsWithoutADartVMServiceAreNotFlutterCandidates(t *testing.T) {
	native := installationproxy.AppInfo{"CFBundleIdentifier": "com.mobilenext.devicekit-h264"}
	if advertisesDartVMService(native) {
		t.Error("an app with no bonjour services is not a Flutter candidate")
	}
	if advertisesDartVMService(appAdvertising("_airplay._tcp")) {
		t.Error("unrelated bonjour services are not a Flutter candidate")
	}
	malformed := installationproxy.AppInfo{"NSBonjourServices": "_dartVmService._tcp"}
	if advertisesDartVMService(malformed) {
		t.Error("a malformed NSBonjourServices value must not be trusted")
	}
}
