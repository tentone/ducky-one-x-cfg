// Package assets embeds the duck artwork so installed apps need no asset files.
package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed ducky.svg
var duckSVG []byte

//go:embed ducky.png
var duckPNG []byte

var (
	Duck    = fyne.NewStaticResource("ducky.svg", duckSVG)
	AppIcon = fyne.NewStaticResource("ducky.png", duckPNG)
)
