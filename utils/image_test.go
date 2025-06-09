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
