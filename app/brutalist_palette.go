package app

import (
	"image/color"

	"github.com/qcountel/zUtility/app/module/modules/modulesutil"
)

// ─── Brutalist palette system ─────────────────────────────────────────
// Fixed background/text colors (do not change with palette).
// Accent colors are mutable — updated by SetPalette().

// chex creates an opaque NRGBA from r, g, b.
func chex(r, g, b uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: 0xFF}
}

// calpha returns c with the given alpha.
func calpha(c color.NRGBA, a uint8) color.NRGBA {
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: a}
}

// ─── Fixed brutalist colors ──────────────────────────────────────────

var (
	bgPrimary  = chex(0x08, 0x08, 0x08)
	bgSurface  = chex(0x0D, 0x0D, 0x0D)
	bgCard     = color.NRGBA{R: 0x0D, G: 0x0D, B: 0x0D, A: 0xFF}
	bgElevated = chex(0x11, 0x11, 0x11)
	bgFooter   = chex(0x05, 0x05, 0x05)
	bgSlider   = chex(0x09, 0x09, 0x09)

	textPrimary   = chex(0xE0, 0xE0, 0xE0)
	textSecondary = chex(0x88, 0x88, 0x88)
	textDim       = chex(0x2A, 0x2A, 0x2A)
	textGhost     = chex(0x33, 0x33, 0x33)

	borderSubtle = chex(0x1A, 0x1A, 0x1A)
	borderRow    = chex(0x11, 0x11, 0x11)

	accentOrange = chex(0xDD, 0x77, 0x22)
	accentGreen  = chex(0x33, 0xBB, 0x55)
)

// ─── Palette definition ──────────────────────────────────────────────

// Palette holds an accent color scheme.
type Palette struct {
	Name         string
	DisplayName  string
	Accent       color.NRGBA
	AccentBright color.NRGBA
}

// Palettes is the list of available accent palettes.
var Palettes = []Palette{
	{"red", "Red", chex(0xCC, 0x33, 0x33), chex(0xFF, 0x55, 0x55)},
	{"blue", "Blue", chex(0x33, 0x66, 0xFF), chex(0x58, 0x84, 0xFF)},
	{"green", "Green", chex(0x00, 0xCC, 0x66), chex(0x00, 0xFF, 0x80)},
	{"violet", "Violet", chex(0x99, 0x44, 0xEE), chex(0xB4, 0x64, 0xFF)},
	{"amber", "Amber", chex(0xEE, 0x88, 0x00), chex(0xFF, 0xAA, 0x2C)},
	{"cyan", "Cyan", chex(0x00, 0xBB, 0xCC), chex(0x00, 0xE1, 0xF0)},
	{"pink", "Pink", chex(0xEE, 0x33, 0x88), chex(0xFF, 0x58, 0xA8)},
}

// ─── Mutable accent colors (updated by SetPalette) ───────────────────
// Names kept for backward-compat with existing code.

var (
	accentRed       = Palettes[0].Accent
	accentRedBright = Palettes[0].AccentBright
	accentRedDim    = chex(0x7A, 0x1A, 0x1A)
	borderRed       = color.NRGBA{R: 0xCC, G: 0x33, B: 0x33, A: 0x20}

	currentPaletteName = "red"
)

// SetPalette switches the accent color scheme.
// All accent-related vars are updated; UI must be rebuilt after calling this.
func SetPalette(name string) {
	for _, p := range Palettes {
		if p.Name == name {
			accentRed = p.Accent
			accentRedBright = p.AccentBright
			accentRedDim = chex(p.Accent.R/2, p.Accent.G/2, p.Accent.B/2)
			borderRed = color.NRGBA{R: p.Accent.R, G: p.Accent.G, B: p.Accent.B, A: 0x20}
			currentPaletteName = p.Name
			
			modulesutil.UpdateAccentColor(p.Accent)
			return
		}
	}
}

// CurrentPaletteName returns the name of the current accent palette.
func CurrentPaletteName() string {
	return currentPaletteName
}

// accentAlpha returns the current accent with custom alpha.
func accentAlpha(a uint8) color.NRGBA {
	return color.NRGBA{R: accentRed.R, G: accentRed.G, B: accentRed.B, A: a}
}
