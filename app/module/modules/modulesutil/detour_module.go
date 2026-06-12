package modulesutil

import (
	"errors"
	"fmt"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/pkg/e"
	"github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
	"github.com/something-that-is-cool/zutil/pkg/win/mem/memutil"
)

type DetourToggleModule struct {
	Settings Settings
	Address  uintptr

	TargetSize uint
	UserCode   []byte
	Process    *win.Process

	Error    func(error)
	OnToggle func(bool, e.ActionCause)
}

func (conf DetourToggleModule) New() (t ToggleableModule, err error) {
	if len(conf.UserCode) == 0 {
		return nil, errors.New("empty user code")
	}
	if conf.Address <= 0 && conf.Settings.Signature.Empty() {
		return nil, errors.New("empty address")
	}
	if conf.OnToggle == nil {
		conf.OnToggle = func(bool, e.ActionCause) {}
	}
	addr := conf.Address
	if addr <= 0 {
		addr, err = mem.ScanSignature(conf.Process, conf.Settings.Signature)
		if err != nil {
			return nil, fmt.Errorf("scan sig: %w", err)
		}
	}
	addr += conf.Settings.Offset

	m := &detourToggleModule{
		ErrorHandler: ErrorHandler{Error: conf.Error},
		proc:         conf.Process,
		detour:       memutil.NewDetour(conf.Process, addr, 0, conf.TargetSize),
		userCode:     conf.UserCode,
		onToggle:     conf.OnToggle,
	}

	return m, nil
}

type detourToggleModule struct {
	e.ErrorHandler
	proc     *win.Process
	detour   *memutil.Detour
	userCode []byte
	state    bool
	onToggle func(bool, e.ActionCause)
}

func (m *detourToggleModule) UpdateState(v bool, cause e.ActionCause, opts ...any) error {
	if m.state == v {
		return e.ErrValuesIsAlready{Value: v}
	}
	if cause == nil {
		cause = e.ActionCauseExternal
	}
	if v {
		if err := m.detour.EnableWithCode(m.userCode); err != nil {
			return fmt.Errorf("enable detour: %w", err)
		}
	} else {
		if err := m.detour.Disable(); err != nil {
			return fmt.Errorf("disable detour: %w", err)
		}
	}
	m.state = v
	m.onToggle(v, cause)
	return nil
}

func (m *detourToggleModule) State() bool {
	return m.state
}

func (m *detourToggleModule) Disable(cause e.ActionCause) {
	m.HandleError("disable detour module", disableOnlyAction(m, cause))
}

func (m *detourToggleModule) Edit(p module.Property, cause e.ActionCause) {
	SyncState(m, p, cause)
}
