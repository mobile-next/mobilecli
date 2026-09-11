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
