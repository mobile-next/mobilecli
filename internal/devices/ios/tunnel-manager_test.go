package ios

import "testing"

func TestResetTunnelSwapsManagerWhenNotRunning(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir()) // own pair-record dir, other packages' tests create theirs in parallel
	tm, err := NewTunnelManager("udid-1")
	if err != nil {
		t.Fatal(err)
	}
	before := tm.GetTunnelManager()

	if err := tm.ResetTunnel(); err != nil {
		t.Fatal(err)
	}

	if tm.GetTunnelManager() == before {
		t.Fatal("ResetTunnel should replace the closed go-ios manager with a new one")
	}
	if tm.IsTunnelRunning() {
		t.Fatal("no tunnel should be running after ResetTunnel")
	}
}
