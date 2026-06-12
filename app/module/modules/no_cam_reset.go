package modules

import (
	"fmt"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/pkg/e"
	"github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
)

var noCamResetSettings = modulesutil.Settings{
	Signature: mem.MustParseSignature("FF 90 ? ? ? ? ? ? ? 48 8B D6 44 8B 4C 24"), //call qword ptr [rax+88h] (2)
	PatchFunc: modulesutil.PatchFuncExtendNop(2),
}

var _ module.Config = (*NoCamReset)(nil)

type NoCamReset struct {
	modulesutil.DefaultDisabled
	Process  *win.Process
	Error    func(error)
	OnToggle func(bool, e.ActionCause)
}

// Create ...
func (conf *NoCamReset) Create(p module.Property, cause e.ActionCause) (module.Module, error) {
	c := &modulesutil.ByteToggleModule{
		Settings: noCamResetSettings,
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
		"no cam reset",
		"prevents teleport rotation interpolation",
	)
	m.Edit(p, cause)
	return m, nil
}

// Identifier ...
func (conf *NoCamReset) Identifier() string {
	return "no_cam_reset"
}
