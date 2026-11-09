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
			name: "notification shade over an app falls back to the focused app",
			input: "  mCurrentFocus=Window{1d2ee65 u0 NotificationShade}\n" +
				"  mFocusedApp=ActivityRecord{e98d7c8 u0 com.android.settings/.Settings t256}",
			wantPackage:  "com.android.settings",
			wantActivity: "com.android.settings.Settings",
		},
		{
			name: "notification shade over the launcher falls back to the launcher",
			input: "  mCurrentFocus=Window{1d2ee65 u0 NotificationShade}\n" +
				"  mFocusedApp=ActivityRecord{8652e8e u0 com.google.android.apps.nexuslauncher/.NexusLauncherActivity t173}",
			wantPackage:  "com.google.android.apps.nexuslauncher",
			wantActivity: "com.google.android.apps.nexuslauncher.NexusLauncherActivity",
		},
		{
			name: "focused window wins over the focused app",
			input: "  mCurrentFocus=Window{da7ab78 u0 com.example.app/com.example.app.DialogActivity}\n" +
				"  mFocusedApp=ActivityRecord{2424b9c u0 com.example.app/.MainActivity t257}",
			wantPackage:  "com.example.app",
			wantActivity: "com.example.app.DialogActivity",
		},
		{
			name: "focused app with a fully qualified activity from another package",
			input: "  mCurrentFocus=Window{1d2ee65 u0 InputMethod}\n" +
				"  mFocusedApp=ActivityRecord{2424b9c u0 com.example.app/com.example.shared.MainActivity t257}",
			wantPackage:  "com.example.app",
			wantActivity: "com.example.shared.MainActivity",
		},
		{
			name: "focused app wrapped in an app window token (older Android)",
			input: "  mCurrentFocus=Window{1d2ee65 u0 StatusBar}\n" +
				"  mFocusedApp=AppWindowToken{c4a9b0 token=Token{2ab2d4f ActivityRecord{89a6d2e u0 com.android.settings/.Settings t9}}}",
			wantPackage:  "com.android.settings",
			wantActivity: "com.android.settings.Settings",
		},
		{
			name: "system window with no focused app, as when the screen is off",
			input: "  mCurrentFocus=Window{1d2ee65 u0 NotificationShade}\n" +
				"  mFocusedApp=null",
			wantErr: true,
		},
		{
			name: "null focus during a transition is not answered from the focused app",
			input: "  mCurrentFocus=null\n" +
				"  mFocusedApp=ActivityRecord{195695205 u0 com.example.app/.MainActivity t8}",
			wantErr: true,
		},
		{
			name: "focused app of another display is not used",
			input: "  mCurrentFocus=Window{1d2ee65 u0 NotificationShade}\n" +
				"  mFocusedApp=null\n" +
				"  mCurrentFocus=null\n" +
				"  mFocusedApp=ActivityRecord{2424b9c u0 com.example.other/.MainActivity t9}",
			wantErr: true,
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
