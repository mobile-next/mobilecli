package commands

import (
	"sync"
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

func TestCacheDeviceGivesEveryConcurrentLookupTheSameInstance(t *testing.T) {
	t.Cleanup(func() {
		mu.Lock()
		deviceCache = map[string]devices.ControllableDevice{}
		mu.Unlock()
	})

	winners := cacheTheSameDeviceIDConcurrently(16)

	for _, winner := range winners {
		assert.Same(t, winners[0], winner, "each lookup must share one device, or its mutexes guard nothing")
	}
}

// cacheTheSameDeviceIDConcurrently mimics a burst of first requests for one
// device: each enumerates its own instance, then caches it at the same moment.
func cacheTheSameDeviceIDConcurrently(lookups int) []devices.ControllableDevice {
	winners := make([]devices.ControllableDevice, lookups)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range winners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ownInstance := devices.NewRemoteDevice(devices.DeviceInfo{ID: "same-device"}, "")
			<-start
			winners[i] = cacheDevice(ownInstance)
		}()
	}
	close(start)
	wg.Wait()
	return winners
}
