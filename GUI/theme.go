package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// cdfTheme — dark, terminal-aesthetic palette matching the CDF spec.
// Extends fyne's default theme; overrides colors and a few sizes only.

type cdfTheme struct{}

var _ fyne.Theme = (*cdfTheme)(nil)

var (
	colBg          = color.NRGBA{R: 0x0d, G: 0x0e, B: 0x11, A: 0xff}
	colSurface     = color.NRGBA{R: 0x14, G: 0x16, B: 0x1b, A: 0xff}
	colSurface2    = color.NRGBA{R: 0x1a, G: 0x1d, B: 0x24, A: 0xff}
	colBorder      = color.NRGBA{R: 0x2a, G: 0x2e, B: 0x37, A: 0xff}
	colBorderStr   = color.NRGBA{R: 0x3a, G: 0x3f, B: 0x4b, A: 0xff}
	colInk         = color.NRGBA{R: 0xe8, G: 0xeb, B: 0xef, A: 0xff}
	colMuted       = color.NRGBA{R: 0x7a, G: 0x80, B: 0x90, A: 0xff}
	colAccent      = color.NRGBA{R: 0x4f, G: 0xd1, B: 0xc5, A: 0xff}
	colAccentDim   = color.NRGBA{R: 0x16, G: 0x3a, B: 0x37, A: 0xff}
	colWarn        = color.NRGBA{R: 0xd4, G: 0xa9, B: 0x4a, A: 0xff}
	colDanger      = color.NRGBA{R: 0xe0, G: 0x70, B: 0x70, A: 0xff}
	colFocus       = color.NRGBA{R: 0x4f, G: 0xd1, B: 0xc5, A: 0x80}
	colHover       = color.NRGBA{R: 0x22, G: 0x25, B: 0x2d, A: 0xff}
)

func (cdfTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colBg
	case theme.ColorNameForeground, theme.ColorNameForegroundOnPrimary:
		return colInk
	case theme.ColorNameDisabled:
		return colMuted
	case theme.ColorNamePlaceHolder:
		return colMuted
	case theme.ColorNamePrimary:
		return colAccent
	case theme.ColorNameFocus:
		return colFocus
	case theme.ColorNameHover:
		return colHover
	case theme.ColorNamePressed:
		return colAccentDim
	case theme.ColorNameSelection:
		return colAccentDim
	case theme.ColorNameSeparator, theme.ColorNameShadow:
		return colBorder
	case theme.ColorNameButton, theme.ColorNameInputBackground, theme.ColorNameOverlayBackground, theme.ColorNameMenuBackground:
		return colSurface
	case theme.ColorNameInputBorder:
		return colBorderStr
	case theme.ColorNameError:
		return colDanger
	case theme.ColorNameWarning:
		return colWarn
	case theme.ColorNameSuccess:
		return colAccent
	case theme.ColorNameScrollBar:
		return colBorderStr
	}
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}

func (cdfTheme) Font(s fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(s)
}

func (cdfTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}

func (cdfTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameText:
		return 13
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 6
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameScrollBar:
		return 10
	case theme.SizeNameSeparatorThickness:
		return 1
	}
	return theme.DefaultTheme().Size(n)
}
