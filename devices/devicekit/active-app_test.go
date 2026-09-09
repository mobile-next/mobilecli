package devicekit

import (
	"encoding/json"
	"testing"
)

func TestActiveAppInfoReadsViewController(t *testing.T) {
	payload := []byte(`{"bundleId":"com.mobilenext.playground","name":"Playground","pid":19917,"viewController":"SwiftUI.NavigationStackHostingController<SwiftUI.AnyView>"}`)

	var info ActiveAppInfo
	if err := json.Unmarshal(payload, &info); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.ViewController != "SwiftUI.NavigationStackHostingController<SwiftUI.AnyView>" {
		t.Errorf("got %q, want the view controller class name", info.ViewController)
	}
}

// An agent older than the view controller support omits the field entirely.
func TestActiveAppInfoWithoutViewControllerIsEmptyNotAnError(t *testing.T) {
	payload := []byte(`{"bundleId":"com.apple.mobilesafari","name":"Safari","pid":48787}`)

	var info ActiveAppInfo
	if err := json.Unmarshal(payload, &info); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.ViewController != "" {
		t.Errorf("got %q, want an empty view controller", info.ViewController)
	}

	if info.BundleID != "com.apple.mobilesafari" {
		t.Errorf("got %q, want the bundle id to still be parsed", info.BundleID)
	}
}
