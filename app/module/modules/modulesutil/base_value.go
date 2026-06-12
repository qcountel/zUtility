package modulesutil

import (
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/pkg/e"
)

func NewBaseValue[T any](m ModuleWithValue[T], name, desc string) *BaseValue[T] {
	return &BaseValue[T]{
		ModuleWithValue: m,

		name: name,
		desc: desc,
	}
}

var _ module.Module = (*BaseValue[struct{}])(nil)

type BaseValue[T any] struct {
	ModuleWithValue[T]
	name, desc string
}

// Name ...
func (b *BaseValue[T]) Name() string {
	return b.name
}

// Description ...
func (b *BaseValue[T]) Description() string {
	return b.desc
}

// HandleError ...
func (b *BaseValue[T]) HandleError(source string, err error) {
	b.ModuleWithValue.HandleError(source, err)
}

// Disable ...
func (b *BaseValue[T]) Disable(cause e.ActionCause) {
	b.ModuleWithValue.Disable(cause)
}

// Edit ...
func (b *BaseValue[T]) Edit(p module.Property, cause e.ActionCause) {
	b.ModuleWithValue.Edit(p, cause)
}
