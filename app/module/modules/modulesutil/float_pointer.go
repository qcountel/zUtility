package modulesutil

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	win "github.com/qcountel/zUtility/pkg/winlegacy"
)

type FloatPointerModule struct {
	Process *win.Process
	Error   func(error)

	Min, Max, Default float64
	SliderToMemory    func(float64) float32
	MemoryToSlider    func(float32) float64

	BaseAddress uintptr
	Offsets     []uintptr

	addr         uintptr
	enabled      bool
	currentValue float64
	uiSlider     *M3BounceSlider
	uiEntry      *widget.Entry
}

func (m *FloatPointerModule) CreateObjects() []fyne.CanvasObject {
	v, err := m.initialRead()
	if err != nil {
		v = m.Default
		if m.Error != nil {
			m.Error(fmt.Errorf("initial read: %w", err))
		}
	} else {
		m.Default = v
	}
	m.currentValue = v

	slider := NewM3BounceSlider(m.Min, m.Max, v)
	slider.Step = 1
	m.uiSlider = slider

	entry := widget.NewEntry()
	entry.SetText(fmt.Sprintf("%.0f", v))
	m.uiEntry = entry

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
		m.write(f)
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
		m.write(f)
		sliderRecursive = true
		slider.SetValueAnimated(f)
		sliderRecursive = false
	}

	entryWrapper := container.New(&minWidthLayout{width: 46}, entry)
	row := container.New(layout.NewBorderLayout(nil, nil, nil, entryWrapper),
		entryWrapper,
		container.New(layout.NewCustomPaddedLayout(0, 0, 0, 8), slider),
	)
	return []fyne.CanvasObject{row}
}

func (m *FloatPointerModule) Enable() {
	m.enabled = true
	m.write(m.currentValue)
}

func (m *FloatPointerModule) Disable() {
	m.enabled = false
	m.write(m.Default)
}

func (m *FloatPointerModule) IsEnabled() bool {
	return m.enabled
}

func (m *FloatPointerModule) Value() float64 {
	return m.currentValue
}

func (m *FloatPointerModule) SetValue(val float64) {
	m.currentValue = val
	if m.uiSlider != nil {
		m.uiSlider.SetValueAnimated(val)
	}
	if m.uiEntry != nil {
		m.uiEntry.SetText(strconv.FormatFloat(val, 'f', 0, 64))
	}
	if m.enabled {
		m.write(val)
	}
}

func (m *FloatPointerModule) write(val float64) {
	addr, err := m.resolveAddress()
	if err != nil {
		if m.Error != nil {
			m.Error(fmt.Errorf("write %g: %w", val, err))
		}
		return
	}
	toWrite := m.SliderToMemory(val)
	if err = win.WriteMemory[float32](m.Process, addr, toWrite); err != nil {
		if m.Error != nil {
			m.Error(fmt.Errorf("write %g: write memory: %w", val, err))
		}
	}
}

func (m *FloatPointerModule) initialRead() (float64, error) {
	addr, err := m.resolveAddress()
	if err != nil {
		return 0, fmt.Errorf("resolve address: %w", err)
	}
	v, err := win.ReadMemory[float32](m.Process, addr)
	if err != nil {
		return 0, fmt.Errorf("read memory: %w", err)
	}

	return m.MemoryToSlider(v), nil
}

func (m *FloatPointerModule) resolveAddress() (uintptr, error) {
	if m.Process == nil {
		return 0, fmt.Errorf("process is nil")
	}
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
