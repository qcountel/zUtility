package modulesutil

import (
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/pkg/e"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
)

type Module interface {
	e.ErrorHandler
	Disable(cause e.ActionCause)
	Edit(p module.Property, cause e.ActionCause)
}

type ToggleableModule interface {
	Module
	UpdateState(v bool, cause e.ActionCause, opts ...any) error
	State() bool
}

type ModuleWithValue[T any] interface {
	Module
	SetValue(v T, cause e.ActionCause, opts ...any) error
	Value() (T, bool)
}

type ToggleableModuleWithValue[T any] interface {
	Module
	UpdateState(v bool, cause e.ActionCause, opts ...any) error
	State() bool
	SetValue(v T, cause e.ActionCause, opts ...any) error
	Value() (T, bool)
}

type Settings struct {
	Signature mem.Signature
	Patch     mem.Signature
	PatchFunc func(Settings) mem.Signature //optional; required if Patch is not set
	Offset    uintptr                      //optional
}
