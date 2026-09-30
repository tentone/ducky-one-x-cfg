// Generate platform icons from assets/ducky.svg. Run from the repository root:
// go run ./scripts/generate-icons
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/fyne-io/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/draw"
)

func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate() error {
	source, err := os.ReadFile("assets/ducky.svg")
	if err != nil {
		return err
	}
	// Resolve CSS currentColor for standalone rasterization; keep the original
	// SVG unchanged so Fyne can recolor it to match the active theme.
	source = bytes.ReplaceAll(source, []byte("currentColor"), []byte("#000000"))
	// A light rounded tile keeps the monochrome duck visible on dark taskbars
	// and desktops. The GUI and tray still use the transparent, themed SVG.
	source = bytes.Replace(source, []byte("<path"), []byte(`<rect x="32" y="32" width="1190" height="1190" rx="200" ry="200" fill="#ffffff"/><path`), 1)
	icon, err := oksvg.ReadIconStream(bytes.NewReader(source))
	if err != nil {
		return err
	}
	const size = 1024
	rendered := image.NewRGBA(image.Rect(0, 0, size, size))
	icon.SetTarget(0, 0, size, size)
	icon.Draw(rasterx.NewDasher(size, size, rasterx.NewScannerGV(size, size, rendered, rendered.Bounds())), 1)
	images := make(map[int][]byte)
	for _, n := range []int{16, 24, 32, 48, 64, 128, 256, 512, 1024} {
		img := image.NewRGBA(image.Rect(0, 0, n, n))
		draw.CatmullRom.Scale(img, img.Bounds(), rendered, rendered.Bounds(), draw.Src, nil)
		var data bytes.Buffer
		if err := png.Encode(&data, img); err != nil {
			return err
		}
		images[n] = data.Bytes()
	}
	if err := os.WriteFile("assets/ducky.png", images[512], 0644); err != nil {
		return err
	}
	// ICO entries contain PNG images, supported by Windows Vista and later.
	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	var ico bytes.Buffer
	writeLE := func(v any) { _ = binary.Write(&ico, binary.LittleEndian, v) }
	writeLE(uint16(0))
	writeLE(uint16(1))
	writeLE(uint16(len(sizes)))
	offset := uint32(6 + 16*len(sizes))
	for _, n := range sizes {
		ico.Write([]byte{byte(n % 256), byte(n % 256), 0, 0})
		writeLE(uint16(1))
		writeLE(uint16(32))
		writeLE(uint32(len(images[n])))
		writeLE(offset)
		offset += uint32(len(images[n]))
	}
	for _, n := range sizes {
		ico.Write(images[n])
	}
	if err := os.WriteFile("assets/ducky.ico", ico.Bytes(), 0644); err != nil {
		return err
	}
	// Modern ICNS uses PNG payloads for these standard size identifiers.
	var chunks bytes.Buffer
	for _, entry := range []struct {
		kind string
		size int
	}{
		{"ic07", 128}, {"ic08", 256}, {"ic09", 512}, {"ic10", 1024},
	} {
		chunks.WriteString(entry.kind)
		_ = binary.Write(&chunks, binary.BigEndian, uint32(8+len(images[entry.size])))
		chunks.Write(images[entry.size])
	}
	var icns bytes.Buffer
	icns.WriteString("icns")
	_ = binary.Write(&icns, binary.BigEndian, uint32(8+chunks.Len()))
	icns.Write(chunks.Bytes())
	return os.WriteFile("assets/ducky.icns", icns.Bytes(), 0644)
}
