package ui

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"time"
)

const (
	maxLightingImageBytes  = 32 << 20
	maxLightingImagePixels = 20_000_000
	maxLightingGIFPixels   = 64_000_000
	maxLightingGIFFrames   = 2000
)

type lightingImageFrame struct {
	colors map[int]color.NRGBA
	delay  time.Duration
}

type lightingImage struct {
	frames    []lightingImageFrame
	loopCount int // GIF convention: -1 once, 0 forever, positive additional repeats.
}

func decodeLightingImage(reader io.Reader) (map[int]color.NRGBA, error) {
	decoded, err := decodeLightingAnimation(reader)
	if err != nil {
		return nil, err
	}
	return decoded.frames[0].colors, nil
}

func decodeLightingAnimation(reader io.Reader) (*lightingImage, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxLightingImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxLightingImageBytes {
		return nil, fmt.Errorf("image exceeds 32 MB")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > maxLightingImagePixels/config.Height {
		return nil, fmt.Errorf("image exceeds 20 million pixels or has invalid dimensions")
	}
	if format == "gif" {
		if err := validateLightingGIF(data); err != nil {
			return nil, err
		}
		animation, err := gif.DecodeAll(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return lightingFramesFromGIF(animation), nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return &lightingImage{frames: []lightingImageFrame{{colors: lightingColorsFromImage(img)}}, loopCount: -1}, nil
}

// Check frame allocations before DecodeAll decompresses the entire GIF.
func validateLightingGIF(data []byte) error {
	if len(data) < 13 {
		return io.ErrUnexpectedEOF
	}
	offset := 13
	if data[10]&0x80 != 0 {
		offset += 3 * (1 << ((data[10] & 7) + 1))
	}
	frames, pixels := 0, uint64(0)
	skipBlocks := func() error {
		for {
			if offset >= len(data) {
				return io.ErrUnexpectedEOF
			}
			size := int(data[offset])
			offset++
			if size == 0 {
				return nil
			}
			if size > len(data)-offset {
				return io.ErrUnexpectedEOF
			}
			offset += size
		}
	}
	for offset < len(data) {
		marker := data[offset]
		offset++
		switch marker {
		case 0x3b:
			return nil
		case 0x21:
			if offset >= len(data) {
				return io.ErrUnexpectedEOF
			}
			offset++ // Extension label; all extension payloads use sub-blocks.
			if err := skipBlocks(); err != nil {
				return err
			}
		case 0x2c:
			if len(data)-offset < 9 {
				return io.ErrUnexpectedEOF
			}
			width := uint64(binary.LittleEndian.Uint16(data[offset+4:]))
			height := uint64(binary.LittleEndian.Uint16(data[offset+6:]))
			pixels += width * height
			frames++
			if pixels > maxLightingGIFPixels || frames > maxLightingGIFFrames {
				return fmt.Errorf("GIF exceeds 64 million total frame pixels or 2000 frames")
			}
			flags := data[offset+8]
			offset += 9
			if flags&0x80 != 0 {
				offset += 3 * (1 << ((flags & 7) + 1))
			}
			if offset >= len(data) {
				return io.ErrUnexpectedEOF
			}
			offset++ // LZW minimum code size.
			if err := skipBlocks(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid GIF block")
		}
	}
	return io.ErrUnexpectedEOF
}

func lightingFramesFromGIF(animation *gif.GIF) *lightingImage {
	decoded := &lightingImage{loopCount: animation.LoopCount}
	bounds := image.Rect(0, 0, animation.Config.Width, animation.Config.Height)
	canvas := image.NewNRGBA(bounds)
	background := color.Color(color.Transparent)
	if palette, ok := animation.Config.ColorModel.(color.Palette); ok && int(animation.BackgroundIndex) < len(palette) {
		background = palette[animation.BackgroundIndex]
		if len(animation.Image) > 0 && int(animation.BackgroundIndex) < len(animation.Image[0].Palette) {
			if _, _, _, alpha := animation.Image[0].Palette[animation.BackgroundIndex].RGBA(); alpha == 0 {
				background = color.Transparent
			}
		}
	}
	draw.Draw(canvas, bounds, image.NewUniform(background), image.Point{}, draw.Src)
	var restore *image.NRGBA
	for i, frame := range animation.Image {
		disposal := byte(0)
		if i < len(animation.Disposal) {
			disposal = animation.Disposal[i]
		}
		if disposal == gif.DisposalPrevious {
			if restore == nil {
				restore = image.NewNRGBA(bounds)
			}
			copy(restore.Pix, canvas.Pix)
		}
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		delay := time.Duration(animation.Delay[i]) * 10 * time.Millisecond
		if delay <= 0 {
			delay = 100 * time.Millisecond
		}
		decoded.frames = append(decoded.frames, lightingImageFrame{colors: lightingColorsFromImage(canvas), delay: delay})
		switch disposal {
		case gif.DisposalBackground:
			draw.Draw(canvas, frame.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			copy(canvas.Pix, restore.Pix)
		}
	}
	return decoded
}

// Fit the whole image to the physical layout, averaging a bounded grid over each
// key's footprint. RGBA supplies premultiplied channels, compositing alpha onto
// black (unlit keys).
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
