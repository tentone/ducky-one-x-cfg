package ui

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"testing"

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

type zeroImageReader struct{}

func (zeroImageReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
