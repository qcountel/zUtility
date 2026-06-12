package modules

import (
	"fmt"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/pkg/e"
	"github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem/memutil"
)

var (
	offVsyncOffset1 uintptr = 0x72EC22
	offVsyncOffset2         = offVsyncOffset1 + 0x10
)

type OffVsync struct {
	modulesutil.DefaultDisabled
	Process  *win.Process
	Error    func(error)
	OnToggle func(bool, e.ActionCause)
}

// Create ...
func (conf OffVsync) Create(p module.Property, cause e.ActionCause) (module.Module, error) {
	mod, err := conf.Process.GetModuleInfo()
	if err != nil {
		return nil, fmt.Errorf("get process module: %w", err)
	}
	if conf.OnToggle == nil {
		conf.OnToggle = func(bool, e.ActionCause) {}
	}
	a1 := mod.Address + offVsyncOffset1
	a2 := mod.Address + offVsyncOffset2

	t1 := &memutil.ByteToggler{
		Process:  conf.Process,
		Address:  a1,
		Original: []byte{0x1},
		Patch:    []byte{0x0},
	}
	t2 := &memutil.ByteToggler{
		Process:  conf.Process,
		Address:  a2,
		Original: []byte{0x1},
		Patch:    []byte{0x0},
	}

	o := &offVsync{
		ErrorHandler: modulesutil.ErrorHandler{Error: conf.Error},
		p1:           t1,
		p2:           t2,
		onToggle:     conf.OnToggle,
	}
	
	// Perform initial read state sync if available
	// Check if already patched
	if t1.Enabled() && t2.Enabled() {
		o.state = true
	}

	m := modulesutil.NewBaseToggleable(o,
		"off vsync",
		"force disables vertical synchronization",
	)
	m.Edit(p, cause)
	return m, nil
}

// Identifier ...
func (conf OffVsync) Identifier() string {
	return "off_vsync"
}

var _ modulesutil.ToggleableModule = (*offVsync)(nil)

type offVsync struct {
	e.ErrorHandler
	p1, p2 *memutil.ByteToggler
	state  bool
	onToggle func(bool, e.ActionCause)
}

// Edit ...
func (o *offVsync) Edit(p module.Property, cause e.ActionCause) {
	modulesutil.SyncState(o, p, cause)
}

// Disable ...
func (o *offVsync) Disable(cause e.ActionCause) {
	o.HandleError("disable vsync", o.UpdateState(false, cause))
}

// UpdateState ...
func (o *offVsync) UpdateState(v bool, cause e.ActionCause, opts ...any) error {
	if o.state == v {
		return e.ErrValuesIsAlready{Value: v}
	}
	if cause == nil {
		cause = e.ActionCauseExternal
	}
	if err := o.p1.Set(v); err != nil {
		return fmt.Errorf("patch 1: %w", err)
	}
	if err := o.p2.Set(v); err != nil {
		return fmt.Errorf("patch 2: %w", err)
	}
	o.state = v
	o.onToggle(v, cause)
	return nil
}

// State ...
func (o *offVsync) State() bool {
	return o.state
}
