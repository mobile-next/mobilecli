package devices

import "testing"

func TestIOSDevicePlatform(t *testing.T) {
	// Real Apple TV units are discovered over the same go-ios path as iPhones/iPads
	// and are identified by their product type rather than a separate device class.
	cases := []struct {
		productType string
		want        string
	}{
		{"iPhone15,3", "ios"},
		{"iPad13,1", "ios"},
		{"AppleTV14,1", "tvos"},
		{"AppleTV11,1", "tvos"},
		{"", "ios"},
	}
	for _, c := range cases {
		d := IOSDevice{ProductType: c.productType}
		if got := d.Platform(); got != c.want {
			t.Errorf("IOSDevice{ProductType: %q}.Platform() = %q, want %q", c.productType, got, c.want)
		}
	}
}

func TestIOSDeviceType(t *testing.T) {
	d := IOSDevice{ProductType: "AppleTV14,1"}
	if got := d.DeviceType(); got != "real" {
		t.Errorf("IOSDevice.DeviceType() = %q, want %q", got, "real")
	}
}

func TestIOSDeviceState(t *testing.T) {
	// Non-CoreDevice devices default to "online".
	def := IOSDevice{ProductType: "AppleTV14,1"}
	if got := def.State(); got != "online" {
		t.Errorf("default IOSDevice.State() = %q, want %q", got, "online")
	}

	// CoreDevice-discovered devices report their cached state.
	offline := IOSDevice{ProductType: "AppleTV14,1", coreDeviceState: "offline"}
	if got := offline.State(); got != "offline" {
		t.Errorf("offline IOSDevice.State() = %q, want %q", got, "offline")
	}

	online := IOSDevice{ProductType: "AppleTV14,1", coreDeviceState: "online"}
	if got := online.State(); got != "online" {
		t.Errorf("online IOSDevice.State() = %q, want %q", got, "online")
	}
}

func TestTVOSRunnerAction(t *testing.T) {
	cases := []struct {
		healthy         bool
		hasOwnedProcess bool
		want            string
	}{
		{true, false, "reuse"},
		{true, true, "reuse"},
		{false, true, "restart"},
		{false, false, "launch"},
	}
	for _, c := range cases {
		if got := tvosRunnerAction(c.healthy, c.hasOwnedProcess); got != c.want {
			t.Errorf("tvosRunnerAction(%v, %v) = %q, want %q", c.healthy, c.hasOwnedProcess, got, c.want)
		}
	}
}

func TestTVOSTunnelBaseURL(t *testing.T) {
	if got := tvosTunnelBaseURL("10.0.0.5", 12004); got != "http://10.0.0.5:12004" {
		t.Errorf("tvosTunnelBaseURL ipv4 = %q", got)
	}
	// IPv6 tunnel addresses must be bracketed.
	if got := tvosTunnelBaseURL("fd7a:1234::1", 12004); got != "http://[fd7a:1234::1]:12004" {
		t.Errorf("tvosTunnelBaseURL ipv6 = %q", got)
	}
}
