package devices

import "testing"

func Test_parseInputShown(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    bool
		wantErr bool
	}{
		{
			name:   "alone on its own line, true",
			output: "      mInputShown=true",
			want:   true,
		},
		{
			name:   "alone on its own line, false",
			output: "      mInputShown=false",
			want:   false,
		},
		{
			name:   "packed after another field on the same line",
			output: "      mRequestedShowExplicitly=false mInputShown=true",
			want:   true,
		},
		{
			name:   "packed before another field on the same line",
			output: "      mInputShown=true mLastImeTargetWindow=android.os.BinderProxy@5d1074c",
			want:   true,
		},
		{
			name:    "invalid value",
			output:  "      mInputShown=maybe",
			wantErr: true,
		},
		{
			name:    "field not present",
			output:  "some random text\nmSystemReady=true",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseInputShown(tt.output)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseInputShown() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got != tt.want {
				t.Errorf("parseInputShown() = %v, want %v", got, tt.want)
			}
		})
	}
}
