package devices

import (
	"fmt"
	"strconv"
	"sync"

	goios "github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/danielpaulus/go-ios/ios/simlocation"
	"github.com/mobile-next/mobilecli/utils"
	log "github.com/sirupsen/logrus"
)

// locationSimulationServices holds the open DTX connections keyed by udid. On
// iOS 17+ the simulated location only lasts as long as that connection, so it
// has to outlive the call that started it (see --wait in the cli).
var locationSimulationServices sync.Map

// SetLocation overrides the device location. On iOS 16 and older the override
// sticks on its own; on iOS 17+ it lasts only while this process keeps the
// instruments connection open.
func (d *IOSDevice) SetLocation(lat, lon float64) error {
	log.SetLevel(log.WarnLevel)

	device, err := d.locationDevice()
	if err != nil {
		return err
	}

	if !device.SupportsRsd() {
		return simlocation.SetLocation(device, strconv.FormatFloat(lat, 'f', -1, 64), strconv.FormatFloat(lon, 'f', -1, 64))
	}

	if existing, ok := locationSimulationServices.Load(d.Udid); ok {
		return existing.(*instruments.LocationSimulationService).StartSimulateLocation(lat, lon)
	}

	service, err := instruments.NewLocationSimulationService(device)
	if err != nil {
		return fmt.Errorf("failed to create location simulation service: %w", err)
	}

	if err := service.StartSimulateLocation(lat, lon); err != nil {
		service.Close()
		return err
	}

	locationSimulationServices.Store(d.Udid, service)
	utils.Verbose("holding location simulation connection for %s, it ends when mobilecli exits", d.Udid)
	return nil
}

// ClearLocation restores the real device location
func (d *IOSDevice) ClearLocation() error {
	log.SetLevel(log.WarnLevel)

	if service, ok := locationSimulationServices.LoadAndDelete(d.Udid); ok {
		// StopSimulateLocation closes the connection on its way out
		return service.(*instruments.LocationSimulationService).StopSimulateLocation()
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
