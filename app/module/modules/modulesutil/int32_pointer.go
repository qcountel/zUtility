package modulesutil

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
)

type Int32PointerModule struct {
	Process     *win.Process
	Error       func(error)
	AfterChange func()

	Min, Max, Default float64
	SliderToMemory    func(float64) int32
	MemoryToSlider    func(int32) float64

	BaseAddress uintptr
	Offsets     []uintptr
	Signature   []byte

	EnableFn  func()
	DisableFn func()

	RightControls []fyne.CanvasObject

	addr         uintptr
	sigAddr      uintptr
	enabled      bool
	initialized  bool
	settingValue bool
	currentValue float64
	uiToggle     *M3Toggle
	uiSlider     *M3BounceSlider
	uiEntry      *widget.Entry
}

func (m *Int32PointerModule) CreateObjects() []fyne.CanvasObject {

	var v float64
	if m.initialized {
		v = m.currentValue
	} else {
		var err error
		v, err = m.initialRead()
		if err != nil {
			v = m.Default
			if m.Error != nil {
				m.Error(fmt.Errorf("initial read: %w", err))
			}
		}
		m.currentValue = v
		m.initialized = true
	}

	toggle := NewM3Toggle(m.enabled)
	m.uiToggle = toggle

	slider := NewM3BounceSlider(m.Min, m.Max, v)
	slider.Step = 1
	m.uiSlider = slider

	entry := widget.NewEntry()
	entry.SetText(fmt.Sprintf("%.0f", v))
	m.uiEntry = entry

	toggle.OnChange = func(checked bool) {
		m.enabled = checked
		if checked {
			if m.EnableFn != nil {
				m.EnableFn()
			} else {
				m.Enable()
			}
		} else {
			if m.DisableFn != nil {
				m.DisableFn()
			} else {
				m.Disable()
			}
		}
		if m.AfterChange != nil {
			m.AfterChange()
		}
	}

	inputRecursive := false
	sliderRecursive := false

	slider.OnChanged = func(f float64) {
		if sliderRecursive {
			return
		}
		inputRecursive = true
		entry.SetText(strconv.FormatFloat(f, 'f', 0, 64))
		inputRecursive = false
	}

	slider.OnChangeEnded = func(f float64) {
		if sliderRecursive {
			return
		}
		m.currentValue = f
		if m.enabled {
			m.write(f)
		}

		if m.AfterChange != nil && !m.settingValue {
			m.AfterChange()
		}
	}

	entry.OnChanged = func(s string) {
		if inputRecursive {
			return
		}

		filtered := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, s)
		if filtered != s {
			inputRecursive = true
			entry.SetText(filtered)
			inputRecursive = false
			s = filtered
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return
		}
		if f > m.Max {
			inputRecursive = true
			entry.SetText(strconv.FormatFloat(m.Max, 'f', 0, 64))
			inputRecursive = false
			f = m.Max
		} else if f < m.Min {
			inputRecursive = true
			entry.SetText(strconv.FormatFloat(m.Min, 'f', 0, 64))
			inputRecursive = false
			f = m.Min
		}
		m.currentValue = f
		if m.enabled {
			m.write(f)
		}
		sliderRecursive = true
		slider.SetValueAnimated(f)
		sliderRecursive = false
		if m.AfterChange != nil && !m.settingValue {
			m.AfterChange()
		}
	}

	entryWrapper := container.New(&minWidthLayout{width: 58}, entry)
	// sliderWrapper оборачивает слайдер для padding — используем напрямую в контейнере
	sliderWrapper := container.New(layout.NewMaxLayout(), slider)

	rightItems := append([]fyne.CanvasObject{entryWrapper}, m.RightControls...)
	rightItems = append(rightItems, toggle)
	rightGroup := container.NewHBox(rightItems...)

	row := container.New(layout.NewBorderLayout(nil, nil, nil, rightGroup),
		rightGroup,
		container.New(layout.NewCustomPaddedLayout(0, 0, 8, 8), sliderWrapper),
	)
	return []fyne.CanvasObject{row}
}

func (m *Int32PointerModule) Enable() {
	m.enabled = true
	if m.uiToggle != nil {
		m.uiToggle.SetChecked(true)
	}
	if err := m.nopSignature(); err != nil {
		if m.Error != nil {
			m.Error(fmt.Errorf("enable: nop signature: %w", err))
		}
		return
	}
	m.write(m.currentValue)
}

func (m *Int32PointerModule) Disable() {
	m.enabled = false
	if m.uiToggle != nil {
		m.uiToggle.SetChecked(false)
	}
	if err := m.restoreSignature(); err != nil {
		if m.Error != nil {
			m.Error(fmt.Errorf("disable: restore signature: %w", err))
		}
	}
}

func (m *Int32PointerModule) IsEnabled() bool {
	return m.enabled
}

func (m *Int32PointerModule) Value() float64 {
	return m.currentValue
}

func (m *Int32PointerModule) SetValue(val float64) {
	m.settingValue = true
	defer func() { m.settingValue = false }()

	m.currentValue = val
	m.initialized = true
	if m.uiSlider != nil {
		m.uiSlider.SetValueImmediate(val)
	}
	if m.uiEntry != nil {
		m.uiEntry.SetText(strconv.FormatFloat(val, 'f', 0, 64))
	}
	if m.enabled {
		m.write(val)
	}
}

func (m *Int32PointerModule) write(val float64) {
	addr, err := m.resolveAddress()
	if err != nil {
		if m.Error != nil {
			m.Error(fmt.Errorf("write %g: %w", val, err))
		}
		return
	}
	toWrite := m.SliderToMemory(val)
	if err = win.WriteMemory[int32](m.Process, addr, toWrite); err != nil {
		if m.Error != nil {
			m.Error(fmt.Errorf("write %g: write memory: %w", val, err))
		}
	}
}

func (m *Int32PointerModule) initialRead() (float64, error) {
	addr, err := m.resolveAddress()
	if err != nil {
		return 0, fmt.Errorf("resolve address: %w", err)
	}
	v, err := win.ReadMemory[int32](m.Process, addr)
	if err != nil {
		return 0, fmt.Errorf("read memory: %w", err)
	}
	return m.MemoryToSlider(v), nil
}

func (m *Int32PointerModule) resolveAddress() (uintptr, error) {
	if m.addr != 0 {
		return m.addr, nil
	}
	addr, err := win.ResolvePointerAddress(m.Process, m.Process.Module, m.BaseAddress, m.Offsets)
	if err != nil {
		return 0, err
	}
	m.addr = addr
	return addr, nil
}

func (m *Int32PointerModule) nopSignature() error {
	addr, err := m.findSignature()
	if err != nil {
		return err
	}
	nops := make([]byte, len(m.Signature))
	for i := range nops {
		nops[i] = 0x90
	}
	return win.Patch(m.Process, addr, nops)
}

func (m *Int32PointerModule) restoreSignature() error {
	addr, err := m.findSignature()
	if err != nil {
		return err
	}
	return win.Patch(m.Process, addr, m.Signature)
}

func (m *Int32PointerModule) findSignature() (uintptr, error) {
	if m.sigAddr != 0 {
		return m.sigAddr, nil
	}
	addr, err := win.ScanSignature(m.Process, m.Process.ModuleSize, m.Process.Module, m.Signature)
	if err != nil {
		return 0, err
	}
	m.sigAddr = addr
	return addr, nil
}

type minWidthLayout struct {
	width float32
}

func (l *minWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	h := float32(0)
	for _, o := range objects {
		if ms := o.MinSize(); ms.Height > h {
			h = ms.Height
		}
	}
	return fyne.NewSize(l.width, h)
}

func (l *minWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		h := o.MinSize().Height
		y := (size.Height - h) / 2
		if y < 0 {
			y = 0
		}
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(l.width, h))
	}
}
