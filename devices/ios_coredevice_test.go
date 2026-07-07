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
