package devices

import "testing"

func TestSplitAdbDevicesLine(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		serial string
		state  string
		ok     bool
	}{
		{"usb device", "R5CT1234ABC\tdevice", "R5CT1234ABC", "device", true},
		{"emulator", "emulator-5554\tdevice", "emulator-5554", "device", true},
		{"tcp device", "192.168.1.20:5555\toffline", "192.168.1.20:5555", "offline", true},
		{"wireless debugging", "adb-R5CT1234ABC-L0NBuy._adb-tls-connect._tcp\tdevice", "adb-R5CT1234ABC-L0NBuy._adb-tls-connect._tcp", "device", true},
		{"wireless debugging after an mDNS name conflict", "adb-R5CT1234ABC-L0NBuy (2)._adb-tls-connect._tcp\tdevice", "adb-R5CT1234ABC-L0NBuy (2)._adb-tls-connect._tcp", "device", true},
		{"state with a space", "R5CT1234ABC\tno permissions", "R5CT1234ABC", "no permissions", true},
		{"windows line ending", "R5CT1234ABC\tdevice\r", "R5CT1234ABC", "device", true},
		{"header", "List of devices attached", "", "", false},
		{"daemon start message", "* daemon started successfully", "", "", false},
		{"empty line", "", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serial, state, ok := splitAdbDevicesLine(tt.line)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if serial != tt.serial || state != tt.state {
				t.Fatalf("got (%q, %q), want (%q, %q)", serial, state, tt.serial, tt.state)
			}
		})
	}
}
