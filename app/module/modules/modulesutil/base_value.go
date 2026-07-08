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

func (b *BaseValue[T]) Min() float64 {
	if fm, ok := b.ModuleWithValue.(interface {
		Min() float64
	}); ok {
		return fm.Min()
	}
	return 1
}

func (b *BaseValue[T]) Max() float64 {
	if fm, ok := b.ModuleWithValue.(interface {
		Max() float64
	}); ok {
		return fm.Max()
	}
	return 300
}

func (b *BaseValue[T]) Def() float64 {
	if fm, ok := b.ModuleWithValue.(interface {
		Def() float64
	}); ok {
		return fm.Def()
	}
	return 100
}

func (b *BaseValue[T]) Step() float64 {
	if fm, ok := b.ModuleWithValue.(interface {
		Step() float64
	}); ok {
		return fm.Step()
	}
	return 1
}
