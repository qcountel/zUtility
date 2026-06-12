package modules

import (
	"fmt"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/pkg/e"
	"github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
)

var noParticleSettings = modulesutil.Settings{
	Signature: mem.MustParseSignature("E8 ? ? ? ? FF 84 B7"),
	PatchFunc: modulesutil.PatchFuncExtendNop(5),
}

var _ module.Config = (*NoParticle)(nil)

type NoParticle struct {
	modulesutil.DefaultDisabled
	Process  *win.Process
	Error    func(error)
	OnToggle func(bool, e.ActionCause)
}

func (conf *NoParticle) Create(p module.Property, cause e.ActionCause) (module.Module, error) {
	c := &modulesutil.ByteToggleModule{
		Settings: noParticleSettings,
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
		"no particle",
		"disables particle rendering",
	)
	m.Edit(p, cause)
	return m, nil
}

// Identifier ...
func (*NoParticle) Identifier() string {
	return "no_particle"
}
