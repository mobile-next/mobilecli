package utils

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"github.com/mobile-next/mobilecli/types"
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

// cropRectInPixels maps clip rect (in screen points) to pixel coordinates of an
// imageWidth-wide screenshot and clamps it to the image bounds. Bounds from a
// UI hierarchy may slightly exceed the screen (e.g. partially scrolled
// elements), so out-of-bounds portions are clipped rather than rejected.
func cropRectInPixels(rect *types.ScreenElementRect, screenWidthPoints, imageWidth, imageHeight int) (image.Rectangle, error) {
	if rect.Width <= 0 || rect.Height <= 0 {
		return image.Rectangle{}, fmt.Errorf("clip width and height must be positive")
	}
	if screenWidthPoints <= 0 {
		return image.Rectangle{}, fmt.Errorf("screen width is required for clip cropping")
	}

	factor := float64(imageWidth) / float64(screenWidthPoints)
	pixels := image.Rect(
		int(float64(rect.X)*factor+0.5),
		int(float64(rect.Y)*factor+0.5),
		int(float64(rect.X+rect.Width)*factor+0.5),
		int(float64(rect.Y+rect.Height)*factor+0.5),
	).Intersect(image.Rect(0, 0, imageWidth, imageHeight))

	if pixels.Empty() {
		return image.Rectangle{}, fmt.Errorf("clip rect is outside the screenshot bounds")
	}
	return pixels, nil
}

// ProcessScreenshot crops, resizes and re-encodes PNG screenshot bytes.
// clip rect (in screen points, mapped to pixels via screenWidthPoints) is applied
// first, then scale/maxSize (see resizeFactor) against the cropped size, then
// encoding to format ("png" or "jpeg"). When nothing is to be done and PNG is
// requested, the input is returned unchanged.
func ProcessScreenshot(pngBytes []byte, format string, quality int, scale float64, maxSize int, rect *types.ScreenElementRect, screenWidthPoints int) ([]byte, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, err
	}

	width, height := cfg.Width, cfg.Height
	var crop image.Rectangle
	if rect != nil {
		crop, err = cropRectInPixels(rect, screenWidthPoints, cfg.Width, cfg.Height)
		if err != nil {
			return nil, err
		}
		width, height = crop.Dx(), crop.Dy()
	}

	factor := resizeFactor(width, height, scale, maxSize)
	if rect == nil && factor == 1.0 {
		if format == "jpeg" {
			return ConvertPngToJpeg(pngBytes, quality)
		}
		return pngBytes, nil
	}

	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, err
	}

	if rect != nil {
		sub, ok := img.(interface {
			SubImage(image.Rectangle) image.Image
		})
		if !ok {
			return nil, fmt.Errorf("unsupported image type for cropping")
		}
		img = sub.SubImage(crop)
	}

	newWidth := max(1, int(float64(width)*factor+0.5))
	newHeight := max(1, int(float64(height)*factor+0.5))
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
