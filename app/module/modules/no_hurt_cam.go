package modules

import (
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
)

var _ module.Module = (*noHurtCam)(nil)

var noHurtCamSig = []byte{0x66, 0x44, 0x0F, 0x6E, 0x83, 0x6C, 0x0E, 0x00, 0x00}

type NoHurtCam struct {
	Process     *win.Process
	Error       func(error)
	AfterChange func()
}

func (conf NoHurtCam) Create() module.Module {
	return &noHurtCam{SigToggleModule: &modulesutil.SigToggleModule{
		Signature:   noHurtCamSig,
		Process:     conf.Process,
		Error:       conf.Error,
		AfterChange: conf.AfterChange,
	}}
}

type noHurtCam struct {
	*modulesutil.SigToggleModule
}

func (*noHurtCam) Name() string { return "NoHurtCam" }
func (*noHurtCam) Description() string {
	return "Отключает тряску камеры при получении урона."
}
