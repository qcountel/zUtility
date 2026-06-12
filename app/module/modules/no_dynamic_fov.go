package modules

import (
	"fmt"
	"strings"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/pkg/e"
	"github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
)

var noDynamicFovSettings = modulesutil.Settings{
	Signature: mem.MustParseSignature("F3 0F 11 83 ? ? ? ? 48 8B 83 ? ? ? ? 48 8B 48"),
	PatchFunc: modulesutil.PatchFuncExtendNop(8), // patch first 8 bytes of sig to nop
}

var _ module.Config = (*NoDynamicFov)(nil)

type NoDynamicFov struct {
	modulesutil.DefaultDisabled
	Process  *win.Process
	Error    func(error)
	OnToggle func(bool, e.ActionCause)
}

func (conf *NoDynamicFov) Create(p module.Property, cause e.ActionCause) (module.Module, error) {
	c := &modulesutil.ByteToggleModule{
		Settings: noDynamicFovSettings,
		Process:  conf.Process,
		Error:    conf.Error,
	}
	cl := &fovClamper{proc: conf.Process, err: conf.Error}
	c.OnToggle = func(b bool, cause e.ActionCause) {
		cl.clamp()
		conf.OnToggle(b, cause)
	}
	b, err := c.New()
	if err != nil {
		return nil, fmt.Errorf("create byte toggle module: %w", err)
	}
	if cause == nil {
		cause = e.ActionCauseExternal
	}
	m := modulesutil.NewBaseToggleable(b,
		"no dynamic fov",
		"prevents game to write new field of view dynamically",
	)
	m.Edit(p, cause)
	return m, nil
}

// Identifier ...
func (*NoDynamicFov) Identifier() string {
	return "no_dynamic_fov"
}

var fovPtr = mem.MustParsePointer(
	"01921DF8", //+module
	"30 D8 20 BE0",
)

const maxStaticFloatValue float32 = 1.0

type fovClamper struct {
	proc *win.Process
	err  func(error)

	addr uintptr
}

func (m *fovClamper) clamp() {
	addr, ok := m.lazyAddress()
	if !ok {
		return
	}
	v, err := mem.ReadMemory[float32](m.proc, addr)
	if err != nil {
		m.addr = 0 // force recalculate address
		m.err(fmt.Errorf("read fov ptr from lazy address: %w", err))
		return
	}
	if v <= maxStaticFloatValue {
		return
	}
	// force write max static fov value
	if err = mem.WriteMemory(m.proc, addr, maxStaticFloatValue); err != nil {
		m.err(fmt.Errorf("write fov ptr (current value overflows max static value): %w", err))
	}
}

func (m *fovClamper) lazyAddress() (uintptr, bool) {
	if m.addr != 0 {
		return m.addr, true
	}
	addr, err := mem.ResolvePointerAddress(m.proc, fovPtr)
	if err != nil {
		if strings.Contains(err.Error(), "Only part of a ReadProcessMemory or WriteProcessMemory request was completed") {
			// hack: prevents error logs when player is not in world
			return 0, false
		}
		m.err(fmt.Errorf("resolve fov ptr addr: %w", err))
		return 0, false
	}
	m.addr = addr
	return addr, true
}
