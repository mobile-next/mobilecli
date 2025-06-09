package utils

import (
	"bytes"
	"image/jpeg"
	"image/png"
)

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
