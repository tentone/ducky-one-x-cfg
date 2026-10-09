package ui

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"testing"
	"time"

	fyneTest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/tentone/ducky-drv/internal/i18n"
	"github.com/tentone/ducky-drv/internal/protocol"
)

func TestLightingImageMapsPhysicalLayout(t *testing.T) {
	// Nonzero bounds also exercise coordinate offsets. Every layout unit is
	// represented by 16 pixels, so key footprints align with these bands.
	img := image.NewNRGBA(image.Rect(10, 20, 10+23*16, 20+6*16))
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			c := color.NRGBA{R: 255, A: 255}
			if x >= 10+15*16 {
				c = color.NRGBA{B: 255, A: 255}
			}
			if y >= 20+5*16 {
				c = color.NRGBA{G: 255, A: 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	colors := lightingColorsFromImage(img)
	if len(colors) != len(fullSizeKeyboardLayout()) {
		t.Fatalf("mapped %d keys, want %d", len(colors), len(fullSizeKeyboardLayout()))
	}
	for index, want := range map[int]color.NRGBA{
		0: {R: 255, A: 255}, 34: {R: 255, A: 255},
		17: {B: 255, A: 255}, 110: {G: 255, A: 255},
		104: {G: 127, B: 127, A: 255}, // Double-height numpad Enter averages two rows.
	} {
		if got := colors[index]; got != want {
			t.Errorf("key %d = %v, want %v", index, got, want)
		}
	}
	settings := customLightingFromControls(&lightingControls{keyColors: colors, brightness: &widget.Slider{Value: 75}})
	for _, key := range settings.Colors {
		want := colors[int(key.Key)]
		if key.Red != want.R || key.Green != want.G || key.Blue != want.B {
			t.Fatalf("protocol color for key %d differs from image mapping", key.Key)
		}
	}
}

func TestLightingImageTransparencyAndSmallImages(t *testing.T) {
	for _, alpha := range []byte{0, 128, 255} {
		img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
		img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: alpha})
		for index, got := range lightingColorsFromImage(img) {
			if want := (color.NRGBA{R: alpha, A: 255}); got != want {
				t.Errorf("alpha %d, key %d = %v, want %v", alpha, index, got, want)
			}
		}
	}
}

func TestCustomLightingRetainsSharedColorsForPainting(t *testing.T) {
	app := fyneTest.NewApp()
	defer app.Quit()
	u := &UI{app: app, window: app.NewWindow("test"), i18n: i18n.New()}
	u.lighting = u.buildLighting()
	defer u.lighting.animator.Stop()
	shared := u.lighting.keyColors
	shared[0] = color.NRGBA{B: 255, A: 255}
	u.setCustomLighting(&u.lighting, protocol.CustomLightingSettings{
		Colors: []protocol.KeyColor{{Key: 17, Green: 255}},
	})
	if _, exists := shared[0]; exists || shared[17].G != 255 {
		t.Fatal("loading custom colors must replace contents of the shared map")
	}
	u.lighting.effect.SetSelected(effectName(protocol.CustomStaticLightingEffect, u.i18n.T))
	u.lighting.keyboard.onTapped(0)
	if got := u.lighting.keyColors[0]; got.R != u.lighting.red || got.G != u.lighting.green || got.B != u.lighting.blue {
		t.Fatalf("painted key was not retained in saved UI state: %v", got)
	}
	if u.lighting.keyColors[17].G != 255 {
		t.Fatal("painting a key lost the imported colors")
	}
}

func TestDecodeLightingImageFormats(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, color.White)
		}
	}
	for name, encode := range map[string]func(io.Writer) error{
		"png":  func(w io.Writer) error { return png.Encode(w, img) },
		"jpeg": func(w io.Writer) error { return jpeg.Encode(w, img, nil) },
		"gif":  func(w io.Writer) error { return gif.Encode(w, img, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			var data bytes.Buffer
			if err := encode(&data); err != nil {
				t.Fatal(err)
			}
			colors, err := decodeLightingImage(&data)
			if err != nil {
				t.Fatal(err)
			}
			for index, got := range colors {
				if got.R < 250 || got.G < 250 || got.B < 250 || got.A != 255 {
					t.Fatalf("key %d = %v, want white", index, got)
				}
			}
		})
	}
}

func TestDecodeLightingImageRejectsInvalidAndOversizedFiles(t *testing.T) {
	if _, err := decodeLightingImage(strings.NewReader("not an image")); err == nil {
		t.Fatal("invalid image accepted")
	}
	if _, err := decodeLightingImage(io.LimitReader(zeroImageReader{}, maxLightingImageBytes+1)); err == nil || !strings.Contains(err.Error(), "32 MB") {
		t.Fatalf("oversized file error = %v", err)
	}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	oversized := data.Bytes()
	binary.BigEndian.PutUint32(oversized[16:20], maxLightingImagePixels+1)
	binary.BigEndian.PutUint32(oversized[29:33], crc32.ChecksumIEEE(oversized[12:29]))
	if _, err := decodeLightingImage(bytes.NewReader(oversized)); err == nil || !strings.Contains(err.Error(), "20 million") {
		t.Fatalf("oversized dimensions error = %v", err)
	}
}

func TestLightingGIFCompositesPartialFramesAndDisposal(t *testing.T) {
	palette := color.Palette{color.Transparent, color.NRGBA{R: 255, A: 255}, color.NRGBA{B: 255, A: 255}}
	newFrame := func(bounds image.Rectangle, index byte) *image.Paletted {
		frame := image.NewPaletted(bounds, palette)
		for i := range frame.Pix {
			frame.Pix[i] = index
		}
		return frame
	}
	for _, disposal := range []byte{gif.DisposalPrevious, gif.DisposalBackground, gif.DisposalNone} {
		t.Run(fmt.Sprintf("disposal_%d", disposal), func(t *testing.T) {
			animation := &gif.GIF{
				Image: []*image.Paletted{
					newFrame(image.Rect(0, 0, 23, 6), 1),
					newFrame(image.Rect(19, 0, 23, 6), 2),
					newFrame(image.Rect(0, 0, 1, 1), 0),
				},
				Delay: []int{0, 7, 11}, Disposal: []byte{gif.DisposalNone, disposal, gif.DisposalNone},
				LoopCount: 2, Config: image.Config{Width: 23, Height: 6, ColorModel: palette},
			}
			var data bytes.Buffer
			if err := gif.EncodeAll(&data, animation); err != nil {
				t.Fatal(err)
			}
			decoded, err := decodeLightingAnimation(&data)
			if err != nil {
				t.Fatal(err)
			}
			if len(decoded.frames) != 3 || decoded.loopCount != 2 {
				t.Fatalf("invalid decoded animation: %+v", decoded)
			}
			for i, want := range []time.Duration{100 * time.Millisecond, 70 * time.Millisecond, 110 * time.Millisecond} {
				if decoded.frames[i].delay != want {
					t.Errorf("frame %d delay = %v, want %v", i, decoded.frames[i].delay, want)
				}
				if got := decoded.frames[i].colors[0]; got != (color.NRGBA{R: 255, A: 255}) {
					t.Errorf("frame %d left key = %v; transparent pixels should retain red", i, got)
				}
			}
			if got := decoded.frames[1].colors[17]; got != (color.NRGBA{B: 255, A: 255}) {
				t.Fatalf("partial frame = %v, want blue", got)
			}
			want := color.NRGBA{R: 255, A: 255}
			if disposal == gif.DisposalBackground {
				want = color.NRGBA{A: 255}
			}
			if disposal == gif.DisposalNone {
				want = color.NRGBA{B: 255, A: 255}
			}
			if got := decoded.frames[2].colors[17]; got != want {
				t.Errorf("disposed frame = %v, want %v", got, want)
			}
		})
	}
}

func TestLightingGIFLimitsBeforeDecompression(t *testing.T) {
	// Minimal GIF structure: image data is intentionally absent. Frame limits
	// must be caught by the preflight, before the image decoder attempts it.
	data := []byte("GIF89a")
	data = append(data, 1, 0, 1, 0, 0, 0, 0)
	appendFrame := func(width, height uint16) {
		data = append(data, 0x2c, 0, 0, 0, 0, byte(width), byte(width>>8), byte(height), byte(height>>8), 0, 2, 0)
	}
	for i := 0; i <= maxLightingGIFFrames; i++ {
		appendFrame(1, 1)
	}
	data = append(data, 0x3b)
	if err := validateLightingGIF(data); err == nil || !strings.Contains(err.Error(), "2000 frames") {
		t.Fatalf("frame limit error = %v", err)
	}
	data = data[:13]
	appendFrame(10000, 10000)
	data = append(data, 0x3b)
	if err := validateLightingGIF(data); err == nil || !strings.Contains(err.Error(), "64 million") {
		t.Fatalf("pixel limit error = %v", err)
	}
	for _, truncated := range [][]byte{data[:5], data[:14], data[:18]} {
		if err := validateLightingGIF(truncated); err == nil {
			t.Fatal("truncated GIF accepted")
		}
	}
}

type zeroImageReader struct{}

func (zeroImageReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
