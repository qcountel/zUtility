package modulesutil

import (
	"fmt"

	"fyne.io/fyne/v2"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
)

type SigToggleModule struct {
	Signature   []byte
	Offset      uintptr
	Process     *win.Process
	Error       func(error)
	AfterChange func()

	toggler     *win.SignatureNopToggler
	uiToggle    *M3Toggle
	wantEnabled bool
}

func (m *SigToggleModule) CreateObjects() []fyne.CanvasObject {
	toggle := NewM3Toggle(m.wantEnabled)
	toggle.OnChange = func(b bool) {
		m.wantEnabled = b
		toggler, err := m.lazyToggler()
		if err != nil {
			if m.Error != nil {
				m.Error(fmt.Errorf("get sig toggler: %w", err))
			}
			return
		}
		if err = toggler.Set(b); err != nil {
			if m.Error != nil {
				m.Error(fmt.Errorf("update sig toggler: %w", err))
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

func (m *SigToggleModule) lazyToggler() (*win.SignatureNopToggler, error) {
	if m.toggler != nil {
		return m.toggler, nil
	}
	conf := win.SignatureNopTogglerConfig{
		Process:   m.Process,
		Module:    m.Process.Module,
		Size:      m.Process.ModuleSize,
		Signature: m.Signature,
	}
	t, err := conf.New()
	if err != nil {
		return nil, fmt.Errorf("init toggler: %w", err)
	}
	m.toggler = t
	return t, nil
}

func (m *SigToggleModule) Disable() {
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

func (m *SigToggleModule) Enable() {
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

func (m *SigToggleModule) IsEnabled() bool {
	return m.wantEnabled
}
