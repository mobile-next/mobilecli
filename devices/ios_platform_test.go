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
