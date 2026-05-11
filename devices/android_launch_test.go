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
