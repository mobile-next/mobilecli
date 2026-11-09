package cli

import (
	"testing"

	"github.com/mobile-next/mobilecli/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseScreenshotClip(t *testing.T) {
	t.Run("empty value means no cropping", func(t *testing.T) {
		rect, err := parseScreenshotClip("")
		require.NoError(t, err)
		assert.Nil(t, rect)
	})

	t.Run("parses x,y,width,height", func(t *testing.T) {
		rect, err := parseScreenshotClip("10,20,30,40")
		require.NoError(t, err)
		assert.Equal(t, &types.ScreenElementRect{X: 10, Y: 20, Width: 30, Height: 40}, rect)
	})

	t.Run("rejects malformed values", func(t *testing.T) {
		for _, value := range []string{"10,20,30", "a,b,c,d", "10 20 30 40", "10,20,30,40,50", "10,20,30,40x", "10,20,30,40,"} {
			_, err := parseScreenshotClip(value)
			assert.Error(t, err, "value %q should be rejected", value)
		}
	})
}
