package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/qcountel/zUtility/app/module"
	"github.com/qcountel/zUtility/app/module/modules/modulesutil"
	"github.com/qcountel/zUtility/pkg/asm"
	"github.com/qcountel/zUtility/pkg/win/mem/memutil"
	win "github.com/qcountel/zUtility/pkg/winlegacy"
)

const (
	offsetGetSkyColor1 = 0x00b96810
	offsetGetSkyColor2 = 0x00b96f90
	offsetGetFogColor  = 0x00b8e5b0
)

type mceColor struct {
	R, G, B, A float32
}

var _ module.Module = (*customFog)(nil)

type CustomFog struct {
	Process     *win.Process
	Error       func(error)
	AfterChange func()
}

func (conf CustomFog) Create() module.Module {
	m := &customFog{
		process:     conf.Process,
		errFn:       conf.Error,
		afterChange: conf.AfterChange,
		r:           1.0,
		g:           1.0,
		b:           1.0,
	}
	return m
}

type customFog struct {
	mu          sync.Mutex
	process     *win.Process
	errFn       func(error)
	afterChange func()

	enabled bool
	r, g, b float64

	cancel     context.CancelFunc
	skyDetour1 *memutil.ProxiedDetour[mceColor]
	skyDetour2 *memutil.ProxiedDetour[mceColor]
	fogDetour  *memutil.ProxiedDetour[mceColor]

	uiToggle *modulesutil.M3Toggle
	rSlider, gSlider, bSlider *modulesutil.M3BounceSlider
	rEntry, gEntry, bEntry    *widget.Entry
}

func (*customFog) Name() string {
	return "CustomFog"
}

func (*customFog) Description() string {
	return "Изменяет цвет тумана. Настройте RGB ползунками."
}

func (m *customFog) IsEnabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

func (m *customFog) Colors() (float64, float64, float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.r, m.g, m.b
}

func (m *customFog) SetColors(r, g, b float64) {
	m.mu.Lock()
	m.r, m.g, m.b = r, g, b
	m.mu.Unlock()

	if m.rSlider != nil {
		m.rSlider.SetValueImmediate(r)
	}
	if m.gSlider != nil {
		m.gSlider.SetValueImmediate(g)
	}
	if m.bSlider != nil {
		m.bSlider.SetValueImmediate(b)
	}

	if m.rEntry != nil {
		m.rEntry.SetText(fmt.Sprintf("%.2f", r))
	}
	if m.gEntry != nil {
		m.gEntry.SetText(fmt.Sprintf("%.2f", g))
	}
	if m.bEntry != nil {
		m.bEntry.SetText(fmt.Sprintf("%.2f", b))
	}
}

func (m *customFog) Enable() {
	m.mu.Lock()
	if m.enabled {
		m.mu.Unlock()
		return
	}
	m.enabled = true
	m.mu.Unlock()

	if m.uiToggle != nil {
		m.uiToggle.SetChecked(true)
	}
	m.startLoop()
}

func (m *customFog) Disable() {
	m.mu.Lock()
	if !m.enabled {
		m.mu.Unlock()
		return
	}
	m.enabled = false
	m.mu.Unlock()

	if m.uiToggle != nil {
		m.uiToggle.SetChecked(false)
	}
	m.stopLoop()
}

func (m *customFog) startLoop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	go m.loop(ctx)
}

func (m *customFog) stopLoop() {
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}

	m.mu.Lock()
	if m.skyDetour1 != nil {
		m.skyDetour1.Disable()
		m.skyDetour1 = nil
	}
	if m.skyDetour2 != nil {
		m.skyDetour2.Disable()
		m.skyDetour2 = nil
	}
	if m.fogDetour != nil {
		m.fogDetour.Disable()
		m.fogDetour = nil
	}
	m.mu.Unlock()
}

func (m *customFog) loop(ctx context.Context) {
	var (
		lastR, lastG, lastB float64 = -1, -1, -1
		initialized         bool
	)

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		if !initialized {
			err := m.enableDetours()
			if err != nil {
				if m.errFn != nil {
					m.errFn(fmt.Errorf("custom fog enable detours: %w", err))
				}
				m.mu.Lock()
				m.enabled = false
				m.mu.Unlock()
				if m.uiToggle != nil {
					fyne.Do(func() {
						prev := m.uiToggle.OnChange
						m.uiToggle.OnChange = nil
						m.uiToggle.SetChecked(false)
						m.uiToggle.OnChange = prev
					})
				}
				m.stopLoop()
				return
			}
			initialized = true
		}

		m.mu.Lock()
		r, g, b := m.r, m.g, m.b
		m.mu.Unlock()

		if r != lastR || g != lastG || b != lastB {
			m.writeColor(float32(r), float32(g), float32(b))
			lastR, lastG, lastB = r, g, b
		}
	}
}

func (m *customFog) enableDetours() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mod, err := m.process.GetModuleInfo()
	if err != nil {
		return fmt.Errorf("get module info: %w", err)
	}

	skyAddr1 := mod.Address + offsetGetSkyColor1
	m.skyDetour1 = memutil.NewProxiedDetour[mceColor](m.process, skyAddr1, 12)
	err = m.skyDetour1.Enable(customFogUserCode, mceColor{R: 1.0, G: 1.0, B: 1.0, A: 1.0})
	if err != nil {
		return fmt.Errorf("enable sky detour 1: %w", err)
	}

	skyAddr2 := mod.Address + offsetGetSkyColor2
	m.skyDetour2 = memutil.NewProxiedDetour[mceColor](m.process, skyAddr2, 12)
	err = m.skyDetour2.Enable(customFogUserCode, mceColor{R: 1.0, G: 1.0, B: 1.0, A: 1.0})
	if err != nil {
		m.skyDetour1.Disable()
		m.skyDetour1 = nil
		return fmt.Errorf("enable sky detour 2: %w", err)
	}

	fogAddr := mod.Address + offsetGetFogColor
	m.fogDetour = memutil.NewProxiedDetour[mceColor](m.process, fogAddr, 10)
	err = m.fogDetour.Enable(customFogUserCode, mceColor{R: 1.0, G: 1.0, B: 1.0, A: 1.0})
	if err != nil {
		m.skyDetour1.Disable()
		m.skyDetour1 = nil
		m.skyDetour2.Disable()
		m.skyDetour2 = nil
		m.fogDetour = nil
		return fmt.Errorf("enable fog detour: %w", err)
	}

	return nil
}

func (m *customFog) writeColor(r, g, b float32) {
	m.mu.Lock()
	skyDet1 := m.skyDetour1
	skyDet2 := m.skyDetour2
	fogDet := m.fogDetour
	m.mu.Unlock()

	col := mceColor{R: r, G: g, B: b, A: 1.0}
	if skyDet1 != nil {
		_ = skyDet1.WriteValue(col)
	}
	if skyDet2 != nil {
		_ = skyDet2.WriteValue(col)
	}
	if fogDet != nil {
		_ = fogDet.WriteValue(col)
	}
}

func customFogUserCode(valAddr uintptr) []byte {
	return asm.Build().
		Mov64(asm.Rax, valAddr).
		Raw(0xF3, 0x0F, 0x10, 0x00).
		Raw(0xF3, 0x0F, 0x11, 0x02).
		Raw(0xF3, 0x0F, 0x10, 0x40, 0x04).
		Raw(0xF3, 0x0F, 0x11, 0x42, 0x04).
		Raw(0xF3, 0x0F, 0x10, 0x40, 0x08).
		Raw(0xF3, 0x0F, 0x11, 0x42, 0x08).
		Raw(0xF3, 0x0F, 0x10, 0x40, 0x0C).
		Raw(0xF3, 0x0F, 0x11, 0x42, 0x0C).
		Raw(0x48, 0x89, 0xD0).
		Ret().
		Result()
}

func (m *customFog) CreateObjects() []fyne.CanvasObject {
	m3Toggle := modulesutil.NewM3Toggle(m.enabled)
	m3Toggle.OnChange = func(b bool) {
		if b {
			m.Enable()
		} else {
			m.Disable()
		}
		if m.afterChange != nil {
			m.afterChange()
		}
	}
	m.uiToggle = m3Toggle

	createSliderControl := func(ptr *float64, sPtr **modulesutil.M3BounceSlider, ePtr **widget.Entry) fyne.CanvasObject {
		m.mu.Lock()
		v := *ptr
		m.mu.Unlock()
		slider := modulesutil.NewM3BounceSlider(0.0, 1.0, v)
		slider.Step = 0.01
		*sPtr = slider

		entry := widget.NewEntry()
		entry.SetText(fmt.Sprintf("%.2f", v))
		*ePtr = entry

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
			*ptr = f
			m.mu.Unlock()
			if m.afterChange != nil {
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
			if f > 1.0 {
				inputRecursive = true
				entry.SetText("1.00")
				inputRecursive = false
				f = 1.0
			} else if f < 0.0 {
				inputRecursive = true
				entry.SetText("0.00")
				inputRecursive = false
				f = 0.0
			}
			m.mu.Lock()
			*ptr = f
			m.mu.Unlock()
			if m.afterChange != nil {
				m.afterChange()
			}

			sliderRecursive = true
			slider.SetValueAnimated(f)
			sliderRecursive = false
		}

		minWidth := float32(46.0)
		entryWrapper := container.New(&minWidthLayoutFog{width: minWidth}, entry)
		sliderWrapper := container.New(layout.NewMaxLayout(), slider)

		row := container.New(layout.NewBorderLayout(nil, nil, nil, entryWrapper),
			entryWrapper,
			container.New(layout.NewCustomPaddedLayout(0, 0, 0, 8), sliderWrapper),
		)
		return row
	}

	rControl := createSliderControl(&m.r, &m.rSlider, &m.rEntry)
	gControl := createSliderControl(&m.g, &m.gSlider, &m.gEntry)
	bControl := createSliderControl(&m.b, &m.bSlider, &m.bEntry)

	form := container.New(layout.NewFormLayout(),
		widget.NewLabel("Red"), rControl,
		widget.NewLabel("Green"), gControl,
		widget.NewLabel("Blue"), bControl,
	)

	slidersPanel := container.New(layout.NewCustomPaddedLayout(8, 0, 0, 0), form)
	slidersPanel.Hide()

	settingsBtn := widget.NewButton("Цвета RGB", func() {
		if slidersPanel.Visible() {
			slidersPanel.Hide()
		} else {
			slidersPanel.Show()
		}
		slidersPanel.Refresh()
	})
	settingsBtn.Importance = widget.LowImportance

	topRow := container.NewHBox(settingsBtn, m3Toggle)

	return []fyne.CanvasObject{topRow, slidersPanel}
}

type minWidthLayoutFog struct {
	width float32
}

func (l *minWidthLayoutFog) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(fyne.NewSize(l.width, size.Height))
		o.Move(fyne.NewPos(size.Width-l.width, 0))
	}
}

func (l *minWidthLayoutFog) MinSize(objects []fyne.CanvasObject) fyne.Size {
	h := float32(0)
	for _, o := range objects {
		if ms := o.MinSize(); ms.Height > h {
			h = ms.Height
		}
	}
	return fyne.NewSize(l.width, h)
}
