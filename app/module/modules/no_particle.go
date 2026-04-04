package modules

import (
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
)

var _ module.Module = (*noParticle)(nil)

var particleSig = []byte{0xE8, 0x68, 0x4F, 0xCF, 0xFF}

type NoParticle struct {
	Process     *win.Process
	Error       func(error)
	AfterChange func()
}

func (conf NoParticle) Create() module.Module {
	return &noParticle{SigToggleModule: &modulesutil.SigToggleModule{
		Signature:   particleSig,
		Process:     conf.Process,
		Error:       conf.Error,
		AfterChange: conf.AfterChange,
	}}
}

type noParticle struct {
	*modulesutil.SigToggleModule
}

func (*noParticle) Name() string { return "NoParticle" }
func (*noParticle) Description() string {
	return "Отключает частицы для более чистой картинки и высокого FPS."
}
