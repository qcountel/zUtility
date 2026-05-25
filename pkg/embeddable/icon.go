package embeddable

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.png
var iconData []byte

func LoadIcon() (fyne.Resource, error) {
	return fyne.NewStaticResource("icon.png", iconData), nil
}
