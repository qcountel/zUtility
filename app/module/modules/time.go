package modules

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/qcountel/zUtility/app/module"
	"github.com/qcountel/zUtility/app/module/modules/modulesutil"
	win "github.com/qcountel/zUtility/pkg/winlegacy"
)

var (
	timeBaseAddress = uintptr(0x01921888)
	timeOffsets     = []uintptr{0x10, 0x58, 0x58, 0xD8, 0x0, 0x38, 0x194}
	timeSig         = []byte{0x41, 0x89, 0x89, 0x94, 0x01, 0x00, 0x00}
)

var _ module.Module = (*timeModule)(nil)

type Time struct {
	Process     *win.Process
	Error       func(error)
	AfterChange func()
}

func (conf Time) Create() module.Module {
	tm := &timeModule{
		Int32PointerModule: &modulesutil.Int32PointerModule{
			Process:     conf.Process,
			Error:       conf.Error,
			AfterChange: conf.AfterChange,
			Min:         0,
			Max:         15000,
			Default:     0,
			SliderToMemory: func(f float64) int32 {
				return int32(f)
			},
			MemoryToSlider: func(i int32) float64 {
				return float64(i)
			},
			BaseAddress: timeBaseAddress,
			Offsets:     timeOffsets,
			Signature:   timeSig,
		},
		proc:     conf.Process,
		errorFn:  conf.Error,
		stopChan: make(chan struct{}),
	}

	tm.Int32PointerModule.EnableFn = tm.Enable
	tm.Int32PointerModule.DisableFn = tm.Disable

	return tm
}

type timeModule struct {
	*modulesutil.Int32PointerModule
	proc    *win.Process
	errorFn func(error)

	stopChan chan struct{}
	freezing bool
	freezeMu sync.Mutex

	cachedTimeAddr uintptr
	cachedAddrMu   sync.Mutex

	originalTime int32
	hasOriginal  bool
	origMu       sync.Mutex
}

func (*timeModule) Name() string { return "TimeChanger" }
func (*timeModule) Description() string {
	return "Изменяет и замораживает игровое время на выбранном значении."
}

func (t *timeModule) CreateObjects() []fyne.CanvasObject {
	presets := []struct {
		label string
		value float64
	}{
		{label: "День", value: 1000},
		{label: "Закат", value: 12750},
		{label: "Ночь", value: 15000},
	}

	presetsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), nil)
	presetsBtn.Importance = widget.LowImportance
	presetsPanel := container.NewGridWithColumns(3)
	presetsPanel.Hide()
	buttons := make([]fyne.CanvasObject, 0, len(presets))
	for _, preset := range presets {
		preset := preset
		btn := widget.NewButton(preset.label, func() {
			t.Int32PointerModule.SetValue(preset.value)
			if t.AfterChange != nil {
				t.AfterChange()
			}
			presetsPanel.Hide()
			presetsPanel.Refresh()
		})
		buttons = append(buttons, btn)
	}
	presetsPanel.Objects = buttons
	presetsBtn.OnTapped = func() {
		if presetsPanel.Visible() {
			presetsPanel.Hide()
		} else {
			presetsPanel.Show()
		}
		presetsPanel.Refresh()
	}

	t.Int32PointerModule.RightControls = []fyne.CanvasObject{presetsBtn}
	base := t.Int32PointerModule.CreateObjects()

	if len(base) == 0 {
		return []fyne.CanvasObject{presetsBtn}
	}

	row := base[0]
	objects := []fyne.CanvasObject{container.NewVBox(row, presetsPanel)}
	if len(base) > 1 {
		objects = append(objects, base[1:]...)
	}
	return objects
}

func (t *timeModule) Enable() {
	t.origMu.Lock()
	if !t.hasOriginal {
		addr, err := t.getTimeAddr()
		if err == nil {
			orig, err := win.ReadMemory[int32](t.proc, addr)
			if err == nil {
				t.originalTime = orig
				t.hasOriginal = true
			}
		}
	}
	t.origMu.Unlock()

	t.Int32PointerModule.Enable()
	t.startFreezeLoop()
}

func (t *timeModule) Disable() {
	t.stopFreezeLoop()

	t.origMu.Lock()
	if t.hasOriginal {
		addr, err := t.getTimeAddr()
		if err == nil {
			_ = win.WriteMemory[int32](t.proc, addr, t.originalTime)
		}
		t.hasOriginal = false
	}
	t.origMu.Unlock()

	t.Int32PointerModule.Disable()
}

func (t *timeModule) startFreezeLoop() {
	t.freezeMu.Lock()
	defer t.freezeMu.Unlock()
	if t.freezing {
		return
	}
	t.freezing = true
	t.stopChan = make(chan struct{})

	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-t.stopChan:
				return
			case <-ticker.C:
				val := t.Int32PointerModule.Value()
				addr, err := t.getTimeAddr()
				if err != nil {
					t.invalidateCache()
					continue
				}
				if err := win.WriteMemory[int32](t.proc, addr, int32(val)); err != nil {
					t.invalidateCache()
				}
			}
		}
	}()
}

func (t *timeModule) stopFreezeLoop() {
	t.freezeMu.Lock()
	defer t.freezeMu.Unlock()
	if !t.freezing {
		return
	}
	t.freezing = false
	close(t.stopChan)
}

func (t *timeModule) getTimeAddr() (uintptr, error) {
	t.cachedAddrMu.Lock()
	defer t.cachedAddrMu.Unlock()
	if t.cachedTimeAddr != 0 {
		return t.cachedTimeAddr, nil
	}
	mInfo, err := t.proc.GetModuleInfo()
	if err != nil {
		return 0, err
	}
	mod := mInfo.Address
	addr, err := win.ResolvePointerAddress(t.proc, mod, timeBaseAddress, timeOffsets)
	if err != nil {
		return 0, err
	}
	t.cachedTimeAddr = addr
	return addr, nil
}

func (t *timeModule) invalidateCache() {
	t.cachedAddrMu.Lock()
	t.cachedTimeAddr = 0
	t.cachedAddrMu.Unlock()
}
