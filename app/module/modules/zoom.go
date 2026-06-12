package modules

import (
	"fmt"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/pkg/e"
	"github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
)

var zoomSettings = modulesutil.Settings{
	Signature: mem.MustParseSignature("F3 0F 10 89 ? ? ? ? 0F 2F F1"),
	PatchFunc: modulesutil.PatchFuncExtendNop(8),
}

var _ module.Config = (*Zoom)(nil)

type Zoom struct {
	modulesutil.DefaultDisabled
	Process  *win.Process
	Error    func(error)
	OnToggle func(bool, e.ActionCause)
}

// Create ...
func (conf *Zoom) Create(p module.Property, cause e.ActionCause) (module.Module, error) {
	c := &modulesutil.ByteToggleModule{
		Settings: zoomSettings,
		Process:  conf.Process,
		Error:    conf.Error,
		OnToggle: conf.OnToggle,
	}
	b, err := c.New()
	if err != nil {
		return nil, fmt.Errorf("create byte toggler module: %w", err)
	}
	if cause == nil {
		cause = e.ActionCauseExternal
	}
	m := modulesutil.NewBaseToggleable(b,
		"zoom",
		"...",
	)
	m.Edit(p, cause)
	return m, nil
}

// Identifier ...
func (conf *Zoom) Identifier() string {
	return "zoom"
}
