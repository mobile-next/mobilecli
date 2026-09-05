package commands

import (
	"testing"

	"github.com/mobile-next/mobilecli/devices"
	"github.com/stretchr/testify/assert"
)

func TestEvictDevicesNotInDropsUnpluggedDevicesOnly(t *testing.T) {
	mu.Lock()
	deviceCache = map[string]devices.ControllableDevice{"kept": nil, "unplugged": nil}
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		deviceCache = map[string]devices.ControllableDevice{}
		mu.Unlock()
	})

	EvictDevicesNotIn([]string{"kept", "new"})

	mu.RLock()
	defer mu.RUnlock()
	_, kept := deviceCache["kept"]
	_, unplugged := deviceCache["unplugged"]
	assert.True(t, kept)
	assert.False(t, unplugged)
}

func TestShouldEvictAfterListingOnlyForUnfilteredSuccessfulScans(t *testing.T) {
	all := devices.DeviceListOptions{}
	assert.True(t, shouldEvictAfterListing(all, false))
	assert.False(t, shouldEvictAfterListing(all, true), "a failed remote fetch must not evict remote devices")
	assert.False(t, shouldEvictAfterListing(devices.DeviceListOptions{Platform: "ios"}, false))
	assert.False(t, shouldEvictAfterListing(devices.DeviceListOptions{DeviceType: "simulator"}, false))
}
