package utils

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
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
	if err != nil {
		t.Fatalf("Failed to encode test PNG: %v", err)
	}

	jpegBytes, err := ConvertPngToJpeg(pngBuf.Bytes(), 90)
	if err != nil {
		t.Errorf("ConvertPngToJpeg() error = %v", err)
	}

	out, err := jpeg.Decode(bytes.NewReader(jpegBytes))
	if err != nil {
		t.Errorf("Output is not valid JPEG: %v", err)
	}

	if out.Bounds().Dx() != w || out.Bounds().Dy() != h {
		t.Errorf("Output is not %dx%d: %v", w, h, out.Bounds())
	}
}

func TestConvertPngToJpeg_InvalidPNG(t *testing.T) {
	// Test with invalid PNG data
	invalidPngData := []byte("not a png file")
	
	_, err := ConvertPngToJpeg(invalidPngData, 90)
	if err == nil {
		t.Error("Expected error for invalid PNG data, got nil")
	}
}

func TestConvertPngToJpeg_EmptyData(t *testing.T) {
	// Test with empty data
	emptyData := []byte{}
	
	_, err := ConvertPngToJpeg(emptyData, 90)
	if err == nil {
		t.Error("Expected error for empty data, got nil")
	}
}

func TestConvertPngToJpeg_CorruptPNG(t *testing.T) {
	// Test with corrupted PNG data (starts with PNG signature but is invalid)
	corruptPngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00} // PNG signature + invalid data
	
	_, err := ConvertPngToJpeg(corruptPngData, 90)
	if err == nil {
		t.Error("Expected error for corrupt PNG data, got nil")
	}
}
