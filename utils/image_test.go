package utils

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/mobile-next/mobilecli/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestPng(t *testing.T, width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 0, 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img), "Failed to encode test PNG")
	return buf.Bytes()
}

func decodeDimensions(t *testing.T, data []byte) (int, int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	require.NoError(t, err, "Output should be a decodable image")
	return cfg.Width, cfg.Height
}

func TestResizeFactor(t *testing.T) {
	tests := []struct {
		name           string
		width, height  int
		scale          float64
		maxSize        int
		expectedFactor float64
	}{
		{"no scale and no max size means no resize", 100, 200, 1.0, 0, 1.0},
		{"scale alone is used", 100, 200, 0.5, 0, 0.5},
		{"max size wins over scale", 100, 200, 0.5, 100, 0.5},
		{"max size is measured against the largest dimension", 100, 200, 1.0, 50, 0.25},
		{"max size larger than the image never upscales", 100, 200, 1.0, 400, 1.0},
		{"zero scale means no resize", 100, 200, 0, 0, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedFactor, resizeFactor(tt.width, tt.height, tt.scale, tt.maxSize))
		})
	}
}

func TestProcessScreenshot_PngWithoutResizeIsReturnedUnchanged(t *testing.T) {
	original := makeTestPng(t, 64, 32)

	out, err := ProcessScreenshot(original, "png", 90, 1.0, 0, nil, 0)

	require.NoError(t, err)
	assert.Equal(t, original, out, "PNG with no resize should be passed through untouched")
}

func TestProcessScreenshot_ScaleHalvesDimensions(t *testing.T) {
	original := makeTestPng(t, 64, 32)

	out, err := ProcessScreenshot(original, "png", 90, 0.5, 0, nil, 0)

	require.NoError(t, err)
	width, height := decodeDimensions(t, out)
	assert.Equal(t, 32, width)
	assert.Equal(t, 16, height)
}

func TestProcessScreenshot_MaxSizeCapsLargestDimension(t *testing.T) {
	original := makeTestPng(t, 64, 32)

	out, err := ProcessScreenshot(original, "jpeg", 90, 1.0, 16, nil, 0)

	require.NoError(t, err)
	assert.True(t, IsJPEG(out), "Output should be a JPEG")
	width, height := decodeDimensions(t, out)
	assert.Equal(t, 16, width)
	assert.Equal(t, 8, height)
}

func TestProcessScreenshot_JpegWithoutResizeIsConverted(t *testing.T) {
	original := makeTestPng(t, 64, 32)

	out, err := ProcessScreenshot(original, "jpeg", 90, 1.0, 0, nil, 0)

	require.NoError(t, err)
	assert.True(t, IsJPEG(out), "Output should be a JPEG")
	width, height := decodeDimensions(t, out)
	assert.Equal(t, 64, width)
	assert.Equal(t, 32, height)
}

func TestProcessScreenshot_InvalidPngReturnsError(t *testing.T) {
	_, err := ProcessScreenshot([]byte("not a png"), "png", 90, 0.5, 0, nil, 0)
	assert.Error(t, err)
}

func TestProcessScreenshot_RectCropsToElementBounds(t *testing.T) {
	original := makeTestPng(t, 100, 200)
	rect := &types.ScreenElementRect{X: 10, Y: 20, Width: 30, Height: 40}

	out, err := ProcessScreenshot(original, "png", 90, 1.0, 0, rect, 100)

	require.NoError(t, err)
	width, height := decodeDimensions(t, out)
	assert.Equal(t, 30, width)
	assert.Equal(t, 40, height)

	img, err := png.Decode(bytes.NewReader(out))
	require.NoError(t, err)
	r, g, _, _ := img.At(img.Bounds().Min.X, img.Bounds().Min.Y).RGBA()
	assert.Equal(t, uint32(10), r>>8, "Top-left pixel should come from x=10 of the source")
	assert.Equal(t, uint32(20), g>>8, "Top-left pixel should come from y=20 of the source")
}

func TestProcessScreenshot_RectInPointsIsScaledToPixels(t *testing.T) {
	original := makeTestPng(t, 200, 400) // 2x density: screen is 100 points wide
	rect := &types.ScreenElementRect{X: 10, Y: 20, Width: 30, Height: 40}

	out, err := ProcessScreenshot(original, "png", 90, 1.0, 0, rect, 100)

	require.NoError(t, err)
	width, height := decodeDimensions(t, out)
	assert.Equal(t, 60, width)
	assert.Equal(t, 80, height)
}

func TestProcessScreenshot_RectAppliesBeforeMaxSize(t *testing.T) {
	original := makeTestPng(t, 100, 200)
	rect := &types.ScreenElementRect{X: 0, Y: 0, Width: 40, Height: 80}

	out, err := ProcessScreenshot(original, "png", 90, 1.0, 40, rect, 100)

	require.NoError(t, err)
	width, height := decodeDimensions(t, out)
	assert.Equal(t, 20, width, "maxSize should cap the cropped image, not the full screen")
	assert.Equal(t, 40, height)
}

func TestProcessScreenshot_RectPartiallyOutsideIsClamped(t *testing.T) {
	original := makeTestPng(t, 100, 200)
	rect := &types.ScreenElementRect{X: 80, Y: 180, Width: 50, Height: 50}

	out, err := ProcessScreenshot(original, "png", 90, 1.0, 0, rect, 100)

	require.NoError(t, err)
	width, height := decodeDimensions(t, out)
	assert.Equal(t, 20, width)
	assert.Equal(t, 20, height)
}

func TestProcessScreenshot_RectFullyOutsideReturnsError(t *testing.T) {
	original := makeTestPng(t, 100, 200)
	rect := &types.ScreenElementRect{X: 150, Y: 0, Width: 10, Height: 10}

	_, err := ProcessScreenshot(original, "png", 90, 1.0, 0, rect, 100)

	assert.ErrorContains(t, err, "outside the screenshot bounds")
}

func TestProcessScreenshot_RectWithoutScreenWidthReturnsError(t *testing.T) {
	original := makeTestPng(t, 100, 200)
	rect := &types.ScreenElementRect{X: 0, Y: 0, Width: 10, Height: 10}

	_, err := ProcessScreenshot(original, "png", 90, 1.0, 0, rect, 0)

	assert.ErrorContains(t, err, "screen width")
}

func TestProcessScreenshot_RectWithNonPositiveSizeReturnsError(t *testing.T) {
	original := makeTestPng(t, 100, 200)
	rect := &types.ScreenElementRect{X: 0, Y: 0, Width: 0, Height: 10}

	_, err := ProcessScreenshot(original, "png", 90, 1.0, 0, rect, 100)

	assert.ErrorContains(t, err, "must be positive")
}

func TestImageMagicByteDetection(t *testing.T) {
	pngData := makeTestPng(t, 8, 8)
	jpegData, err := ConvertPngToJpeg(pngData, 90)
	require.NoError(t, err)

	assert.True(t, IsPNG(pngData))
	assert.False(t, IsPNG(jpegData))
	assert.True(t, IsJPEG(jpegData))
	assert.False(t, IsJPEG(pngData))
	assert.False(t, IsPNG(nil))
	assert.False(t, IsJPEG(nil))
}

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

func TestConvertPngToJpeg_CorruptPNG(t *testing.T) {
	// Test with corrupted PNG data (starts with PNG signature but is invalid)
	corruptPngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00} // PNG signature + invalid data

	_, err := ConvertPngToJpeg(corruptPngData, 90)
	assert.Error(t, err, "Should return error for corrupt PNG data")
}

func TestConvertPngToJpeg_EmptyData(t *testing.T) {
	// Test with empty data
	emptyData := []byte{}

	_, err := ConvertPngToJpeg(emptyData, 90)
	if err == nil {
		t.Error("Expected error for empty data, got nil")
	}
}
