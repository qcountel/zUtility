package modules

import (
	"github.com/qcountel/zUtility/app/module"
	"github.com/qcountel/zUtility/app/module/modules/modulesutil"
	win "github.com/qcountel/zUtility/pkg/winlegacy"
)

var _ module.Module = (*autoSprint)(nil)

var (
	autoSprintSig   = []byte{0x0F, 0xB6, 0x41, 0x63, 0x40, 0x32, 0xED}
	autoSprintPatch = []byte{0x66, 0xB8, 0x01, 0x00, 0x40, 0x30, 0xED}
)

type AutoSprint struct {
	Process     *win.Process
	Error       func(error)
	AfterChange func()
}

func (conf AutoSprint) Create() module.Module {
	return &autoSprint{ByteToggleModule: &modulesutil.ByteToggleModule{
		Signature:   autoSprintSig,
		Patch:       autoSprintPatch,
		Process:     conf.Process,
		Error:       conf.Error,
		AfterChange: conf.AfterChange,
	}}
}

type autoSprint struct {
	*modulesutil.ByteToggleModule
}

func (*autoSprint) Name() string { return "AutoSprint" }
func (*autoSprint) Description() string {
	return "Постоянный спринт без удержания клавиши."
}
