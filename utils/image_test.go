package utils

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertPngToJpeg(t *testing.T) {
	w := 32
	h := 32
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 0, 255})
		}
	}

	var pngBuf bytes.Buffer
	err := png.Encode(&pngBuf, img)
	require.NoError(t, err, "Failed to encode test PNG")

	jpegBytes, err := ConvertPngToJpeg(pngBuf.Bytes(), 90)
	assert.NoError(t, err, "ConvertPngToJpeg should succeed")

	out, err := jpeg.Decode(bytes.NewReader(jpegBytes))
	assert.NoError(t, err, "Output should be valid JPEG")

	assert.Equal(t, w, out.Bounds().Dx(), "Output width should match")
	assert.Equal(t, h, out.Bounds().Dy(), "Output height should match")
}

func TestConvertPngToJpeg_InvalidPNG(t *testing.T) {
	// Test with invalid PNG data
	invalidPngData := []byte("not a png file")
	
	_, err := ConvertPngToJpeg(invalidPngData, 90)
	assert.Error(t, err, "Should return error for invalid PNG data")
}

func TestConvertPngToJpeg_EmptyData(t *testing.T) {
	// Test with empty data
	emptyData := []byte{}
	
	_, err := ConvertPngToJpeg(emptyData, 90)
	assert.Error(t, err, "Should return error for empty data")
}

func TestConvertPngToJpeg_CorruptPNG(t *testing.T) {
	// Test with corrupted PNG data (starts with PNG signature but is invalid)
	corruptPngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00} // PNG signature + invalid data
	
	_, err := ConvertPngToJpeg(corruptPngData, 90)
	assert.Error(t, err, "Should return error for corrupt PNG data")
}
