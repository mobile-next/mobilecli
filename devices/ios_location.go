package devices

import (
	"fmt"
	"strconv"

	goios "github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/danielpaulus/go-ios/ios/simlocation"
	"github.com/mobile-next/mobilecli/utils"
	log "github.com/sirupsen/logrus"
)

// SetLocation overrides the device location. On iOS 16 and older the override
// sticks on its own; on iOS 17+ it lasts only while this process keeps the
// instruments connection open, which is why the service is held on the device.
func (d *IOSDevice) SetLocation(lat, lon float64) error {
	log.SetLevel(log.WarnLevel)

	device, err := d.locationDevice()
	if err != nil {
		return err
	}

	if !device.SupportsRsd() {
		return simlocation.SetLocation(device, strconv.FormatFloat(lat, 'f', -1, 64), strconv.FormatFloat(lon, 'f', -1, 64))
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.locationSimulationService != nil {
		if err := d.locationSimulationService.StartSimulateLocation(lat, lon); err == nil {
			return nil
		}

		// the connection went away with the device, drop it and start over
		utils.Verbose("location simulation connection for %s is stale, reconnecting", d.Udid)
		d.locationSimulationService.Close()
		d.locationSimulationService = nil
	}

	service, err := instruments.NewLocationSimulationService(device)
	if err != nil {
		return fmt.Errorf("failed to create location simulation service: %w", err)
	}

	if err := service.StartSimulateLocation(lat, lon); err != nil {
		service.Close()
		return err
	}

	d.locationSimulationService = service
	utils.Verbose("holding location simulation connection for %s, it ends when mobilecli exits", d.Udid)
	return nil
}

// ClearLocation restores the real device location
func (d *IOSDevice) ClearLocation() error {
	log.SetLevel(log.WarnLevel)

	if held, err := d.stopHeldLocationSimulation(); held || err != nil {
		return err
	}

	device, err := d.locationDevice()
	if err != nil {
		return err
	}

	if device.SupportsRsd() {
		return fmt.Errorf("no location override is held by this process: on iOS 17+ the override belongs to the mobilecli process that set it (see --wait)")
	}

	return simlocation.ResetLocation(device)
}

// stopHeldLocationSimulation stops the iOS 17+ simulation this process is
// holding, and reports whether there was one to stop.
func (d *IOSDevice) stopHeldLocationSimulation() (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.locationSimulationService == nil {
		return false, nil
	}

	// StopSimulateLocation closes the connection on its way out, but returns
	// before that when the call itself fails, so the service is only dropped once
	// the device confirmed it stopped
	if err := d.locationSimulationService.StopSimulateLocation(); err != nil {
		return true, err
	}

	d.locationSimulationService = nil
	return true, nil
}

// locationDevice returns a device entry usable for location services, with the
// iOS 17+ tunnel already up when one is needed.
func (d *IOSDevice) locationDevice() (goios.DeviceEntry, error) {
	if err := d.startTunnel(); err != nil {
		return goios.DeviceEntry{}, fmt.Errorf("failed to start tunnel: %w", err)
	}

	device, err := d.getEnhancedDevice()
	if err != nil {
		return goios.DeviceEntry{}, fmt.Errorf("failed to get enhanced device connection: %w", err)
	}

	return device, nil
}
