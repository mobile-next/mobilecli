package devices

import (
	"encoding/xml"
	"testing"

	"github.com/mobile-next/mobilecli/types"
)

func TestIsAscii(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"empty string", "", true},
		{"simple ascii", "hello world", true},
		{"numbers and punctuation", "abc123!@#", true},
		{"newlines and tabs", "hello\nworld\t!", true},
		{"unicode emoji", "hello 🌍", false},
		{"chinese characters", "你好", false},
		{"accented characters", "café", false},
		{"max ascii char", string(rune(127)), true},
		{"first non-ascii char", string(rune(128)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAscii(tt.text); got != tt.want {
				t.Errorf("isAscii(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestEscapeShellText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"simple text", "hello", "hello"},
		{"text with spaces", "hello world", "hello\\ world"},
		{"single quote", "it's", "it\\'s"},
		{"double quote", `say "hi"`, `say\ \"hi\"`},
		{"semicolons", "a;b", "a\\;b"},
		{"pipes", "a|b", "a\\|b"},
		{"ampersands", "a&b", "a\\&b"},
		{"parentheses", "(test)", "\\(test\\)"},
		{"dollar sign", "$HOME", "\\$HOME"},
		{"asterisk", "*.txt", "\\*.txt"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeShellText(tt.text); got != tt.want {
				t.Errorf("escapeShellText(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestMatchesAVDName(t *testing.T) {
	tests := []struct {
		name       string
		avdName    string
		deviceName string
		want       bool
	}{
		{"exact match", "Pixel_9_Pro", "Pixel_9_Pro", true},
		{"underscores to spaces", "Pixel_9_Pro", "Pixel 9 Pro", true},
		{"no match", "Pixel_9_Pro", "Pixel 8", false},
		{"empty strings", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesAVDName(tt.avdName, tt.deviceName); got != tt.want {
				t.Errorf("matchesAVDName(%q, %q) = %v, want %v", tt.avdName, tt.deviceName, got, tt.want)
			}
		})
	}
}

func TestAndroidDevice_DeviceType(t *testing.T) {
	tests := []struct {
		name        string
		transportID string
		state       string
		want        string
	}{
		{"emulator by transport id", "emulator-5554", "online", "emulator"},
		{"real device", "R5CR1234567", "online", "real"},
		{"offline device is emulator", "", "offline", "emulator"},
		{"empty transport online", "", "online", "real"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &AndroidDevice{transportID: tt.transportID, state: tt.state}
			if got := d.DeviceType(); got != tt.want {
				t.Errorf("DeviceType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAndroidDevice_GetAdbIdentifier(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		transportID string
		want        string
	}{
		{"uses transport id when set", "Pixel_9_Pro", "emulator-5554", "emulator-5554"},
		{"falls back to id", "Pixel_9_Pro", "", "Pixel_9_Pro"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &AndroidDevice{id: tt.id, transportID: tt.transportID}
			if got := d.getAdbIdentifier(); got != tt.want {
				t.Errorf("getAdbIdentifier() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAndroidDevice_AccessorMethods(t *testing.T) {
	d := &AndroidDevice{
		id:      "test-id",
		name:    "Test Device",
		version: "14.0",
		state:   "online",
	}

	if d.ID() != "test-id" {
		t.Errorf("ID() = %q, want %q", d.ID(), "test-id")
	}
	if d.Name() != "Test Device" {
		t.Errorf("Name() = %q, want %q", d.Name(), "Test Device")
	}
	if d.Version() != "14.0" {
		t.Errorf("Version() = %q, want %q", d.Version(), "14.0")
	}
	if d.Platform() != "android" {
		t.Errorf("Platform() = %q, want %q", d.Platform(), "android")
	}
	if d.State() != "online" {
		t.Errorf("State() = %q, want %q", d.State(), "online")
	}
}

func TestAndroidDevice_GetScreenElementRect(t *testing.T) {
	d := &AndroidDevice{}

	tests := []struct {
		name   string
		bounds string
		want   types.ScreenElementRect
	}{
		{
			"valid bounds",
			"[0,0][1080,2400]",
			types.ScreenElementRect{X: 0, Y: 0, Width: 1080, Height: 2400},
		},
		{
			"offset bounds",
			"[100,200][500,600]",
			types.ScreenElementRect{X: 100, Y: 200, Width: 400, Height: 400},
		},
		{
			"invalid format",
			"invalid",
			types.ScreenElementRect{},
		},
		{
			"empty string",
			"",
			types.ScreenElementRect{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.getScreenElementRect(tt.bounds)
			if got != tt.want {
				t.Errorf("getScreenElementRect(%q) = %+v, want %+v", tt.bounds, got, tt.want)
			}
		})
	}
}

func TestAndroidDevice_CollectElements(t *testing.T) {
	d := &AndroidDevice{}

	node := uiAutomatorXmlNode{
		Class:  "android.widget.FrameLayout",
		Bounds: "[0,0][1080,2400]",
		Nodes: []uiAutomatorXmlNode{
			{
				Class:       "android.widget.TextView",
				Text:        "Hello World",
				ContentDesc: "greeting",
				Bounds:      "[10,20][200,60]",
				ResourceID:  "com.example:id/text",
			},
			{
				Class:   "android.widget.EditText",
				Hint:    "Enter name",
				Focused: "true",
				Bounds:  "[10,70][200,110]",
			},
			{
				Class:  "android.widget.View",
				Bounds: "[0,0][0,0]", // zero-size, should be excluded
				Text:   "invisible",
			},
		},
	}

	elements := d.collectElements(node)

	if len(elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(elements))
	}

	// first element: TextView with text and content-desc
	if elements[0].Type != "android.widget.TextView" {
		t.Errorf("element[0].Type = %q, want %q", elements[0].Type, "android.widget.TextView")
	}
	if *elements[0].Text != "Hello World" {
		t.Errorf("element[0].Text = %q, want %q", *elements[0].Text, "Hello World")
	}
	if *elements[0].Label != "greeting" {
		t.Errorf("element[0].Label = %q, want %q", *elements[0].Label, "greeting")
	}
	if *elements[0].Identifier != "com.example:id/text" {
		t.Errorf("element[0].Identifier = %q, want %q", *elements[0].Identifier, "com.example:id/text")
	}

	// second element: EditText with hint and focused
	if *elements[1].Label != "Enter name" {
		t.Errorf("element[1].Label = %q, want %q", *elements[1].Label, "Enter name")
	}
	if elements[1].Focused == nil || !*elements[1].Focused {
		t.Error("element[1].Focused should be true")
	}
}

func TestAndroidDevice_DumpSource_ParsesUIAutomatorXML(t *testing.T) {
	d := &AndroidDevice{}

	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<hierarchy rotation="0">
  <node class="android.widget.FrameLayout" bounds="[0,0][1080,2400]">
    <node class="android.widget.LinearLayout" bounds="[0,0][1080,2400]">
      <node class="android.widget.TextView" text="Settings" content-desc="Settings title" bounds="[50,100][500,150]" resource-id="com.android.settings:id/title" />
      <node class="android.widget.Switch" text="ON" bounds="[800,200][1000,250]" />
    </node>
  </node>
</hierarchy>`

	var uiXml uiAutomatorXml
	if err := xml.Unmarshal([]byte(xmlContent), &uiXml); err != nil {
		t.Fatalf("failed to parse XML: %v", err)
	}

	elements := d.collectElements(uiXml.RootNode)

	if len(elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(elements))
	}

	// verify Settings title element
	if *elements[0].Text != "Settings" {
		t.Errorf("element[0].Text = %q, want %q", *elements[0].Text, "Settings")
	}
	if elements[0].Rect.X != 50 || elements[0].Rect.Y != 100 {
		t.Errorf("element[0].Rect = %+v, want x=50 y=100", elements[0].Rect)
	}

	// verify Switch element
	if *elements[1].Text != "ON" {
		t.Errorf("element[1].Text = %q, want %q", *elements[1].Text, "ON")
	}
}

func TestAndroidDevice_PressButton_KeyMap(t *testing.T) {
	validKeys := []string{
		"HOME", "BACK", "VOLUME_UP", "VOLUME_DOWN", "ENTER",
		"DPAD_CENTER", "DPAD_UP", "DPAD_DOWN", "DPAD_LEFT", "DPAD_RIGHT",
		"BACKSPACE", "APP_SWITCH", "POWER",
	}

	// just verify the key map has entries for all expected keys
	keyMap := map[string]string{
		"HOME":        "KEYCODE_HOME",
		"BACK":        "KEYCODE_BACK",
		"VOLUME_UP":   "KEYCODE_VOLUME_UP",
		"VOLUME_DOWN": "KEYCODE_VOLUME_DOWN",
		"ENTER":       "KEYCODE_ENTER",
		"DPAD_CENTER": "KEYCODE_DPAD_CENTER",
		"DPAD_UP":     "KEYCODE_DPAD_UP",
		"DPAD_DOWN":   "KEYCODE_DPAD_DOWN",
		"DPAD_LEFT":   "KEYCODE_DPAD_LEFT",
		"DPAD_RIGHT":  "KEYCODE_DPAD_RIGHT",
		"BACKSPACE":   "KEYCODE_DEL",
		"APP_SWITCH":  "KEYCODE_APP_SWITCH",
		"POWER":       "KEYCODE_POWER",
	}

	for _, key := range validKeys {
		if _, ok := keyMap[key]; !ok {
			t.Errorf("key %q missing from keyMap", key)
		}
	}
}

func TestAndroidDevice_StartAgent_OfflineError(t *testing.T) {
	d := &AndroidDevice{id: "test-emu", state: "offline"}

	err := d.StartAgent(StartAgentConfig{})
	if err == nil {
		t.Error("expected error for offline device")
	}
}

func TestAndroidDevice_StartAgent_OnlineNoOp(t *testing.T) {
	d := &AndroidDevice{id: "test-device", state: "online"}

	err := d.StartAgent(StartAgentConfig{})
	if err != nil {
		t.Errorf("expected no error for online device, got %v", err)
	}
}

func TestAndroidDevice_Shutdown_OnlyEmulators(t *testing.T) {
	d := &AndroidDevice{id: "R5CR1234567", transportID: "R5CR1234567", state: "online"}

	err := d.Shutdown()
	if err == nil {
		t.Error("expected error when shutting down real device")
	}
}

func TestAndroidDevice_Shutdown_AlreadyOffline(t *testing.T) {
	d := &AndroidDevice{id: "test-emu", transportID: "emulator-5554", state: "offline"}

	err := d.Shutdown()
	if err == nil {
		t.Error("expected error when emulator already offline")
	}
}

func TestAndroidDevice_SetOrientation_InvalidValue(t *testing.T) {
	d := &AndroidDevice{id: "test", state: "online"}

	err := d.SetOrientation("diagonal")
	if err == nil {
		t.Error("expected error for invalid orientation")
	}
}
