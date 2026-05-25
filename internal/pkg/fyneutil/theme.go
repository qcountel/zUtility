package fyneutil

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// AccentColor is set by the app package when palette changes.
var AccentColor color.Color = hex(0xCC, 0x33, 0x33)

type CrimsonDarkTheme struct{}

var _ fyne.Theme = (*CrimsonDarkTheme)(nil)

func hex(r, g, b uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}
}

func (t *CrimsonDarkTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return hex(0x08, 0x08, 0x08)
	case theme.ColorNameButton:
		return hex(0x0D, 0x0D, 0x0D)
	case theme.ColorNameDisabledButton:
		return hex(0x0D, 0x0D, 0x0D)
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0x08, G: 0x08, B: 0x08, A: 0xF2}
	case theme.ColorNameMenuBackground:
		return hex(0x0D, 0x0D, 0x0D)
	case theme.ColorNameHeaderBackground:
		return hex(0x0D, 0x0D, 0x0D)
	case theme.ColorNameInputBackground:
		return hex(0x0D, 0x0D, 0x0D)
	case theme.ColorNameForeground:
		return hex(0xE0, 0xE0, 0xE0)
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 0x44, G: 0x44, B: 0x44, A: 0xFF}
	case theme.ColorNamePrimary:
		return AccentColor
	case theme.ColorNameFocus:
		return AccentColor
	case theme.ColorNameHyperlink:
		return AccentColor
	case theme.ColorNameHover:
		if c, ok := AccentColor.(color.NRGBA); ok {
			return color.NRGBA{R: c.R, G: c.G, B: c.B, A: 0x22}
		}
		return color.NRGBA{R: 0xCC, G: 0x22, B: 0x22, A: 0x22}
	case theme.ColorNamePressed:
		if c, ok := AccentColor.(color.NRGBA); ok {
			return color.NRGBA{R: c.R, G: c.G, B: c.B, A: 0x35}
		}
		return color.NRGBA{R: 0xCC, G: 0x22, B: 0x22, A: 0x35}
	case theme.ColorNameSelection:
		if c, ok := AccentColor.(color.NRGBA); ok {
			return color.NRGBA{R: c.R, G: c.G, B: c.B, A: 0x45}
		}
		return color.NRGBA{R: 0xCC, G: 0x22, B: 0x22, A: 0x45}
	case theme.ColorNameInputBorder:
		return hex(0x1A, 0x1A, 0x1A)
	case theme.ColorNameSeparator:
		return hex(0x1A, 0x1A, 0x1A)
	case theme.ColorNameScrollBar:
		if c, ok := AccentColor.(color.NRGBA); ok {
			return color.NRGBA{R: c.R, G: c.G, B: c.B, A: 0x50}
		}
		return color.NRGBA{R: 0xCC, G: 0x22, B: 0x22, A: 0x50}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x90}
	case theme.ColorNameSuccess:
		return hex(0x33, 0xBB, 0x55)
	case theme.ColorNameWarning:
		return hex(0xDD, 0x77, 0x22)
	case theme.ColorNameError:
		return hex(0xFF, 0x44, 0x44)
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (t *CrimsonDarkTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *CrimsonDarkTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *CrimsonDarkTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 5
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 22
	case theme.SizeNameSubHeadingText:
		return 17
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameScrollBar:
		return 0
	case theme.SizeNameScrollBarSmall:
		return 0
	case theme.SizeNameSeparatorThickness:
		return 1
	}
	return theme.DefaultTheme().Size(name)
}
