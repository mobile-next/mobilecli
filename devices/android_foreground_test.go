package devices

import "testing"

func Test_parseForegroundComponent(t *testing.T) { //nolint:funlen
	tests := []struct {
		name         string
		input        string
		wantPackage  string
		wantActivity string
		wantErr      bool
	}{
		{
			name:         "single display",
			input:        "  mCurrentFocus=Window{9c8e10c u0 com.example.app/com.example.app.MainActivity}",
			wantPackage:  "com.example.app",
			wantActivity: "com.example.app.MainActivity",
		},
		{
			name: "multi display with a null before the focused one",
			input: "  mCurrentFocus=null\n" +
				"  mCurrentFocus=Window{d0ebdc2 u0 com.example.app/com.example.app.MainActivity}",
			wantPackage:  "com.example.app",
			wantActivity: "com.example.app.MainActivity",
		},
		{
			name:    "every display unfocused",
			input:   "  mCurrentFocus=null\n  mCurrentFocus=null",
			wantErr: true,
		},
		{
			name:    "no mCurrentFocus line at all",
			input:   "Display: mDisplayId=0\n  mBaseDisplayWidth=1080",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, activity, err := parseForegroundComponent(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseForegroundComponent() expected an error, got %q/%q", pkg, activity)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseForegroundComponent() error = %v", err)
			}
			if pkg != tt.wantPackage || activity != tt.wantActivity {
				t.Errorf("parseForegroundComponent() = %q/%q, want %q/%q",
					pkg, activity, tt.wantPackage, tt.wantActivity)
			}
		})
	}
}
