package modulesutil

import (
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	win "github.com/qcountel/zUtility/pkg/winlegacy"
)

type SigToggleModule struct {
	Signature   []byte
	Offset      uintptr
	Process     *win.Process
	Error       func(error)
	AfterChange func()

	mu          sync.Mutex
	toggler     *win.SignatureNopToggler
	uiToggle    *M3Toggle
	wantEnabled bool
}

func (m *SigToggleModule) CreateObjects() []fyne.CanvasObject {
	toggle := NewM3Toggle(m.wantEnabled)
	toggle.OnChange = func(b bool) {
		m.mu.Lock()
		m.wantEnabled = b
		m.mu.Unlock()
		go func(enabled bool) {
			toggler, err := m.lazyToggler()
			if err != nil {
				if m.Error != nil {
					m.Error(fmt.Errorf("get sig toggler: %w", err))
				}
				return
			}
			if err = toggler.Set(enabled); err != nil {
				if m.Error != nil {
					m.Error(fmt.Errorf("update sig toggler: %w", err))
				}
				return
			}
			if m.AfterChange != nil {
				m.AfterChange()
			}
		}(b)
	}
	m.uiToggle = toggle
	return []fyne.CanvasObject{toggle}
}

func (m *SigToggleModule) lazyToggler() (*win.SignatureNopToggler, error) {
	m.mu.Lock()
	if m.toggler != nil {
		m.mu.Unlock()
		return m.toggler, nil
	}
	m.mu.Unlock()

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

	m.mu.Lock()
	m.toggler = t
	m.mu.Unlock()

	// Не вызываем Set(Enabled()) — состояние уже установлено внутри conf.New()
	// при сканировании адреса (если нашёл NOP-сигнатуру — state = true).
	return t, nil
}

func (m *SigToggleModule) Disable() {
	m.mu.Lock()
	m.wantEnabled = false
	toggler := m.toggler
	m.mu.Unlock()
	
	if toggler != nil && toggler.Enabled() {
		_ = toggler.Set(false)
	}
	if m.uiToggle != nil {
		prev := m.uiToggle.OnChange
		m.uiToggle.OnChange = nil
		m.uiToggle.SetChecked(false)
		m.uiToggle.OnChange = prev
	}
}

func (m *SigToggleModule) Enable() {
	m.mu.Lock()
	m.wantEnabled = true
	m.mu.Unlock()

	if m.uiToggle != nil {
		prev := m.uiToggle.OnChange
		m.uiToggle.OnChange = nil
		m.uiToggle.SetChecked(true)
		m.uiToggle.OnChange = prev
	}

	go func() {
		toggler, err := m.lazyToggler()
		if err != nil {
			if m.Error != nil {
				m.Error(fmt.Errorf("enable module: %w", err))
			}
		} else {
			_ = toggler.Set(true)
		}
	}()
}

func (m *SigToggleModule) IsEnabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.wantEnabled
}
