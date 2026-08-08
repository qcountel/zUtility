package module

import "fyne.io/fyne/v2"

type Module interface {
	Name() string
	Description() string
	CreateObjects() []fyne.CanvasObject
	Enable()
	Disable()
	IsEnabled() bool
}

type Config interface {
	Create() Module
}
