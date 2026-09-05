package commands

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateScreenshotResize(t *testing.T) {
	tests := []struct {
		name          string
		scale         float64
		maxSize       int
		expectedScale float64
		expectError   bool
	}{
		{"zero scale defaults to no scaling", 0, 0, 1.0, false},
		{"valid scale is kept", 0.5, 0, 0.5, false},
		{"valid max size is accepted", 1.0, 800, 1.0, false},
		{"negative scale is rejected", -0.5, 0, 0, true},
		{"scale above one is rejected", 1.5, 0, 0, true},
		{"NaN scale is rejected", math.NaN(), 0, 0, true},
		{"negative max size is rejected", 1.0, -100, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scale, err := validateScreenshotResize(tt.scale, tt.maxSize)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedScale, scale)
			}
		})
	}
}
