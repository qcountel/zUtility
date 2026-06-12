package modules

import (
	"fmt"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/pkg/asm"
	"github.com/something-that-is-cool/zutil/pkg/e"
	"github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
)

var autoSprintSettings = modulesutil.Settings{
	Signature: mem.MustParseSignature("0F B6 41 ? 40 32 ED"),
	PatchFunc: modulesutil.PatchFuncExtendBuilder(asm.Build().
		MovAxImm8(0x1).
		X().
		XorChBpl(),
	),
}

var _ module.Config = (*AutoSprint)(nil)

type AutoSprint struct {
	modulesutil.DefaultDisabled
	Process  *win.Process
	Error    func(error)
	OnToggle func(bool, e.ActionCause)
}

func (conf *AutoSprint) Create(p module.Property, cause e.ActionCause) (module.Module, error) {
	c := &modulesutil.ByteToggleModule{
		Settings: autoSprintSettings,
		Process:  conf.Process,
		Error:    conf.Error,
		OnToggle: conf.OnToggle,
	}
	b, err := c.New()
	if err != nil {
		return nil, fmt.Errorf("create byte toggle module: %w", err)
	}
	if cause == nil {
		cause = e.ActionCauseExternal
	}
	m := modulesutil.NewBaseToggleable(b,
		"auto sprint",
		"automatically sprints for you",
	)
	m.Edit(p, cause)
	return m, nil
}

// Identifier ...
func (*AutoSprint) Identifier() string {
	return "auto_sprint"
}
