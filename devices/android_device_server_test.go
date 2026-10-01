package devices

import (
	"testing"
	"time"
)

func Test_deviceServerIdleTimeout_defaultsToThirtyMinutes(t *testing.T) {
	t.Setenv(deviceServerIdleTimeoutEnv, "")

	if got := deviceServerIdleTimeout(); got != 30*time.Minute {
		t.Errorf("got %s, want 30m", got)
	}
}

func Test_deviceServerIdleTimeout_readsTheEnvironment(t *testing.T) {
	t.Setenv(deviceServerIdleTimeoutEnv, "2s")

	if got := deviceServerIdleTimeout(); got != 2*time.Second {
		t.Errorf("got %s, want 2s", got)
	}
}

func Test_deviceServerIdleTimeout_ignoresAValueItCannotUse(t *testing.T) {
	for _, value := range []string{"soon", "-5m", "0"} {
		t.Setenv(deviceServerIdleTimeoutEnv, value)

		if got := deviceServerIdleTimeout(); got != 30*time.Minute {
			t.Errorf("%q: got %s, want the 30m default", value, got)
		}
	}
}
