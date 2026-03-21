package modules

import (
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
)

var _ module.Module = (*itemUseDelay)(nil)

var itemUseDelaySig = []byte{0x48, 0x89, 0x86, 0x88, 0x00, 0x00, 0x00, 0x48}

type ItemUseDelay struct {
	Process     *win.Process
	Error       func(error)
	AfterChange func()
}

func (conf ItemUseDelay) Create() module.Module {
	return &itemUseDelay{SigToggleModule: &modulesutil.SigToggleModule{
		Signature:   itemUseDelaySig,
		Process:     conf.Process,
		Error:       conf.Error,
		AfterChange: conf.AfterChange,
	}}
}

type itemUseDelay struct {
	*modulesutil.SigToggleModule
}

func (*itemUseDelay) Name() string { return "ItemUseDelay" }
func (*itemUseDelay) Description() string {
	return "Убирает задержку 200 мс после атаки при использовании предметов."
}
