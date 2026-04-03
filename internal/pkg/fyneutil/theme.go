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
		return hex(0x0F, 0x0F, 0x12) // фон как в HTML #0f0f12
	case theme.ColorNameButton:
		return hex(0x1C, 0x1C, 0x1C)
	case theme.ColorNameDisabledButton:
		return hex(0x1C, 0x1C, 0x1C)
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0x12, G: 0x12, B: 0x12, A: 0xF2}
	case theme.ColorNameMenuBackground:
		return hex(0x1C, 0x1C, 0x1C)
	case theme.ColorNameHeaderBackground:
		return hex(0x1C, 0x1C, 0x1C)
	case theme.ColorNameInputBackground:
		return hex(0x22, 0x22, 0x22)
	case theme.ColorNameForeground:
		return hex(0xE0, 0xE0, 0xE0)
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 0x50, G: 0x50, B: 0x50, A: 0xFF}
	case theme.ColorNamePrimary:
		return hex(0xCC, 0x33, 0x33)
	case theme.ColorNameFocus:
		return hex(0xCC, 0x33, 0x33)
	case theme.ColorNameHyperlink:
		return hex(0xFF, 0x55, 0x55)
	case theme.ColorNameHover:
		// ОЧЕНЬ тихий hover — почти невидимый белый оверлей (4%)
		return color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x0A}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 0xCC, G: 0x33, B: 0x33, A: 0x22}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0xCC, G: 0x33, B: 0x33, A: 0x40}
	case theme.ColorNameInputBorder:
		return hex(0x30, 0x30, 0x30)
	case theme.ColorNameSeparator:
		return hex(0x28, 0x28, 0x28)
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x00} // полностью прозрачный
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
