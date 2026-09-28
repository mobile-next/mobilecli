package commands

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// FoldRequest represents the parameters for folding a foldable device
type FoldRequest struct {
	DeviceID string `json:"deviceId"`
	State    string `json:"state"`
}

// hingeController is implemented by devices that can simulate a hinge (foldables)
type hingeController interface {
	SetHingeAngle(angle float64) error
}

var namedFoldAngles = map[string]float64{
	"folded":    0,
	"half-open": 90,
	"open":      180,
}

// parseFoldAngle accepts "folded", "half-open", "open" or an angle in degrees (0-180)
func parseFoldAngle(state string) (float64, error) {
	if angle, ok := namedFoldAngles[strings.ToLower(state)]; ok {
		return angle, nil
	}

	angle, err := strconv.ParseFloat(state, 64)
	if err != nil || math.IsNaN(angle) || angle < 0 || angle > 180 {
		return 0, fmt.Errorf("invalid fold state '%s', must be 'folded', 'half-open', 'open' or an angle between 0 and 180", state)
	}

	return angle, nil
}

// FoldCommand folds or unfolds a foldable device
func FoldCommand(req FoldRequest) *CommandResponse {
	angle, err := parseFoldAngle(req.State)
	if err != nil {
		return NewErrorResponse(err)
	}

	targetDevice, err := FindDeviceWithAgent(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	foldable, ok := targetDevice.(hingeController)
	if !ok {
		return NewErrorResponse(fmt.Errorf("device %s does not support folding", targetDevice.ID()))
	}

	if err := foldable.SetHingeAngle(angle); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to fold on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Set fold angle to %g° on device %s", angle, targetDevice.ID()),
	})
}
