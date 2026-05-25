package modules

import (
	"context"
	"fmt"
	"image/color"
	"sync"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/sys/windows"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
)

var (
	_hotbarUser32   = windows.NewLazySystemDLL("user32.dll")
	_procMouseEvent = _hotbarUser32.NewProc("mouse_event")

	_winmmDLL        = windows.NewLazySystemDLL("winmm.dll")
	_procJoyGetPosEx = _winmmDLL.NewProc("joyGetPosEx")
)

const (
	mouseeventfWheel = 0x0800
	wheelDelta       = 120
)

type joyInfoEx struct {
	dwSize         uint32
	dwFlags        uint32
	dwXpos         uint32
	dwYpos         uint32
	dwZpos         uint32
	dwRpos         uint32
	dwUpos         uint32
	dwVpos         uint32
	dwButtons      uint32
	dwButtonNumber uint32
	dwPOV          uint32
	dwReserved1    uint32
	dwReserved2    uint32
}

var _ module.Module = (*InstantHotbarModule)(nil)

type InstantHotbar struct {
	Process     *win.Process
	Error       func(error)
	AfterChange func()
	Window      fyne.Window
}

func (conf InstantHotbar) Create() module.Module {
	return &InstantHotbarModule{
		process:     conf.Process,
		errFn:       conf.Error,
		afterChange: conf.AfterChange,
		window:      conf.Window,
		bindVKLeft:  5, // Default: Button 5 (LB)
		bindVKRight: 6, // Default: Button 6 (RB)
	}
}

type InstantHotbarModule struct {
	mu          sync.Mutex
	process     *win.Process
	errFn       func(error)
	afterChange func()
	window      fyne.Window

	enabled     bool
	bindVKLeft  uint32
	bindVKRight uint32
	hideWarning bool
	cancel      context.CancelFunc

	toggle         *modulesutil.M3Toggle
	bindLabelLeft  *widget.Label
	bindLabelRight *widget.Label
}

func (*InstantHotbarModule) Name() string { return "InstantHotbar" }
func (*InstantHotbarModule) Description() string {
	return "Мгновенное переключение хотбара по первому клику геймпада (WinMM эмуляция скролла)."
}

func (m *InstantHotbarModule) IsEnabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

func (m *InstantHotbarModule) BindVK() uint32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bindVKLeft
}

func (m *InstantHotbarModule) SetBindVK(vk uint32) {
	m.mu.Lock()
	m.bindVKLeft = vk
	m.mu.Unlock()
	if m.bindLabelLeft != nil {
		m.bindLabelLeft.SetText(m.vkName(vk))
	}
}

func (m *InstantHotbarModule) BindVKRight() uint32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bindVKRight
}

func (m *InstantHotbarModule) SetBindVKRight(vk uint32) {
	m.mu.Lock()
	m.bindVKRight = vk
	m.mu.Unlock()
	if m.bindLabelRight != nil {
		m.bindLabelRight.SetText(m.vkName(vk))
	}
}

func (m *InstantHotbarModule) ModeInt() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.hideWarning {
		return 1
	}
	return 0
}

func (m *InstantHotbarModule) SetModeInt(mode int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hideWarning = (mode == 1)
}

func (m *InstantHotbarModule) Enable() {
	m.mu.Lock()
	m.enabled = true
	m.mu.Unlock()
	m.startLoop()
	if m.toggle != nil {
		prev := m.toggle.OnChange
		m.toggle.OnChange = nil
		m.toggle.SetChecked(true)
		m.toggle.OnChange = prev
	}
}

func (m *InstantHotbarModule) Disable() {
	m.mu.Lock()
	m.enabled = false
	m.mu.Unlock()
	m.stopLoop()
	if m.toggle != nil {
		prev := m.toggle.OnChange
		m.toggle.OnChange = nil
		m.toggle.SetChecked(false)
		m.toggle.OnChange = prev
	}
}

func (m *InstantHotbarModule) CreateObjects() []fyne.CanvasObject {
	toggle := modulesutil.NewM3Toggle(m.enabled)
	toggle.OnChange = func(b bool) {
		m.mu.Lock()
		m.enabled = b
		hide := m.hideWarning
		m.mu.Unlock()

		if b {
			m.startLoop()
			if !hide && m.window != nil {
				m.showWarningDialog()
			}
		} else {
			m.stopLoop()
		}
		if m.afterChange != nil {
			m.afterChange()
		}
	}
	m.toggle = toggle

	m.mu.Lock()
	initLeftVK := m.bindVKLeft
	initRightVK := m.bindVKRight
	m.mu.Unlock()

	// Влево (Left bind) UI
	labelLeft := widget.NewLabel("Влево:")
	labelLeft.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	keyLabelLeft := widget.NewLabel(m.vkName(initLeftVK))
	m.bindLabelLeft = keyLabelLeft

	clearBtnLeft := widget.NewButtonWithIcon("", theme.ContentClearIcon(), nil)
	clearBtnLeft.Importance = widget.LowImportance

	bindBtnLeft := widget.NewButton("Бинд", nil)
	bindBtnLeft.Importance = widget.LowImportance

	// Вправо (Right bind) UI
	labelRight := widget.NewLabel("Вправо:")
	labelRight.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	keyLabelRight := widget.NewLabel(m.vkName(initRightVK))
	m.bindLabelRight = keyLabelRight

	clearBtnRight := widget.NewButtonWithIcon("", theme.ContentClearIcon(), nil)
	clearBtnRight.Importance = widget.LowImportance

	bindBtnRight := widget.NewButton("Бинд", nil)
	bindBtnRight.Importance = widget.LowImportance

	var (
		captureCancel   context.CancelFunc
		captureCancelMu sync.Mutex
	)

	cancelCapture := func() {
		captureCancelMu.Lock()
		cc := captureCancel
		captureCancel = nil
		captureCancelMu.Unlock()
		if cc != nil {
			cc()
		}
	}

	clearBtnLeft.OnTapped = func() {
		cancelCapture()
		m.SetBindVK(0)
		bindBtnLeft.SetText("Бинд")
		if m.afterChange != nil {
			m.afterChange()
		}
	}

	clearBtnRight.OnTapped = func() {
		cancelCapture()
		m.SetBindVKRight(0)
		bindBtnRight.SetText("Бинд")
		if m.afterChange != nil {
			m.afterChange()
		}
	}

	bindBtnLeft.OnTapped = func() {
		cancelCapture()
		bindBtnLeft.SetText("Нажмите...")
		keyLabelLeft.SetText("...")

		captureCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		captureCancelMu.Lock()
		captureCancel = cancel
		captureCancelMu.Unlock()

		go func() {
			defer func() {
				cancel()
				captureCancelMu.Lock()
				captureCancel = nil
				captureCancelMu.Unlock()
			}()

			vk, ok := captureNextJoyKey(captureCtx)
			fyne.Do(func() {
				if ok && vk != 0 {
					m.SetBindVK(vk)
					if m.afterChange != nil {
						m.afterChange()
					}
				} else {
					m.mu.Lock()
					cur := m.bindVKLeft
					m.mu.Unlock()
					keyLabelLeft.SetText(m.vkName(cur))
				}
				bindBtnLeft.SetText("Бинд")
			})
		}()
	}

	bindBtnRight.OnTapped = func() {
		cancelCapture()
		bindBtnRight.SetText("Нажмите...")
		keyLabelRight.SetText("...")

		captureCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		captureCancelMu.Lock()
		captureCancel = cancel
		captureCancelMu.Unlock()

		go func() {
			defer func() {
				cancel()
				captureCancelMu.Lock()
				captureCancel = nil
				captureCancelMu.Unlock()
			}()

			vk, ok := captureNextJoyKey(captureCtx)
			fyne.Do(func() {
				if ok && vk != 0 {
					m.SetBindVKRight(vk)
					if m.afterChange != nil {
						m.afterChange()
					}
				} else {
					m.mu.Lock()
					cur := m.bindVKRight
					m.mu.Unlock()
					keyLabelRight.SetText(m.vkName(cur))
				}
				bindBtnRight.SetText("Бинд")
			})
		}()
	}

	// Perfect alignment using BorderLayout:
	// "Влево:" at the left edge of the subcard, controls at the right edge
	rowLeft := container.NewBorder(
		nil, nil,
		labelLeft,
		container.NewHBox(container.NewCenter(keyLabelLeft), clearBtnLeft, bindBtnLeft),
	)

	// "Вправо:" at the left edge of the subcard, controls at the right edge
	rowRight := container.NewBorder(
		nil, nil,
		labelRight,
		container.NewHBox(container.NewCenter(keyLabelRight), clearBtnRight, bindBtnRight),
	)
	
	presetsPanel := container.NewVBox(
		container.New(layout.NewCustomPaddedLayout(4, 4, 16, 16), rowLeft),
		container.New(layout.NewCustomPaddedLayout(4, 4, 16, 16), rowRight),
	)

	// Subcard container with 1px border matching the Brutalist theme
	panelBg := canvas.NewRectangle(color.NRGBA{R: 0x0A, G: 0x0A, B: 0x0A, A: 0xFF})
	panelBg.StrokeWidth = 1
	panelBg.StrokeColor = color.NRGBA{R: 0x1A, G: 0x1A, B: 0x1A, A: 0xFF}

	presetsPanelContainer := container.NewStack(
		panelBg,
		container.New(layout.NewCustomPaddedLayout(8, 8, 12, 12), presetsPanel),
	)
	presetsPanelContainer.Hide()

	// Gear Settings Button to toggle the panel
	presetsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), nil)
	presetsBtn.Importance = widget.LowImportance

	// Main Row: We wrap it in a BorderLayout to push the toggle and gear to the far right!
	mainRowControls := container.NewHBox(presetsBtn, toggle)
	mainRow := container.NewBorder(nil, nil, nil, mainRowControls)

	presetsBtn.OnTapped = func() {
		if presetsPanelContainer.Visible() {
			presetsPanelContainer.Hide()
		} else {
			presetsPanelContainer.Show()
		}
		presetsPanelContainer.Refresh()
	}

	// Combine main row and the hidden settings subcard
	col := container.NewVBox(
		mainRow,
		container.New(layout.NewCustomPaddedLayout(8, 0, 0, 0), presetsPanelContainer),
	)

	return []fyne.CanvasObject{col}
}

func (m *InstantHotbarModule) showWarningDialog() {
	// 1. Header Banner filled with solid Accent Color
	headerBg := canvas.NewRectangle(theme.PrimaryColor())
	headerBg.SetMinSize(fyne.NewSize(380, 36))
	
	headerText := canvas.NewText("ВНИМАНИЕ / WARNING", color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF})
	headerText.TextSize = 13
	headerText.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	
	header := container.NewStack(
		headerBg,
		container.New(layout.NewCustomPaddedLayout(10, 10, 14, 14), headerText),
	)

	// 2. Body Message
	bodyText1 := widget.NewLabel("Чтобы избежать двойного переключения слотов, пожалуйста, зайдите в настройки Minecraft -> Контроллер и снимите (очистите) бинды с кнопок переключения слотов хотбара.")
	bodyText1.Wrapping = fyne.TextWrapWord
	
	bodyText2 := widget.NewLabel("Модуль эмулирует мгновенную прокрутку колесика мыши, поэтому оригинальные кнопки геймпада в игре должны быть свободны от смены слотов.")
	bodyText2.Wrapping = fyne.TextWrapWord
	bodyText2.TextStyle = fyne.TextStyle{Italic: true}

	// 3. Checkbox
	var check *widget.Check
	check = widget.NewCheck("Больше не показывать это предупреждение", func(checked bool) {
		m.mu.Lock()
		m.hideWarning = checked
		m.mu.Unlock()
		if m.afterChange != nil {
			m.afterChange()
		}
	})

	var pop *widget.PopUp

	// 4. Chunky Brutalist OK Button
	okBtn := widget.NewButton("ПОНЯТНО", func() {
		if pop != nil {
			pop.Hide()
		}
	})
	okBtn.Importance = widget.HighImportance

	// 5. Card container and thick 2px border matching the Brutalist theme
	cardBg := canvas.NewRectangle(color.NRGBA{R: 0x08, G: 0x08, B: 0x08, A: 0xFF})
	
	cardBorder := canvas.NewRectangle(color.Transparent)
	cardBorder.StrokeWidth = 2
	cardBorder.StrokeColor = theme.PrimaryColor()

	cardContent := container.NewVBox(
		header,
		container.New(layout.NewCustomPaddedLayout(16, 16, 16, 16), container.NewVBox(
			bodyText1,
			container.New(layout.NewCustomPaddedLayout(8, 0, 0, 0), bodyText2),
			container.New(layout.NewCustomPaddedLayout(12, 12, 0, 0), check),
			container.New(layout.NewCustomPaddedLayout(16, 0, 0, 0), okBtn),
		)),
	)

	card := container.NewStack(
		cardBg,
		cardContent,
		cardBorder,
	)

	// Create and display PopUp Modal
	pop = widget.NewModalPopUp(card, m.window.Canvas())
	pop.Show()
}

func captureNextJoyKey(ctx context.Context) (uint32, bool) {
	time.Sleep(100 * time.Millisecond)
	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()

	var joyID uintptr = 0
	for id := uintptr(0); id < 16; id++ {
		var testInfo joyInfoEx
		testInfo.dwSize = uint32(unsafe.Sizeof(testInfo))
		testInfo.dwFlags = 0x000000FF
		r, _, _ := _procJoyGetPosEx.Call(id, uintptr(unsafe.Pointer(&testInfo)))
		if r == 0 {
			joyID = id
			break
		}
	}

	var initInfo joyInfoEx
	initInfo.dwSize = uint32(unsafe.Sizeof(initInfo))
	initInfo.dwFlags = 0x000000FF
	_procJoyGetPosEx.Call(joyID, uintptr(unsafe.Pointer(&initInfo)))

	for {
		select {
		case <-ctx.Done():
			return 0, false
		case <-ticker.C:
		}

		var info joyInfoEx
		info.dwSize = uint32(unsafe.Sizeof(info))
		info.dwFlags = 0x000000FF
		r, _, _ := _procJoyGetPosEx.Call(joyID, uintptr(unsafe.Pointer(&info)))
		if r != 0 {
			for id := uintptr(0); id < 16; id++ {
				r2, _, _ := _procJoyGetPosEx.Call(id, uintptr(unsafe.Pointer(&info)))
				if r2 == 0 {
					joyID = id
					break
				}
			}
			continue
		}

		for btn := uint32(1); btn <= 16; btn++ {
			mask := uint32(1) << (btn - 1)
			if (info.dwButtons & mask) != 0 && (initInfo.dwButtons & mask) == 0 {
				return btn, true
			}
		}

		if info.dwPOV != 65535 && initInfo.dwPOV == 65535 {
			return info.dwPOV, true
		}
	}
}

func (m *InstantHotbarModule) startLoop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	go m.loop(ctx)
}

func (m *InstantHotbarModule) stopLoop() {
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *InstantHotbarModule) loop(ctx context.Context) {
	ticker := time.NewTicker(8 * time.Millisecond)
	defer ticker.Stop()

	var (
		leftWasDown  bool
		rightWasDown bool

		leftLastTrigger  time.Time
		rightLastTrigger time.Time
	)

	var info joyInfoEx
	info.dwSize = uint32(unsafe.Sizeof(info))
	info.dwFlags = 0x000000FF

	var joyID uintptr = 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		m.mu.Lock()
		leftVK := m.bindVKLeft
		rightVK := m.bindVKRight
		m.mu.Unlock()

		r, _, _ := _procJoyGetPosEx.Call(joyID, uintptr(unsafe.Pointer(&info)))
		if r != 0 {
			found := false
			for id := uintptr(0); id < 16; id++ {
				r2, _, _ := _procJoyGetPosEx.Call(id, uintptr(unsafe.Pointer(&info)))
				if r2 == 0 {
					joyID = id
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 1. Process Left switch bind
		if leftVK != 0 {
			isPressed := isJoyKeyPressed(leftVK, &info)

			if isPressed && !leftWasDown {
				if time.Since(leftLastTrigger) > 180*time.Millisecond {
					leftLastTrigger = time.Now()
					leftWasDown = true
					go m.triggerScroll(wheelDelta) // Scroll Up (Prev Slot)
				}
			} else if !isPressed {
				leftWasDown = false
			}
		}

		// 2. Process Right switch bind
		if rightVK != 0 {
			isPressed := isJoyKeyPressed(rightVK, &info)

			if isPressed && !rightWasDown {
				if time.Since(rightLastTrigger) > 180*time.Millisecond {
					rightLastTrigger = time.Now()
					rightWasDown = true
					go m.triggerScroll(-wheelDelta) // Scroll Down (Next Slot)
				}
			} else if !isPressed {
				rightWasDown = false
			}
		}
	}
}

func isJoyKeyPressed(bind uint32, info *joyInfoEx) bool {
	if bind == 0 {
		return false
	}
	if bind == 27000 || bind == 9000 || bind == 0 || bind == 18000 {
		return info.dwPOV == bind
	}
	if bind >= 1 && bind <= 32 {
		mask := uint32(1) << (bind - 1)
		return (info.dwButtons & mask) != 0
	}
	return false
}

func (m *InstantHotbarModule) triggerScroll(delta int32) {
	_procMouseEvent.Call(
		uintptr(mouseeventfWheel),
		0,
		0,
		uintptr(delta),
		0,
	)
}

func (m *InstantHotbarModule) vkName(vk uint32) string {
	if vk == 0 {
		return "Нет"
	}
	switch vk {
	case 1:
		return "Joy: A"
	case 2:
		return "Joy: B"
	case 3:
		return "Joy: X"
	case 4:
		return "Joy: Y"
	case 5:
		return "Joy: LB"
	case 6:
		return "Joy: RB"
	case 7:
		return "Joy: Back"
	case 8:
		return "Joy: Start"
	case 9:
		return "Joy: L3"
	case 10:
		return "Joy: R3"
	case 27000:
		return "Joy: D-Left"
	case 9000:
		return "Joy: D-Right"
	case 0:
		return "Joy: D-Up"
	case 18000:
		return "Joy: D-Down"
	}
	if vk >= 11 && vk <= 32 {
		return fmt.Sprintf("Joy: Btn %d", vk)
	}
	return fmt.Sprintf("Joy: Code %d", vk)
}
