package devices

import "testing"

// displayIdParseCase is one "given this command output, expect this display id" case.
type displayIdParseCase struct {
	name   string
	output string
	want   string
}

// expectDisplayIdParsedFromOutput runs every case through parse and reports mismatches.
func expectDisplayIdParsedFromOutput(t *testing.T, parserName string, parse func(string) string, cases []displayIdParseCase) {
	t.Helper()

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := parse(tt.output); got != tt.want {
				t.Errorf("%s() = %v, want %v", parserName, got, tt.want)
			}
		})
	}
}

func Test_parseDisplayIdFromCmdDisplay(t *testing.T) {
	expectDisplayIdParsedFromOutput(t, "parseDisplayIdFromCmdDisplay", parseDisplayIdFromCmdDisplay, []displayIdParseCase{
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
	})
}

func Test_parseDisplayIdFromDumpsysViewport(t *testing.T) {
	expectDisplayIdParsedFromOutput(t, "parseDisplayIdFromDumpsysViewport", parseDisplayIdFromDumpsysViewport, []displayIdParseCase{
		{
			name: "single display active",
			output: `DISPLAY MANAGER (dumpsys display)
  mViewports=[DisplayViewport{type=INTERNAL, valid=true, isActive=true, displayId=0, uniqueId='local:4619827259835644672', physicalPort=0}]`,
			want: "4619827259835644672",
		},
		{
			name: "two displays first active",
			output: `DISPLAY MANAGER (dumpsys display)
  mViewports=[DisplayViewport{type=INTERNAL, valid=true, isActive=true, displayId=0, uniqueId='local:4619827259835644672', physicalPort=0}, DisplayViewport{type=INTERNAL, valid=true, isActive=false, displayId=3, uniqueId='local:4619827551948147201', physicalPort=1}]`,
			want: "4619827259835644672",
		},
		{
			name:   "invalid input",
			output: "some random text",
			want:   "",
		},
	})
}

func Test_parseDisplayIdFromDumpsysState(t *testing.T) {
	expectDisplayIdParsedFromOutput(t, "parseDisplayIdFromDumpsysState", parseDisplayIdFromDumpsysState, []displayIdParseCase{
		{
			name: "single display ON",
			output: `Display States: size=1
  Display Id=0
  Display State=ON`,
			want: "0",
		},
		{
			name: "two displays first ON",
			output: `Display States: size=2
  Display Id=0
  Display State=ON
  Display Id=3
  Display State=OFF`,
			want: "0",
		},
		{
			name:   "invalid input",
			output: "some random text",
			want:   "",
		},
	})
}

// rotationParseCase is one "given this command output, expect this rotation" case.
type rotationParseCase struct {
	name     string
	output   string
	want     int
	wantRead bool
}

func expectRotationParsedFromOutput(t *testing.T, parserName string, parse func(string) (int, bool), cases []rotationParseCase) {
	t.Helper()

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, read := parse(tt.output)
			if got != tt.want || read != tt.wantRead {
				t.Errorf("%s() = %v, %v, want %v, %v", parserName, got, read, tt.want, tt.wantRead)
			}
		})
	}
}

func Test_parseRotationFromCmdDisplay(t *testing.T) {
	expectRotationParsedFromOutput(t, "parseRotationFromCmdDisplay", parseRotationFromCmdDisplay, []rotationParseCase{
		{
			name: "portrait",
			output: `Displays:
Display id 0: DisplayInfo{"Built-in Screen", displayId 0, displayGroupId 0, real 1080 x 2400, largest app 2400 x 2400, smallest app 1080 x 1080, rotation 0, state ON, committedState ON, type INTERNAL, uniqueId "local:4619827259835644672", app 1080 x 2400}`,
			want:     0,
			wantRead: true,
		},
		{
			name: "landscape",
			output: `Displays:
Display id 0: DisplayInfo{"Built-in Screen", displayId 0, displayGroupId 0, real 2400 x 1080, largest app 2400 x 2400, smallest app 1080 x 1080, rotation 1, state ON, committedState ON, type INTERNAL, uniqueId "local:4619827259835644672", app 2400 x 1080}`,
			want:     1,
			wantRead: true,
		},
		{
			name: "android 13 indents each display",
			output: `Displays:
  Display id 0: DisplayInfo{"Built-in Screen", displayId 0, real 2400 x 1080, rotation 1, state ON, type INTERNAL, uniqueId "local:4619827259835644672"}`,
			want:     1,
			wantRead: true,
		},
		{
			name:     "after wm size, in one shell",
			output:   "Physical size: 1080x2400\r\nDisplays:\r\nDisplay id 0: DisplayInfo{\"Built-in Screen\", displayId 0, real 2400 x 1080, rotation 3, state ON, type INTERNAL}\r\n",
			want:     3,
			wantRead: true,
		},
		{
			name: "screen off",
			output: `Displays:
Display id 0: DisplayInfo{"Built-in Screen", displayId 0, real 1080 x 2400, rotation 1, state OFF, type INTERNAL}`,
			want:     1,
			wantRead: true,
		},
		{
			name:   "no get-displays before android 13",
			output: "Physical size: 1080x2400\n",
		},
		{
			name: "folded, the default display wm size measures wins over the one on",
			output: `Displays:
Display id 3: DisplayInfo{"Second Screen", displayId 3, real 2424 x 1080, rotation 3, state ON, type INTERNAL, uniqueId "local:4619827551948147201"}
Display id 0: DisplayInfo{"Built-in Screen", displayId 0, real 2076 x 2152, rotation 0, state OFF, type INTERNAL, uniqueId "local:4619827259835644672"}`,
			want:     0,
			wantRead: true,
		},
		{
			name: "display 10 is not display 0",
			output: `Displays:
Display id 10: DisplayInfo{"Virtual", displayId 10, real 1920 x 1080, rotation 1, state ON, type VIRTUAL}`,
		},
		{
			name:   "invalid input",
			output: "some random text",
		},
	})
}

func Test_parseRotationFromDumpsysDisplay(t *testing.T) {
	expectRotationParsedFromOutput(t, "parseRotationFromDumpsysDisplay", parseRotationFromDumpsysDisplay, []rotationParseCase{
		{
			name: "override follows the display, base stays natural",
			output: `  Logical Displays: size=1
  Display 0:
    mDisplayId=0
    mBaseDisplayInfo=DisplayInfo{"Built-in Screen", displayId 0, real 1080 x 2400, rotation 0, state ON, type INTERNAL}
    mOverrideDisplayInfo=DisplayInfo{"Built-in Screen", displayId 0, real 2400 x 1080, rotation 1, state ON, type INTERNAL}`,
			want:     1,
			wantRead: true,
		},
		{
			name: "android 10 shape",
			output: `  Display 0:
    mBaseDisplayInfo=DisplayInfo{"Built-in Screen, displayId 0", uniqueId "local:0", app 1080 x 2400, real 1080 x 2400, largest app 2400 x 2337, smallest app 1080 x 1017, mode 1, defaultMode 1, rotation 0, density 420 (420.0 x 420.0) dpi, layerStack 0, state ON}
    mOverrideDisplayInfo=DisplayInfo{"Built-in Screen, displayId 0", uniqueId "local:0", app 2400 x 1080, real 2400 x 1080, largest app 2400 x 2337, smallest app 1080 x 1017, mode 1, defaultMode 1, rotation 1, density 420 (420.0 x 420.0) dpi, layerStack 0, state ON}`,
			want:     1,
			wantRead: true,
		},
		{
			name:   "invalid input",
			output: "some random text",
		},
	})
}

func Test_parseWmSize(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		width, height int
		wantErr       bool
	}{
		{name: "physical", output: "Physical size: 1080x2400\n", width: 1080, height: 2400},
		{name: "crlf", output: "Physical size: 1080x2400\r\n", width: 1080, height: 2400},
		{name: "override wins", output: "Physical size: 1080x2400\nOverride size: 720x1600\n", width: 720, height: 1600},
		{name: "followed by get-displays", output: "Physical size: 1080x2400\nDisplays:\nDisplay id 0: DisplayInfo{\"Built-in Screen\", real 1080 x 2400, rotation 0, state ON}\n", width: 1080, height: 2400},
		{name: "garbage", output: "error: no devices", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width, height, err := parseWmSize(tt.output)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseWmSize() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && (width != tt.width || height != tt.height) {
				t.Errorf("parseWmSize() = %dx%d, want %dx%d", width, height, tt.width, tt.height)
			}
		})
	}
}
