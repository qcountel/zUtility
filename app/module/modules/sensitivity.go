package modules

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/qcountel/zUtility/app/module"
	"github.com/qcountel/zUtility/app/module/modules/modulesutil"
	"github.com/qcountel/zUtility/pkg/asm"
	win "github.com/qcountel/zUtility/pkg/winlegacy"
	"github.com/qcountel/zUtility/pkg/win/mem"
	"github.com/qcountel/zUtility/pkg/win/mem/memutil"
)

var sensitivitySig = mem.MustParseSignature("F3 0F 10 40 14 48 83 C4 10 5B C3")

var _ module.Module = (*sensitivity)(nil)

type Sensitivity struct {
	Process     *win.Process
	Error       func(error)
	AfterChange func()
}

func (conf Sensitivity) Create() module.Module {
	m := &sensitivity{
		process:     conf.Process,
		errFn:       conf.Error,
		afterChange: conf.AfterChange,
		min:         0.1,
		max:         3.0,
		def:         1.0,
		val:         1.0,
	}
	go m.Enable()
	return m
}

type sensitivity struct {
	mu           sync.Mutex
	process      *win.Process
	errFn        func(error)
	afterChange  func()
	min, max, def float64

	val          float64
	enabled      bool
	settingValue bool
	det          *memutil.ProxiedDetour[float32]
	uiSlider     *modulesutil.M3BounceSlider
	uiEntry      *widget.Entry
	detOnce      sync.Once
	detErr       error
}

func (*sensitivity) Name() string {
	return "Sensitivity"
}

func (*sensitivity) Description() string {
	return "Увеличивает или уменьшает общую чувствительность мыши и геймпада."
}

func (m *sensitivity) IsEnabled() bool {
	return true
}

func (m *sensitivity) Enable() {
	m.mu.Lock()
	if m.enabled {
		m.mu.Unlock()
		return
	}
	m.enabled = true
	m.mu.Unlock()

	go func() {
		det, err := m.lazyDetour()
		if err != nil {
			if m.errFn != nil {
				m.errFn(fmt.Errorf("sensitivity: enable: %w", err))
			}
			return
		}
		m.mu.Lock()
		val := m.val
		m.mu.Unlock()
		_ = det.WriteValue(float32(val))
	}()
}

func (m *sensitivity) Disable() {
	m.mu.Lock()
	det := m.det
	m.mu.Unlock()
	if det != nil {
		_ = det.WriteValue(float32(m.def))
	}
}

func (m *sensitivity) Value() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.val
}

func (m *sensitivity) SetValue(val float64) {
	m.settingValue = true
	defer func() { m.settingValue = false }()

	m.mu.Lock()
	m.val = val
	enabled := m.enabled
	m.mu.Unlock()

	if m.uiSlider != nil {
		m.uiSlider.SetValueAnimated(val)
	}
	if m.uiEntry != nil {
		m.uiEntry.SetText(strconv.FormatFloat(val, 'f', 2, 64))
	}
	if enabled {
		m.write(val)
	}
}

func (m *sensitivity) CreateObjects() []fyne.CanvasObject {
	m.mu.Lock()
	v := m.val
	m.mu.Unlock()

	slider := modulesutil.NewM3BounceSlider(m.min, m.max, v)
	slider.Step = 0.05
	m.uiSlider = slider

	entry := widget.NewEntry()
	entry.SetText(fmt.Sprintf("%.2f", v))
	m.uiEntry = entry

	inputRecursive := false
	sliderRecursive := false

	slider.OnChanged = func(f float64) {
		if sliderRecursive {
			return
		}
		inputRecursive = true
		entry.SetText(strconv.FormatFloat(f, 'f', 2, 64))
		inputRecursive = false
	}
	slider.OnChangeEnded = func(f float64) {
		if sliderRecursive {
			return
		}
		m.mu.Lock()
		m.val = f
		enabled := m.enabled
		m.mu.Unlock()
		if enabled {
			m.write(f)
		}
		if m.afterChange != nil && !m.settingValue {
			m.afterChange()
		}
	}

	entry.OnChanged = func(s string) {
		if inputRecursive {
			return
		}
		filtered := strings.Map(func(r rune) rune {
			if (r >= '0' && r <= '9') || r == '.' || r == ',' {
				if r == ',' {
					return '.'
				}
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
		if f > m.max {
			inputRecursive = true
			entry.SetText(strconv.FormatFloat(m.max, 'f', 2, 64))
			inputRecursive = false
			f = m.max
		} else if f < m.min {
			inputRecursive = true
			entry.SetText(strconv.FormatFloat(m.min, 'f', 2, 64))
			inputRecursive = false
			f = m.min
		}
		m.mu.Lock()
		m.val = f
		enabled := m.enabled
		m.mu.Unlock()
		if enabled {
			m.write(f)
		}
		if m.afterChange != nil && !m.settingValue {
			m.afterChange()
		}
		sliderRecursive = true
		slider.SetValueAnimated(f)
		sliderRecursive = false
	}

	minWidth := 46
	entryWrapper := container.New(&minWidthLayout5{width: float32(minWidth)}, entry)
	row := container.New(layout.NewBorderLayout(nil, nil, nil, entryWrapper),
		entryWrapper,
		container.New(layout.NewCustomPaddedLayout(0, 0, 0, 8), slider),
	)
	return []fyne.CanvasObject{row}
}

type minWidthLayout5 struct {
	width float32
}

func (l *minWidthLayout5) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(fyne.NewSize(l.width, size.Height))
		o.Move(fyne.NewPos(size.Width-l.width, 0))
	}
}

func (l *minWidthLayout5) MinSize(objects []fyne.CanvasObject) fyne.Size {
	h := float32(0)
	for _, o := range objects {
		h = maxFloat32_5(h, o.MinSize().Height)
	}
	return fyne.NewSize(l.width, h)
}

func maxFloat32_5(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func (m *sensitivity) write(val float64) {
	m.mu.Lock()
	det := m.det
	m.mu.Unlock()

	if det == nil {
		return
	}
	if err := det.WriteValue(float32(val)); err != nil {
		if m.errFn != nil {
			m.errFn(fmt.Errorf("sensitivity: write %g: %w", val, err))
		}
	}
}

func (m *sensitivity) lazyDetour() (*memutil.ProxiedDetour[float32], error) {
	m.detOnce.Do(func() {
		modInfo, err := m.process.GetModuleInfo()
		if err != nil {
			m.detErr = fmt.Errorf("get module info: %w", err)
			return
		}

		addr := modInfo.Address + 0x004956a6

		det := memutil.NewProxiedDetour[float32](m.process, addr, 11)
		if err = det.Enable(sensitivityUserCode, float32(m.def)); err != nil {
			m.detErr = fmt.Errorf("enable detour: %w", err)
			return
		}

		m.det = det
	})

	return m.det, m.detErr
}

func sensitivityUserCode(valAddr uintptr) []byte {
	return asm.Build().
		RawStr("F3 0F 10 40 14").
		Mov64(asm.Rax, valAddr).
		MulssXmm(0, asm.Rax).
		RawStr("48 83 C4 10").
		Pop(asm.Rbx).
		Ret().
		Result()
}
