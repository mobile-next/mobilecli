package devices

import "testing"

func Test_amStartError(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "activity started",
			output: "Starting: Intent { act=android.intent.action.VIEW dat=https://example.com/... }\n",
			want:   "",
		},
		{
			name: "intent delivered to an activity that is already running",
			output: "Starting: Intent { act=android.intent.action.VIEW dat=https://example.com/... }\n" +
				"Warning: Activity not started, intent has been delivered to currently running top-most instance.\n",
			want: "",
		},
		{
			name: "no app handles the url",
			output: "Starting: Intent { act=android.intent.action.VIEW dat=notascheme://foo/... }\n" +
				"Error: Activity not started, unable to resolve Intent { act=android.intent.action.VIEW dat=notascheme://foo/... flg=0x10000000 }\n",
			want: "Activity not started, unable to resolve Intent { act=android.intent.action.VIEW dat=notascheme://foo/... flg=0x10000000 }",
		},
		{
			name: "error type line before the reason",
			output: "Starting: Intent { cmp=com.example/.Missing }\n" +
				"Error type 3\n" +
				"Error: Activity class {com.example/com.example.Missing} does not exist.\n",
			want: "Activity class {com.example/com.example.Missing} does not exist.",
		},
		{
			name:   "windows line endings",
			output: "Starting: Intent { act=android.intent.action.VIEW }\r\nError: Activity not started\r\n",
			want:   "Activity not started",
		},
		{
			name:   "a url that only mentions an error",
			output: "Starting: Intent { act=android.intent.action.VIEW dat=https://example.com/Error: }\n",
			want:   "",
		},
		{
			name:   "empty output",
			output: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := amStartError(tt.output)
			if got != tt.want {
				t.Fatalf("amStartError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_urlHasScheme(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{url: "https://example.com", want: true},
		{url: "tel:123", want: true},
		{url: "myapp://path", want: true},
		{url: "example.com", want: false},
		// a host with a port reads as a scheme, like tel:123 does; am rejects it
		{url: "localhost:3000", want: true},
		{url: "example.com/path?next=https://other.example", want: false},
		{url: "//example.com", want: false},
		{url: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := urlHasScheme(tt.url)
			if got != tt.want {
				t.Fatalf("urlHasScheme(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
