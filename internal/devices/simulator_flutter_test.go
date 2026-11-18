package devices

import "testing"

// A verdict that outlives the process it was taken for is the only way this
// cache can be wrong, and it would be wrong silently: a Flutter app would be
// dumped through the accessibility tree, or a native app would be probed for a
// Dart VM on every dump. The tests below pin the rules that prevent that.
func TestFlutterCandidateIsRememberedForTheSameProcess(t *testing.T) {
	t.Cleanup(func() { delete(simulatorFlutterCandidates, "udid-1") })

	if _, ok := cachedFlutterCandidate("udid-1", 100); ok {
		t.Fatal("an unknown simulator must not have a verdict")
	}

	rememberFlutterCandidate("udid-1", 100, true)

	isCandidate, ok := cachedFlutterCandidate("udid-1", 100)
	if !ok || !isCandidate {
		t.Errorf("the verdict for the same process must be reused, got (%v, %v)", isCandidate, ok)
	}
}

// A reinstall can replace a native build with a Flutter one under the same
// bundle id, so the pid, not the bundle id, is what the verdict is tied to.
func TestFlutterCandidateIsLookedAtAgainAfterTheAppRestarts(t *testing.T) {
	t.Cleanup(func() { delete(simulatorFlutterCandidates, "udid-2") })

	rememberFlutterCandidate("udid-2", 100, false)

	if _, ok := cachedFlutterCandidate("udid-2", 101); ok {
		t.Error("a relaunched or reinstalled app has a new pid and must be looked at again")
	}
}

// The agent reports pid 0 when it cannot name the foreground process. Storing
// a verdict against it would pin an answer that no app relaunch could ever
// clear, so paying the simctl call again is the cheaper mistake.
func TestFlutterCandidateIsNotRememberedWithoutAProcessID(t *testing.T) {
	t.Cleanup(func() { delete(simulatorFlutterCandidates, "udid-3") })

	rememberFlutterCandidate("udid-3", 0, true)

	if _, ok := cachedFlutterCandidate("udid-3", 0); ok {
		t.Error("a verdict that cannot be invalidated must not be stored")
	}
	if _, ok := cachedFlutterCandidate("udid-3", 100); ok {
		t.Error("a verdict stored without a pid must not be served to a real process")
	}
}

// Pids are per simulator, so two booted simulators running different apps can
// hand out the same number; the udid has to be part of the key.
func TestFlutterCandidatesAreNotSharedBetweenSimulators(t *testing.T) {
	t.Cleanup(func() {
		delete(simulatorFlutterCandidates, "udid-4")
		delete(simulatorFlutterCandidates, "udid-5")
	})

	rememberFlutterCandidate("udid-4", 100, true)

	if _, ok := cachedFlutterCandidate("udid-5", 100); ok {
		t.Error("two simulators can run the same pid; a verdict must not cross over")
	}
}
