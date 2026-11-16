package devices

import (
	"reflect"
	"testing"
)

func Test_simctlLaunchArgs(t *testing.T) {
	tests := []struct {
		name string
		opts LaunchOptions
		want []string
	}{
		{
			name: "no options",
			opts: LaunchOptions{},
			want: []string{"launch", "UDID", "com.example.app"},
		},
		{
			name: "arguments follow the bundle id",
			opts: LaunchOptions{Arguments: []string{"-FeatureX", "YES"}},
			want: []string{"launch", "UDID", "com.example.app", "-FeatureX", "YES"},
		},
		{
			name: "locales come before arguments",
			opts: LaunchOptions{Locales: []string{"fr-FR"}, Arguments: []string{"--uitesting"}},
			want: []string{"launch", "UDID", "com.example.app", "-AppleLanguages", "(fr-FR)", "--uitesting"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := simctlLaunchArgs("UDID", "com.example.app", tt.opts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("simctlLaunchArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_simctlLaunchEnv(t *testing.T) {
	got := simctlLaunchEnv(map[string]string{"B": "2", "A": "x=y", "EMPTY": ""})
	want := []string{"SIMCTL_CHILD_A=x=y", "SIMCTL_CHILD_B=2", "SIMCTL_CHILD_EMPTY="}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("simctlLaunchEnv() = %v, want %v", got, want)
	}

	if got := simctlLaunchEnv(nil); len(got) != 0 {
		t.Errorf("simctlLaunchEnv(nil) = %v, want empty", got)
	}
}

func Test_LaunchApp_EnvAndBareArgumentsRejectedOnAndroid(t *testing.T) {
	device := &AndroidDevice{}

	if err := device.LaunchApp("com.example.app", LaunchOptions{Arguments: []string{"-FeatureX"}}); err == nil {
		t.Fatal("expected error for an argument without '=', got nil")
	}

	if err := device.LaunchApp("com.example.app", LaunchOptions{Env: map[string]string{"A": "1"}}); err == nil {
		t.Fatal("expected error for env, got nil")
	}
}
