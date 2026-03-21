package modulesutil

import (
	"fmt"

	"fyne.io/fyne/v2"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
)

type ByteToggleModule struct {
	Signature   []byte
	Offset      uintptr
	Original    []byte
	Patch       []byte
	Process     *win.Process
	Error       func(error)
	AfterChange func()

	toggler     *win.ByteToggler
	uiToggle    *M3Toggle
	wantEnabled bool
}

func (m *ByteToggleModule) CreateObjects() []fyne.CanvasObject {
	toggle := NewM3Toggle(m.wantEnabled)

	toggle.OnChange = func(b bool) {
		m.wantEnabled = b
		toggler, err := m.lazyToggler()
		if err != nil {
			if m.Error != nil {
				m.Error(fmt.Errorf("get byte toggler: %w", err))
			}
			return
		}
		if err = toggler.Set(b); err != nil {
			if m.Error != nil {
				m.Error(fmt.Errorf("update byte toggler: %w", err))
			}
			return
		}
		if m.AfterChange != nil {
			m.AfterChange()
		}
	}
	m.uiToggle = toggle
	return []fyne.CanvasObject{toggle}
}

func (m *ByteToggleModule) lazyToggler() (*win.ByteToggler, error) {
	if m.toggler != nil {
		return m.toggler, nil
	}
	addr, err := win.ScanSignature(m.Process, m.Process.ModuleSize, m.Process.Module, m.Signature)
	if err != nil {
		addr, err = win.ScanSignature(m.Process, m.Process.ModuleSize, m.Process.Module, m.Patch)
		if err != nil {
			return nil, fmt.Errorf("signature not found: %w", err)
		}
	}
	addr += m.Offset
	if len(m.Original) == 0 {
		m.Original = m.Signature
	}
	t := &win.ByteToggler{
		Process:  m.Process,
		Address:  addr,
		Original: m.Original,
		Patch:    m.Patch,
	}
	testAddr, _ := win.ScanSignature(m.Process, uintptr(len(m.Patch)), addr, m.Patch)
	if testAddr != 0 {
		t.SetState(true)
	}
	m.toggler = t
	return t, nil
}

func (m *ByteToggleModule) Disable() {
	m.wantEnabled = false
	if m.toggler != nil && m.toggler.Enabled() {
		_ = m.toggler.Set(false)
	}

	if m.uiToggle != nil {
		prev := m.uiToggle.OnChange
		m.uiToggle.OnChange = nil
		m.uiToggle.SetChecked(false)
		m.uiToggle.OnChange = prev
	}
}

func (m *ByteToggleModule) Enable() {
	m.wantEnabled = true
	toggler, err := m.lazyToggler()
	if err != nil {
		if m.Error != nil {
			m.Error(fmt.Errorf("enable module: %w", err))
		}
	} else {
		_ = toggler.Set(true)
	}
	if m.uiToggle != nil {
		prev := m.uiToggle.OnChange
		m.uiToggle.OnChange = nil
		m.uiToggle.SetChecked(true)
		m.uiToggle.OnChange = prev
	}
}

func (m *ByteToggleModule) IsEnabled() bool {
	return m.wantEnabled
}
