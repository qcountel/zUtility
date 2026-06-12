package modulesutil

import (
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/pkg/e"
)

func NewBaseToggleable(m ToggleableModule, name, desc string) *BaseToggleable {
	return &BaseToggleable{
		ToggleableModule: m,

		name: name,
		desc: desc,
	}
}

var _ module.Module = (*BaseToggleable)(nil)

type BaseToggleable struct {
	ToggleableModule
	name, desc string
}

// Name ...
func (b *BaseToggleable) Name() string {
	return b.name
}

// Description ...
func (b *BaseToggleable) Description() string {
	return b.desc
}

// HandleError ...
func (b *BaseToggleable) HandleError(source string, err error) {
	b.ToggleableModule.HandleError(source, err)
}

// Disable ...
func (b *BaseToggleable) Disable(cause e.ActionCause) {
	b.ToggleableModule.Disable(cause)
}

// Edit ...
func (b *BaseToggleable) Edit(p module.Property, cause e.ActionCause) {
	b.ToggleableModule.Edit(p, cause)
}
