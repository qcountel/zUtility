package fyneutil

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

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
		return hex(0x14, 0x0A, 0x0A)
	case theme.ColorNameDisabledButton:
		return hex(0x14, 0x0A, 0x0A)
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0x08, G: 0x04, B: 0x04, A: 0xF2}
	case theme.ColorNameMenuBackground:
		return hex(0x11, 0x08, 0x08)
	case theme.ColorNameHeaderBackground:
		return hex(0x11, 0x08, 0x08)
	case theme.ColorNameInputBackground:
		return hex(0x16, 0x0C, 0x0C)
	case theme.ColorNameForeground:
		return hex(0xF0, 0xF0, 0xF0)
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 0x55, G: 0x44, B: 0x44, A: 0xFF}
	case theme.ColorNamePrimary:
		return hex(0xCC, 0x22, 0x22)
	case theme.ColorNameFocus:
		return hex(0xCC, 0x22, 0x22)
	case theme.ColorNameHyperlink:
		return hex(0xE5, 0x44, 0x44)
	case theme.ColorNameHover:
		return color.NRGBA{R: 0xCC, G: 0x22, B: 0x22, A: 0x22}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 0xCC, G: 0x22, B: 0x22, A: 0x35}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0xCC, G: 0x22, B: 0x22, A: 0x45}
	case theme.ColorNameInputBorder:
		return hex(0x28, 0x18, 0x18)
	case theme.ColorNameSeparator:
		return hex(0x22, 0x14, 0x14)
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0xCC, G: 0x22, B: 0x22, A: 0x50}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x90}
	case theme.ColorNameSuccess:
		return hex(0x44, 0xCC, 0x66)
	case theme.ColorNameWarning:
		return hex(0xFF, 0xA0, 0x30)
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
		return 13
	case theme.SizeNameHeadingText:
		return 20
	case theme.SizeNameSubHeadingText:
		return 15
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
