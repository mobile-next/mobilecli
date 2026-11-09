package devices

import (
	"os"
	"path/filepath"
	"testing"
)

const thisUDID = "4EBD52C1-C43E-4822-8654-D932AA635D3B"

func writeReport(t *testing.T, dir, name, header, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(header+"\n"+body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCrashReportIsForSimulatorMatchesOnlyThisSimulator(t *testing.T) {
	dir := t.TempDir()

	// a crash from this simulator: is_simulated + this device's coalition
	sim := writeReport(t, dir, "MobileSafari.ips",
		`{"is_simulated":1,"app_name":"MobileSafari","bug_type":"309"}`,
		`{"coalitionName":"com.apple.CoreSimulator.SimDevice.`+thisUDID+`"}`)

	// a host process crash: no is_simulated
	host := writeReport(t, dir, "adb.ips",
		`{"app_name":"adb","bug_type":"309"}`,
		`{"coalitionName":"adb"}`)

	// a crash from a *different* simulator
	otherSim := writeReport(t, dir, "Other.ips",
		`{"is_simulated":1,"app_name":"Maps"}`,
		`{"coalitionName":"com.apple.CoreSimulator.SimDevice.00000000-0000-0000-0000-000000000000"}`)

	// is_simulated expressed as a boolean should also count
	simBool := writeReport(t, dir, "Bool.ips",
		`{"is_simulated":true,"app_name":"Health"}`,
		`com.apple.CoreSimulator.SimDevice.`+thisUDID)

	if !crashReportIsForSimulator(sim, thisUDID) {
		t.Error("expected this simulator's report to match")
	}
	if !crashReportIsForSimulator(simBool, thisUDID) {
		t.Error("expected is_simulated:true report to match")
	}
	if crashReportIsForSimulator(host, thisUDID) {
		t.Error("host-process report must not match")
	}
	if crashReportIsForSimulator(otherSim, thisUDID) {
		t.Error("another simulator's report must not match")
	}
	if crashReportIsForSimulator(filepath.Join(dir, "missing.ips"), thisUDID) {
		t.Error("a missing file must not match")
	}
}
