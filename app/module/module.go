package module

import (
	"github.com/something-that-is-cool/zutil/pkg/e"
)

type Module interface {
	Name() string
	Description() string
	Edit(p Property, cause e.ActionCause)
	Disable(cause e.ActionCause) // must not be fyne-obsessed
}

type Config interface {
	Create(p Property, cause e.ActionCause) (Module, error)
	DefaultProperty() Property
	Identifier() string // identify in config
}

type Property struct {
	Enabled bool `json:"enabled,omitempty"`
	Value   any  `json:"value,omitempty"`
}
