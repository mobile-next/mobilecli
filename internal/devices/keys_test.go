package devices

import (
	"encoding/json"
	"testing"
)

// The iOS agent rejects "modifiers": null, so a combo built without modifiers must send [].
func TestToDeviceKitKeyCombosSendsEmptyModifiersInsteadOfNull(t *testing.T) {
	payload, err := json.Marshal(toDeviceKitKeyCombos([]KeyCombo{{Key: "clear"}}))
	if err != nil {
		t.Fatal(err)
	}

	want := `[{"key":"clear","modifiers":[]}]`
	if string(payload) != want {
		t.Fatalf("toDeviceKitKeyCombos() = %s, want %s", payload, want)
	}
}
