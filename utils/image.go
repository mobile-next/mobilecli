package utils

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"

	"golang.org/x/image/draw"
)

// IsPNG reports whether data starts with the PNG signature.
func IsPNG(data []byte) bool {
	return bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n"))
}

// IsJPEG reports whether data starts with the JPEG SOI marker.
func IsJPEG(data []byte) bool {
	return bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff})
}

func ConvertPngToJpeg(pngBytes []byte, quality int) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, err
	}

	var jpegBytes bytes.Buffer
	if err := jpeg.Encode(&jpegBytes, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}

	return jpegBytes.Bytes(), nil
}

// resizeFactor returns the downscale factor for an image of the given
// dimensions. maxSize caps max(width, height) and takes precedence over
// scale; a factor of 1.0 means no resize. Never upscales.
func resizeFactor(width, height int, scale float64, maxSize int) float64 {
	largest := max(width, height)

	factor := scale
	if maxSize > 0 {
		factor = 1.0
		if maxSize < largest {
			factor = float64(maxSize) / float64(largest)
		}
	}

	if factor <= 0 || factor >= 1.0 {
		return 1.0
	}
	return factor
}

// ProcessScreenshot resizes and re-encodes PNG screenshot bytes according to
// scale/maxSize (see resizeFactor) and format ("png" or "jpeg"). When no
// resize is needed and PNG is requested, the input is returned unchanged.
func ProcessScreenshot(pngBytes []byte, format string, quality int, scale float64, maxSize int) ([]byte, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, err
	}

	factor := resizeFactor(cfg.Width, cfg.Height, scale, maxSize)
	if factor == 1.0 {
		if format == "jpeg" {
			return ConvertPngToJpeg(pngBytes, quality)
		}
		return pngBytes, nil
	}

	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, err
	}

	newWidth := max(1, int(float64(cfg.Width)*factor+0.5))
	newHeight := max(1, int(float64(cfg.Height)*factor+0.5))
	resized := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	draw.CatmullRom.Scale(resized, resized.Bounds(), img, img.Bounds(), draw.Src, nil)

	var out bytes.Buffer
	if format == "jpeg" {
		err = jpeg.Encode(&out, resized, &jpeg.Options{Quality: quality})
	} else {
		err = png.Encode(&out, resized)
	}
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
