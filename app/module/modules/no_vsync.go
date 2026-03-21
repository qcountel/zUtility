package modules

import (
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
)

const (
	offVsyncOffset1 = uintptr(0x72EC22)
	offVsyncOffset2 = uintptr(0x72EC32)
)

var _ module.Module = (*noVsync)(nil)

type NoVsync struct {
	Process     *win.Process
	Error       func(error)
	AfterChange func()
}

func (conf NoVsync) Create() module.Module {
	return &noVsync{
		process:     conf.Process,
		errFn:       conf.Error,
		afterChange: conf.AfterChange,
	}
}

type noVsync struct {
	process     *win.Process
	errFn       func(error)
	afterChange func()

	mu           sync.Mutex
	patch1       *win.ByteToggler
	patch2       *win.ByteToggler
	wantEnabled  bool
	togglersInit bool
	uiToggle     *modulesutil.M3Toggle
}

func (*noVsync) Name() string { return "NoVsync" }
func (*noVsync) Description() string {
	return "Force disables VSync."
}

func (n *noVsync) CreateObjects() []fyne.CanvasObject {
	toggle := modulesutil.NewM3Toggle(n.wantEnabled)
	toggle.OnChange = func(b bool) {
		if err := n.setEnabled(b); err != nil {
			if n.errFn != nil {
				n.errFn(fmt.Errorf("no_vsync: toggle: %w", err))
			}
			return
		}
		if n.afterChange != nil {
			n.afterChange()
		}
	}
	n.uiToggle = toggle
	return []fyne.CanvasObject{toggle}
}

func (n *noVsync) Enable() {
	if err := n.setEnabled(true); err != nil && n.errFn != nil {
		n.errFn(fmt.Errorf("no_vsync: enable: %w", err))
	}
	n.syncToggleUI(true)
}

func (n *noVsync) Disable() {
	if err := n.setEnabled(false); err != nil && n.errFn != nil {
		n.errFn(fmt.Errorf("no_vsync: disable: %w", err))
	}
	n.syncToggleUI(false)
}

func (n *noVsync) IsEnabled() bool { return n.wantEnabled }

func (n *noVsync) setEnabled(b bool) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if err := n.lazyTogglersLocked(); err != nil {
		return err
	}
	if err := n.patch2.Set(b); err != nil {
		return fmt.Errorf("set patch2: %w", err)
	}
	if err := n.patch1.Set(b); err != nil {
		return fmt.Errorf("set patch1: %w", err)
	}
	n.wantEnabled = b
	n.syncToggleUILocked(b)
	return nil
}

func (n *noVsync) lazyTogglersLocked() error {
	if n.togglersInit {
		return nil
	}

	base, _, err := n.process.GetModuleInfo()
	if err != nil {
		return fmt.Errorf("get module info: %w", err)
	}
	addr1 := base + offVsyncOffset1
	addr2 := base + offVsyncOffset2

	n.patch1 = &win.ByteToggler{
		Process:  n.process,
		Address:  addr1,
		Original: []byte{0x1},
		Patch:    []byte{0x0},
	}
	n.patch2 = &win.ByteToggler{
		Process:  n.process,
		Address:  addr2,
		Original: []byte{0x1},
		Patch:    []byte{0x0},
	}

	v2, err := win.ReadMemory[byte](n.process, addr2)
	if err == nil {
		enabled := v2 == 0x0
		n.patch1.SetState(enabled)
		n.patch2.SetState(enabled)
		n.wantEnabled = enabled
	}

	n.togglersInit = true
	return nil
}

func (n *noVsync) syncToggleUI(b bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.syncToggleUILocked(b)
}

func (n *noVsync) syncToggleUILocked(b bool) {
	if n.uiToggle == nil {
		return
	}
	prev := n.uiToggle.OnChange
	n.uiToggle.OnChange = nil
	n.uiToggle.SetChecked(b)
	n.uiToggle.OnChange = prev
}
