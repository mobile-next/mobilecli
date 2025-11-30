package devices

import "testing"

func Test_parseDisplayIdFromCmdDisplay(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name: "single display ON",
			output: `Displays:
Display id 0: DisplayInfo{"Built-in Screen", displayId 0, state ON, type INTERNAL, uniqueId "local:4619827259835644672", app 2076 x 2152}`,
			want: "4619827259835644672",
		},
		{
			name: "two displays first ON",
			output: `Displays:
Display id 0: DisplayInfo{"Built-in Screen", displayId 0, state ON, type INTERNAL, uniqueId "local:4619827259835644672", app 2076 x 2152}
Display id 3: DisplayInfo{"Second Screen", displayId 3, state OFF, type INTERNAL, uniqueId "local:4619827551948147201", app 1080 x 2424}`,
			want: "4619827259835644672",
		},
		{
			name:   "invalid input",
			output: "some random text",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseDisplayIdFromCmdDisplay(tt.output); got != tt.want {
				t.Errorf("parseDisplayIdFromCmdDisplay() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseDisplayIdFromDumpsysViewport(t *testing.T) {
	tests := []struct {
		name    string
		dumpsys string
		want    string
	}{
		{
			name: "single display active",
			dumpsys: `DISPLAY MANAGER (dumpsys display)
  mViewports=[DisplayViewport{type=INTERNAL, valid=true, isActive=true, displayId=0, uniqueId='local:4619827259835644672', physicalPort=0}]`,
			want: "4619827259835644672",
		},
		{
			name: "two displays first active",
			dumpsys: `DISPLAY MANAGER (dumpsys display)
  mViewports=[DisplayViewport{type=INTERNAL, valid=true, isActive=true, displayId=0, uniqueId='local:4619827259835644672', physicalPort=0}, DisplayViewport{type=INTERNAL, valid=true, isActive=false, displayId=3, uniqueId='local:4619827551948147201', physicalPort=1}]`,
			want: "4619827259835644672",
		},
		{
			name:    "invalid input",
			dumpsys: "some random text",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseDisplayIdFromDumpsysViewport(tt.dumpsys); got != tt.want {
				t.Errorf("parseDisplayIdFromDumpsysViewport() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseDisplayIdFromDumpsysState(t *testing.T) {
	tests := []struct {
		name    string
		dumpsys string
		want    string
	}{
		{
			name: "single display ON",
			dumpsys: `Display States: size=1
  Display Id=0
  Display State=ON`,
			want: "0",
		},
		{
			name: "two displays first ON",
			dumpsys: `Display States: size=2
  Display Id=0
  Display State=ON
  Display Id=3
  Display State=OFF`,
			want: "0",
		},
		{
			name:    "invalid input",
			dumpsys: "some random text",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseDisplayIdFromDumpsysState(tt.dumpsys); got != tt.want {
				t.Errorf("parseDisplayIdFromDumpsysState() = %v, want %v", got, tt.want)
			}
		})
	}
}
