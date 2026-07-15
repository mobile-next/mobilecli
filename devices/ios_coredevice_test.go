package devices

import "testing"

func TestParseCoreDevicePhysicalInfos(t *testing.T) {
	data := []byte(`{
		"result": {
			"devices": [
				{
					"connectionProperties": {
						"pairingState": "paired",
						"transportType": "localNetwork",
						"tunnelState": "connected"
					},
					"deviceProperties": {
						"bootState": "booted",
						"name": "iPhone-J7KRQL2Q75",
						"osVersionNumber": "26.5.2"
					},
					"hardwareProperties": {
						"productType": "iPhone17,5",
						"reality": "physical",
						"udid": "00008140-001975D9349B801C"
					}
				},
				{
					"connectionProperties": {
						"pairingState": "paired",
						"transportType": "sameMachine",
						"tunnelState": "disconnected"
					},
					"deviceProperties": {
						"bootState": "booted",
						"name": "UnitTests (iOS)",
						"osVersionNumber": "26.2"
					},
					"hardwareProperties": {
						"productType": "iPhone18,3",
						"reality": "simulated",
						"udid": "56BABF10-C8A0-43F4-93B0-1891E98BE95E"
					}
				}
			]
		}
	}`)

	infos, err := parseCoreDevicePhysicalInfos(data)
	if err != nil {
		t.Fatalf("parseCoreDevicePhysicalInfos returned error: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("expected 1 physical device, got %d: %+v", len(infos), infos)
	}

	info := infos[0]
	if info.UDID != "00008140-001975D9349B801C" {
		t.Fatalf("expected UDID to be preserved, got %q", info.UDID)
	}
	if info.Name != "iPhone-J7KRQL2Q75" {
		t.Fatalf("expected device name to be parsed, got %q", info.Name)
	}
	if info.OSVersion != "26.5.2" {
		t.Fatalf("expected OS version to be parsed, got %q", info.OSVersion)
	}
	if info.ProductType != "iPhone17,5" {
		t.Fatalf("expected product type to be parsed, got %q", info.ProductType)
	}
}

func TestParseCoreDevicePhysicalInfosRetainsAppleTVIdentity(t *testing.T) {
	data := []byte(`{
		"result": {
			"devices": [
				{
					"identifier": "8A2B39A3-F7B6-5EF5-B0AC-9E7D17592953",
					"connectionProperties": {
						"pairingState": "paired",
						"transportType": "localNetwork",
						"tunnelState": "connected",
						"tunnelIPAddress": "fd7a:1234::1"
					},
					"deviceProperties": {
						"bootState": "booted",
						"name": "Bedroom",
						"osVersionNumber": "26.5"
					},
					"hardwareProperties": {
						"productType": "AppleTV14,1",
						"reality": "physical",
						"udid": "bbfebc944c272f42d78ff80b8553655d3f936046"
					}
				}
			]
		}
	}`)

	infos, err := parseCoreDevicePhysicalInfos(data)
	if err != nil {
		t.Fatalf("parseCoreDevicePhysicalInfos returned error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 physical Apple TV, got %d: %+v", len(infos), infos)
	}

	info := infos[0]
	if info.Identifier != "8A2B39A3-F7B6-5EF5-B0AC-9E7D17592953" {
		t.Errorf("expected CoreDevice identifier to be retained, got %q", info.Identifier)
	}
	if info.UDID != "bbfebc944c272f42d78ff80b8553655d3f936046" {
		t.Errorf("expected hardware UDID to be retained, got %q", info.UDID)
	}
	if info.TunnelIP != "fd7a:1234::1" {
		t.Errorf("expected tunnel IP to be retained, got %q", info.TunnelIP)
	}
	if info.BootState != "booted" {
		t.Errorf("expected boot state to be parsed, got %q", info.BootState)
	}
	if info.TunnelState != "connected" {
		t.Errorf("expected tunnel state to be parsed, got %q", info.TunnelState)
	}
}

func TestCoreDeviceIDResolution(t *testing.T) {
	withIdentifier := IOSDevice{Udid: "hardware-udid", CoreDeviceIdentifier: "core-id"}
	if got := withIdentifier.coreDeviceID(); got != "core-id" {
		t.Errorf("expected CoreDevice identifier, got %q", got)
	}
	// ID() must keep returning the public hardware UDID.
	if got := withIdentifier.ID(); got != "hardware-udid" {
		t.Errorf("expected ID() to return hardware UDID, got %q", got)
	}

	withoutIdentifier := IOSDevice{Udid: "hardware-udid"}
	if got := withoutIdentifier.coreDeviceID(); got != "hardware-udid" {
		t.Errorf("expected fallback to hardware UDID, got %q", got)
	}
}

func TestDeriveCoreDeviceState(t *testing.T) {
	cases := []struct {
		boot   string
		tunnel string
		want   string
	}{
		{"booted", "connected", "online"},
		{"booted", "disconnected", "online"},
		{"notBooted", "connected", "online"},
		{"notBooted", "disconnected", "offline"},
		{"", "", "offline"},
	}
	for _, c := range cases {
		if got := deriveCoreDeviceState(c.boot, c.tunnel); got != c.want {
			t.Errorf("deriveCoreDeviceState(%q, %q) = %q, want %q", c.boot, c.tunnel, got, c.want)
		}
	}
}

func TestNormalizeDevicectlPath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"file:///private/var/containers/Bundle/Application/AAA/Demo.app", "/private/var/containers/Bundle/Application/AAA/Demo.app"},
		{"  file:///var/Demo.app/  ", "/var/Demo.app"},
		{"/var/Demo.app/", "/var/Demo.app"},
		{"", ""},
		{"file://", ""},
	}
	for _, c := range cases {
		if got := normalizeDevicectlPath(c.in); got != c.want {
			t.Errorf("normalizeDevicectlPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMatchesBundlePathRejectsSiblingPrefix(t *testing.T) {
	bundlePath := "/var/containers/Bundle/Application/AAA/Foo.app"
	cases := []struct {
		exe  string
		want bool
	}{
		// executable inside the bundle matches
		{"/var/containers/Bundle/Application/AAA/Foo.app/Foo", true},
		// exact bundle path matches
		{bundlePath, true},
		// sibling bundle sharing a path prefix must NOT match
		{"/var/containers/Bundle/Application/AAA/FooBar.app/FooBar", false},
		// unrelated path
		{"/var/containers/Bundle/Application/BBB/Baz.app/Baz", false},
		// empty inputs never match
		{"", false},
	}
	for _, c := range cases {
		if got := matchesBundlePath(c.exe, bundlePath); got != c.want {
			t.Errorf("matchesBundlePath(%q, %q) = %v, want %v", c.exe, bundlePath, got, c.want)
		}
	}
	if matchesBundlePath("/var/Foo.app/Foo", "") {
		t.Error("expected empty bundlePath to never match")
	}
}

func TestParseCoreDeviceAppBundlePath(t *testing.T) {
	data := []byte(`{
		"result": {
			"apps": [
				{"bundleIdentifier": "com.example.other", "url": "file:///var/Other.app/"},
				{"bundleIdentifier": "com.example.demo", "url": "file:///var/containers/Bundle/Application/AAA/Demo.app/"}
			]
		}
	}`)

	path, err := parseCoreDeviceAppBundlePath(data, "device-1", "com.example.demo")
	if err != nil {
		t.Fatalf("parseCoreDeviceAppBundlePath returned error: %v", err)
	}
	if path != "/var/containers/Bundle/Application/AAA/Demo.app" {
		t.Errorf("unexpected bundle path %q", path)
	}
}

func TestParseCoreDeviceAppBundlePathFallsBackToPath(t *testing.T) {
	// url missing/empty must fall back to path without panicking.
	data := []byte(`{
		"result": {
			"apps": [
				{"bundleIdentifier": "com.example.demo", "path": "/var/containers/Bundle/Application/AAA/Demo.app"}
			]
		}
	}`)

	path, err := parseCoreDeviceAppBundlePath(data, "device-1", "com.example.demo")
	if err != nil {
		t.Fatalf("parseCoreDeviceAppBundlePath returned error: %v", err)
	}
	if path != "/var/containers/Bundle/Application/AAA/Demo.app" {
		t.Errorf("expected fallback to path field, got %q", path)
	}
}

func TestParseCoreDeviceAppBundlePathMissingBundle(t *testing.T) {
	data := []byte(`{"result": {"apps": [{"bundleIdentifier": "com.example.other", "url": "file:///var/Other.app"}]}}`)

	if _, err := parseCoreDeviceAppBundlePath(data, "device-1", "com.example.demo"); err == nil {
		t.Fatal("expected error for missing bundle, got nil")
	}
}

func TestParseCoreDevicePIDMatchingProcess(t *testing.T) {
	data := []byte(`{
		"result": {
			"runningProcesses": [
				{"processIdentifier": 101, "executable": "file:///usr/libexec/other"},
				{"processIdentifier": 202, "executable": "file:///var/containers/Bundle/Application/AAA/Demo.app/Demo"}
			]
		}
	}`)

	pid, err := parseCoreDevicePID(data, "device-1", "com.example.demo", "/var/containers/Bundle/Application/AAA/Demo.app")
	if err != nil {
		t.Fatalf("parseCoreDevicePID returned error: %v", err)
	}
	if pid != 202 {
		t.Errorf("expected pid 202, got %d", pid)
	}
}

func TestParseCoreDevicePIDNoMatch(t *testing.T) {
	// A sibling bundle sharing a prefix must not be matched, yielding a clean error.
	data := []byte(`{
		"result": {
			"runningProcesses": [
				{"processIdentifier": 303, "executable": "file:///var/containers/Bundle/Application/AAA/DemoTests.app/DemoTests"}
			]
		}
	}`)

	if _, err := parseCoreDevicePID(data, "device-1", "com.example.demo", "/var/containers/Bundle/Application/AAA/Demo.app"); err == nil {
		t.Fatal("expected error when no process matches, got nil")
	}
}

func TestParseCoreDevicePIDMissingExecutable(t *testing.T) {
	// Missing executable field must not panic and must yield a clean error.
	data := []byte(`{"result": {"runningProcesses": [{"processIdentifier": 404}]}}`)

	if _, err := parseCoreDevicePID(data, "device-1", "com.example.demo", "/var/containers/Bundle/Application/AAA/Demo.app"); err == nil {
		t.Fatal("expected error when executable is missing, got nil")
	}
}
