package main

// icon.go — the app/window icon, embedded so the binary carries it.
// Source art: ../dist/build/cdf-viewer.svg (rendered at 256 px).

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed cdf-viewer.png
var iconPNG []byte

var appIcon = fyne.NewStaticResource("cdf-viewer.png", iconPNG)
