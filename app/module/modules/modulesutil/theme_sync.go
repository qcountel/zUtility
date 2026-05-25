package modulesutil

import "image/color"

func UpdateAccentColor(c color.NRGBA) {
	m3ToggleBgOn = c
	sliderActiveColor = c
}
