package devices

import "testing"

func Test_parseResolveActivityOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name: "brief output with launcher",
			output: `priority=0 preferredOrder=0 match=0x108000 specificIndex=-1 isDefault=false
com.android.settings/.Settings`,
			want: "com.android.settings/.Settings",
		},
		{
			name: "fully qualified activity class",
			output: `priority=0 preferredOrder=0 match=0x108000 specificIndex=-1 isDefault=false
com.example.app/com.example.app.MainActivity`,
			want: "com.example.app/com.example.app.MainActivity",
		},
		{
			name: "activity with inner class",
			output: `priority=0 preferredOrder=0 match=0x108000 specificIndex=-1 isDefault=false
com.example/.Outer$Inner`,
			want: "com.example/.Outer$Inner",
		},
		{
			name: "no launcher activity",
			output: `priority=0 preferredOrder=0 match=0x0 specificIndex=-1 isDefault=false
{}`,
			want: "",
		},
		{
			name:   "empty output",
			output: "",
			want:   "",
		},
		{
			name:   "no activity found message",
			output: "No activity found",
			want:   "",
		},
		{
			name: "extra whitespace around component",
			output: `priority=0 preferredOrder=0 match=0x108000 specificIndex=-1 isDefault=false
   com.android.settings/.Settings   `,
			want: "com.android.settings/.Settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseResolveActivityOutput(tt.output)
			if got != tt.want {
				t.Fatalf("parseResolveActivityOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_buildLaunchComponent(t *testing.T) {
	tests := []struct {
		name     string
		bundleID string
		activity string
		want     string
		wantErr  bool
	}{
		{
			name:     "relative activity gets package prefix",
			bundleID: "com.example.app",
			activity: ".DebugActivity",
			want:     "com.example.app/.DebugActivity",
		},
		{
			name:     "fully qualified class gets package prefix",
			bundleID: "com.example.app",
			activity: "com.example.app.DebugActivity",
			want:     "com.example.app/com.example.app.DebugActivity",
		},
		{
			name:     "full component is used as-is",
			bundleID: "com.example.app",
			activity: "com.other/.DebugActivity",
			want:     "com.other/.DebugActivity",
		},
		{
			name:     "inner class is allowed",
			bundleID: "com.example.app",
			activity: ".Outer$Inner",
			want:     "com.example.app/.Outer$Inner",
		},
		{
			name:     "activity with space is rejected",
			bundleID: "com.example.app",
			activity: ".Debug Activity",
			wantErr:  true,
		},
		{
			name:     "activity with shell metacharacter is rejected",
			bundleID: "com.example.app",
			activity: ".Debug;rm",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildLaunchComponent(tt.bundleID, tt.activity)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("buildLaunchComponent() expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildLaunchComponent() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("buildLaunchComponent() = %q, want %q", got, tt.want)
			}
		})
	}
}
