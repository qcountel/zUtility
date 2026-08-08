package modulesutil

import (
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	win "github.com/qcountel/zUtility/pkg/winlegacy"
)

type ByteToggleModule struct {
	Signature   []byte
	Offset      uintptr
	Original    []byte
	Patch       []byte
	Process     *win.Process
	Error       func(error)
	AfterChange func()

	mu          sync.Mutex
	toggler     *win.ByteToggler
	uiToggle    *M3Toggle
	wantEnabled bool
	once        sync.Once
	initErr     error
}

func (m *ByteToggleModule) CreateObjects() []fyne.CanvasObject {
	toggle := NewM3Toggle(m.wantEnabled)

	toggle.OnChange = func(b bool) {
		m.mu.Lock()
		m.wantEnabled = b
		m.mu.Unlock()
		go func(enabled bool) {
			toggler, err := m.lazyToggler()
			if err != nil {
				if m.Error != nil {
					m.Error(fmt.Errorf("get byte toggler: %w", err))
				}
				return
			}
			if err = toggler.Set(enabled); err != nil {
				if m.Error != nil {
					m.Error(fmt.Errorf("update byte toggler: %w", err))
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

func (m *ByteToggleModule) lazyToggler() (*win.ByteToggler, error) {
	if m.Process == nil {
		return nil, fmt.Errorf("process is nil")
	}

	m.once.Do(func() {
		addr, err := win.ScanSignature(m.Process, m.Process.ModuleSize, m.Process.Module, m.Signature)
		if err != nil {
			addr, err = win.ScanSignature(m.Process, m.Process.ModuleSize, m.Process.Module, m.Patch)
			if err != nil {
				m.initErr = fmt.Errorf("signature not found: %w", err)
				return
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
	})

	return m.toggler, m.initErr
}

func (m *ByteToggleModule) Disable() {
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

func (m *ByteToggleModule) Enable() {
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
			return
		}
		m.mu.Lock()
		want := m.wantEnabled
		m.mu.Unlock()
		if !want {
			return
		}
		_ = toggler.Set(true)
	}()
}

func (m *ByteToggleModule) IsEnabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.wantEnabled
}
