package modules

import (
	"fmt"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/pkg/e"
	"github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
)

var noHurtCamSettings = modulesutil.Settings{
	Signature: mem.MustParseSignature("66 44 0F 6E 83 ? ? ? ? 45 0F 5B C0 44 0F 29 4C 24"),
	PatchFunc: modulesutil.PatchFuncExtendNop(9),
}

var _ module.Config = (*NoHurtCam)(nil)

type NoHurtCam struct {
	modulesutil.DefaultDisabled
	Process  *win.Process
	Error    func(error)
	OnToggle func(bool, e.ActionCause)
}

func (conf *NoHurtCam) Create(p module.Property, cause e.ActionCause) (module.Module, error) {
	c := &modulesutil.ByteToggleModule{
		Settings: noHurtCamSettings,
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
		"no hurt cam",
		"prevents camera shaking when player hurt",
	)
	m.Edit(p, cause)
	return m, nil
}

// Identifier ...
func (*NoHurtCam) Identifier() string {
	return "no_hurt_cam"
}
