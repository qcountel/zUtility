package modules

import (
	"fmt"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/pkg/e"
	"github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
)

var itemDelayFixSettings = modulesutil.Settings{
	Signature: mem.MustParseSignature("48 89 86 ? ? ? ? 48 83 7E ? 00"),
	PatchFunc: modulesutil.PatchFuncExtendNop(7),
}

var _ module.Config = (*ItemDelayFix)(nil)

type ItemDelayFix struct {
	modulesutil.DefaultDisabled
	Process  *win.Process
	Error    func(error)
	OnToggle func(bool, e.ActionCause)
}

// Create ...
func (conf *ItemDelayFix) Create(p module.Property, cause e.ActionCause) (module.Module, error) {
	c := &modulesutil.ByteToggleModule{
		Settings: itemDelayFixSettings,
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
		"item delay fix",
		"removes attack use delay of 200 ms",
	)
	m.Edit(p, cause)
	return m, nil
}

// Identifier ...
func (conf *ItemDelayFix) Identifier() string {
	return "item_delay_fix"
}
