package devicekit

// SetHingeAngle sets the hinge of a foldable device: 0 is folded, 180 is fully open
func (c *DeviceKitClient) SetHingeAngle(angle float64) error {
	_, err := c.CallRPC("device.io.hinge.set", map[string]float64{"angle": angle})
	return err
}
