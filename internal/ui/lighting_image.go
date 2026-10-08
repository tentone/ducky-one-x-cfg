package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
)

const (
	maxLightingImageBytes  = 32 << 20
	maxLightingImagePixels = 20_000_000
)

func decodeLightingImage(reader io.Reader) (map[int]color.NRGBA, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxLightingImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxLightingImageBytes {
		return nil, fmt.Errorf("image exceeds 32 MB")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > maxLightingImagePixels/config.Height {
		return nil, fmt.Errorf("image exceeds 20 million pixels or has invalid dimensions")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return lightingColorsFromImage(img), nil
}

// Fit the whole image to the physical layout, averaging a bounded grid over each
// key's footprint. RGBA supplies premultiplied channels, compositing alpha onto
// black (unlit keys). GIF decoding uses its first frame.
func lightingColorsFromImage(img image.Image) map[int]color.NRGBA {
	keys := fullSizeKeyboardLayout()
	var width, height float32
	for _, key := range keys {
		width = max(width, key.x+key.width)
		height = max(height, key.y+key.height)
	}
	bounds := img.Bounds()
	colors := make(map[int]color.NRGBA, len(keys))
	const samples = 16
	for _, key := range keys {
		var red, green, blue uint64
		for y := 0; y < samples; y++ {
			py := bounds.Min.Y + int((float64(key.y)+float64(key.height)*(float64(y)+0.5)/samples)/float64(height)*float64(bounds.Dy()))
			for x := 0; x < samples; x++ {
				px := bounds.Min.X + int((float64(key.x)+float64(key.width)*(float64(x)+0.5)/samples)/float64(width)*float64(bounds.Dx()))
				r, g, b, _ := img.At(px, py).RGBA()
				red += uint64(r)
				green += uint64(g)
				blue += uint64(b)
			}
		}
		const divisor = samples * samples * 257
		colors[key.index] = color.NRGBA{R: byte(red / divisor), G: byte(green / divisor), B: byte(blue / divisor), A: 255}
	}
	return colors
}
