package cli

import "testing"

func TestResolveDeviceID(t *testing.T) {
	cases := []struct {
		name string
		flag string
		env  map[string]string
		want string
	}{
		{"--device flag is used", "flag-device", nil, "flag-device"},
		{"--device flag wins over MOBILECLI_DEVICE", "flag-device", map[string]string{"MOBILECLI_DEVICE": "env-device"}, "flag-device"},
		{"MOBILECLI_DEVICE is used without --device", "", map[string]string{"MOBILECLI_DEVICE": "env-device"}, "env-device"},
		{"no device without either", "", nil, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveDeviceID(tc.flag, envWith(tc.env)); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
