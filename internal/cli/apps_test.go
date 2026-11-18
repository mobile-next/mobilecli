package cli

import (
	"reflect"
	"testing"
)

func Test_parseLaunchEnv(t *testing.T) {
	got, err := parseLaunchEnv([]string{"API_URL=https://x.test/?a=b", "EMPTY="})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"API_URL": "https://x.test/?a=b", "EMPTY": ""}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseLaunchEnv() = %v, want %v", got, want)
	}

	for _, bad := range []string{"NOEQUALS", "=value"} {
		if _, err := parseLaunchEnv([]string{bad}); err == nil {
			t.Errorf("parseLaunchEnv(%q): expected error, got nil", bad)
		}
	}
}
