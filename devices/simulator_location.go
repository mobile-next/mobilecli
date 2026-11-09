package devices

import "fmt"

// SetLocation overrides the simulator location. The override survives app
// launches and mobilecli exiting, until ClearLocation is called.
func (s *SimulatorDevice) SetLocation(lat, lon float64) error {
	_, err := runSimctl("location", s.UDID, "set", fmt.Sprintf("%f,%f", lat, lon))
	return err
}

// ClearLocation stops any running scenario and restores the simulator's own location
func (s *SimulatorDevice) ClearLocation() error {
	_, err := runSimctl("location", s.UDID, "clear")
	return err
}
