package modules

import (
	"context"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
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
	_zoomUser32               = windows.NewLazySystemDLL("user32.dll")
	_procEnumDisplaySettingsW = _zoomUser32.NewProc("EnumDisplaySettingsW")
	_procGetCursorInfo        = _zoomUser32.NewProc("GetCursorInfo")
)

type cursorInfo struct {
	cbSize      uint32
	flags       uint32
	hCursor     uintptr
	ptScreenPos struct {
		x, y int32
	}
}

const cursorShowing = 0x00000001

func isCursorVisible() bool {
	var ci cursorInfo
	ci.cbSize = uint32(unsafe.Sizeof(ci))
	r, _, _ := _procGetCursorInfo.Call(uintptr(unsafe.Pointer(&ci)))
	if r == 0 {
		return true
	}
	return (ci.flags & cursorShowing) != 0
}

type devModeW struct {
	dmDeviceName         [32]uint16
	dmSpecVersion        uint16
	dmDriverVersion      uint16
	dmSize               uint16
	dmDriverExtra        uint16
	dmFields             uint32
	dmPositionX          int32
	dmPositionY          int32
	dmDisplayOrientation uint32
	dmDisplayFixedOutput uint32
	dmColor              int16
	dmDuplex             int16
	dmYResolution        int16
	dmTTOption           int16
	dmCollate            int16
	dmFormName           [32]uint16
	dmLogPixels          uint16
	dmBitsPerPel         uint32
	dmPelsWidth          uint32
	dmPelsHeight         uint32
	dmDisplayFlags       uint32
	dmDisplayFrequency   uint32
	_                    [72]byte
}

const enumCurrentSettings = ^uint32(0)

func getDisplayHz() uint32 {
	var dm devModeW
	dm.dmSize = uint16(unsafe.Sizeof(dm))
	r, _, _ := _procEnumDisplaySettingsW.Call(
		0,
		uintptr(enumCurrentSettings),
		uintptr(unsafe.Pointer(&dm)),
	)
	if r == 0 || dm.dmDisplayFrequency < 30 {
		return 60
	}
	return dm.dmDisplayFrequency
}

func calcTickInterval(hz uint32) time.Duration {
	if hz < 30 {
		hz = 60
	}
	ms := 500 / uint32(hz)
	if ms < 1 {
		ms = 1
	}
	return time.Duration(ms) * time.Millisecond
}

var _ module.Module = (*ZoomV2Module)(nil)

type ZoomV2 struct {
	Process      *win.Process
	Error        func(error)
	AfterChange  func()
	NoDynamicFov module.Module
	Localize     func(ru, en string) string
}

func (conf ZoomV2) Create() module.Module {
	return &ZoomV2Module{
		process:      conf.Process,
		errFn:        conf.Error,
		afterChange:  conf.AfterChange,
		noDynamicFov: conf.NoDynamicFov,
		localize:     conf.Localize,
	}
}

type ZoomV2Module struct {
	mu          sync.Mutex
	process     *win.Process
	errFn       func(error)
	afterChange func()

	enabled      bool
	bindVK       uint32
	addrCache    uintptr
	addrCacheAt  time.Time
	cancel       context.CancelFunc
	noDynamicFov module.Module
	localize     func(ru, en string) string

	toggle    *modulesutil.M3Toggle
	bindLabel *widget.Label
}

func (*ZoomV2Module) Name() string { return "Zoom" }
func (*ZoomV2Module) Description() string {
	return "Приближает изображение при удержании выбранной клавиши."
}

func (z *ZoomV2Module) IsEnabled() bool {
	z.mu.Lock()
	defer z.mu.Unlock()
	return z.enabled
}

func (z *ZoomV2Module) BindVK() uint32 {
	z.mu.Lock()
	defer z.mu.Unlock()
	return z.bindVK
}

func (z *ZoomV2Module) SetBindVK(vk uint32) {
	z.mu.Lock()
	z.bindVK = vk
	z.mu.Unlock()
	if z.bindLabel != nil {
		z.bindLabel.SetText(z.vkName(vk))
	}
}

func (z *ZoomV2Module) Enable() {
	z.mu.Lock()
	z.enabled = true
	z.mu.Unlock()
	z.startLoop()
	if z.toggle != nil {
		prev := z.toggle.OnChange
		z.toggle.OnChange = nil
		z.toggle.SetChecked(true)
		z.toggle.OnChange = prev
	}
}

func (z *ZoomV2Module) Disable() {
	z.mu.Lock()
	z.enabled = false
	z.mu.Unlock()
	z.stopLoop()
	if z.toggle != nil {
		prev := z.toggle.OnChange
		z.toggle.OnChange = nil
		z.toggle.SetChecked(false)
		z.toggle.OnChange = prev
	}
}

func (z *ZoomV2Module) CreateObjects() []fyne.CanvasObject {
	toggle := modulesutil.NewM3Toggle(z.enabled)
	toggle.OnChange = func(b bool) {
		z.mu.Lock()
		z.enabled = b
		z.mu.Unlock()
		if b {
			z.startLoop()
		} else {
			z.stopLoop()
		}
		if z.afterChange != nil {
			z.afterChange()
		}
	}
	z.toggle = toggle

	z.mu.Lock()
	initVK := z.bindVK
	z.mu.Unlock()

	keyLabel := widget.NewLabel(z.vkName(initVK))
	z.bindLabel = keyLabel

	var captureCancel context.CancelFunc

	clearBtn := widget.NewButtonWithIcon("", theme.ContentClearIcon(), nil)
	clearBtn.Importance = widget.LowImportance

	bindBtn := widget.NewButton(z.t("Бинд", "Bind"), nil)
	bindBtn.Importance = widget.LowImportance

	clearBtn.OnTapped = func() {
		if captureCancel != nil {
			captureCancel()
			captureCancel = nil
		}
		z.mu.Lock()
		z.bindVK = 0
		z.mu.Unlock()
		keyLabel.SetText(z.vkName(0))
		bindBtn.SetText(z.t("Бинд", "Bind"))
		if z.afterChange != nil {
			z.afterChange()
		}
	}

	bindBtn.OnTapped = func() {

		if captureCancel != nil {
			captureCancel()
			captureCancel = nil
			bindBtn.SetText(z.t("Бинд", "Bind"))
			z.mu.Lock()
			vk := z.bindVK
			z.mu.Unlock()
			keyLabel.SetText(z.vkName(vk))
			return
		}

		bindBtn.SetText(z.t("Нажмите...", "Press..."))
		keyLabel.SetText("...")

		captureCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		captureCancel = cancel

		go func() {
			defer func() {
				cancel()
				captureCancel = nil
			}()

			vk, ok := captureNextZoomKey(captureCtx)

			fyne.Do(func() {
				if ok && vk != 0 {
					z.mu.Lock()
					z.bindVK = vk
					z.mu.Unlock()
					keyLabel.SetText(z.vkName(vk))
					if z.afterChange != nil {
						z.afterChange()
					}
				} else {
					z.mu.Lock()
					cur := z.bindVK
					z.mu.Unlock()
					keyLabel.SetText(z.vkName(cur))
				}
				bindBtn.SetText(z.t("Бинд", "Bind"))
			})
		}()
	}

	row := container.New(layout.NewHBoxLayout(), toggle, keyLabel, clearBtn, bindBtn)
	return []fyne.CanvasObject{row}
}

func captureNextZoomKey(ctx context.Context) (uint32, bool) {
	time.Sleep(250 * time.Millisecond)

	resultCh := make(chan uint32, 2)

	go func() {
		ticker := time.NewTicker(16 * time.Millisecond)
		defer ticker.Stop()
		skipVKs := map[uint32]bool{
			0x00: true, 0x07: true, 0x0A: true, 0x0B: true,
			0x1B: true, 0x5B: true, 0x5C: true, 0x5D: true, 0xFF: true,
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for vk := uint32(0x01); vk <= 0xFE; vk++ {
					if skipVKs[vk] {
						continue
					}
					r, _, _ := _procGetAsyncKeyState.Call(uintptr(vk))
					if r&0x8000 != 0 {
						for {
							r2, _, _ := _procGetAsyncKeyState.Call(uintptr(vk))
							if r2&0x8000 == 0 {
								break
							}
							time.Sleep(16 * time.Millisecond)
						}
						select {
						case resultCh <- vk:
						default:
						}
						return
					}
				}
			}
		}
	}()

	go func() {
		doneCh := make(chan struct{})
		go func() {
			<-ctx.Done()
			close(doneCh)
		}()
		if vk, ok := win.CaptureGamepadButton(doneCh); ok {
			select {
			case resultCh <- vk:
			default:
			}
		}
	}()

	select {
	case <-ctx.Done():
		return 0, false
	case vk := <-resultCh:
		return vk, true
	}
}

func (z *ZoomV2Module) startLoop() {
	z.mu.Lock()
	defer z.mu.Unlock()
	if z.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	z.cancel = cancel
	go z.loop(ctx)
}

func (z *ZoomV2Module) stopLoop() {
	z.mu.Lock()
	cancel := z.cancel
	z.cancel = nil
	z.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (z *ZoomV2Module) invalidateAddrCache() {
	z.mu.Lock()
	z.addrCache = 0
	z.addrCacheAt = time.Time{}
	z.mu.Unlock()
}

func (z *ZoomV2Module) loop(ctx context.Context) {
	var (
		wasPressed    bool
		originalFOV   float32
		zoomedFOV     float32
		lastKnownFOV  float32
		hasLastKnown  bool
		patchedDynFov bool
	)

	hz := getDisplayHz()
	interval := calcTickInterval(hz)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if wasPressed {
				z.writeFOV(originalFOV)
				if patchedDynFov {
					z.noDynamicFov.Disable()
				}
			}
			return
		case <-ticker.C:
		}

		if isCursorVisible() {
			if wasPressed {
				z.writeFOV(originalFOV)
				wasPressed = false
				if patchedDynFov {
					z.noDynamicFov.Disable()
					patchedDynFov = false
				}
			}
			hasLastKnown = false
			continue
		}

		z.mu.Lock()
		vk := z.bindVK
		z.mu.Unlock()

		if vk == 0 {
			continue
		}

		var isPressed bool
		if win.IsGamepadVK(vk) {
			isPressed = win.IsGamepadVKHeld(vk)
		} else {
			r, _, _ := _procGetAsyncKeyState.Call(uintptr(vk))
			isPressed = (r & 0x8000) != 0
		}

		switch {
		case !wasPressed && !isPressed:

			if fov, err := z.readFOV(); err == nil {
				lastKnownFOV = fov
				hasLastKnown = true
			} else {
				z.invalidateAddrCache()
				hasLastKnown = false
			}

		case !wasPressed && isPressed:

			if z.noDynamicFov != nil && !z.noDynamicFov.IsEnabled() {
				z.noDynamicFov.Enable()
				patchedDynFov = true
			}
			if hasLastKnown {
				originalFOV = lastKnownFOV
			} else {
				fov, err := z.readFOV()
				if err != nil {
					if z.errFn != nil {
						z.errFn(fmt.Errorf("zoom: read fov: %w", err))
					}
					z.invalidateAddrCache()
					if patchedDynFov {
						z.noDynamicFov.Disable()
						patchedDynFov = false
					}
					continue
				}
				originalFOV = fov
			}
			zoomedFOV = originalFOV / 4.0
			wasPressed = true
			z.writeFOV(zoomedFOV)

		case wasPressed && isPressed:
			z.writeFOV(zoomedFOV)

		case wasPressed && !isPressed:
			z.writeFOV(originalFOV)
			wasPressed = false
			lastKnownFOV = originalFOV
			hasLastKnown = true

			if patchedDynFov {
				z.noDynamicFov.Disable()
				patchedDynFov = false
			}
		}
	}
}

func (z *ZoomV2Module) readFOV() (float32, error) {
	addr, err := z.resolveAddr()
	if err != nil {
		return 0, err
	}
	val, err := win.ReadMemory[float32](z.process, addr)
	if err != nil {

		z.invalidateAddrCache()
		return 0, fmt.Errorf("read fov memory: %w", err)
	}
	return val, nil
}

func (z *ZoomV2Module) writeFOV(val float32) {
	addr, err := z.resolveAddr()
	if err != nil {
		return
	}
	if err := win.WriteMemory[float32](z.process, addr, val); err != nil {

		z.invalidateAddrCache()
	}
}

func (z *ZoomV2Module) resolveAddr() (uintptr, error) {
	z.mu.Lock()
	cached := z.addrCache
	cachedAt := z.addrCacheAt
	z.mu.Unlock()
	const addrCacheTTL = 2 * time.Second
	if cached != 0 && time.Since(cachedAt) < addrCacheTTL {
		return cached, nil
	}
	addr, err := win.ResolvePointerAddress(
		z.process, z.process.Module,
		0x01921DF8,
		[]uintptr{0x30, 0xD8, 0x20, 0xBE0},
	)
	if err != nil {
		return 0, fmt.Errorf("resolve fov: %w", err)
	}
	z.mu.Lock()
	z.addrCache = addr
	z.addrCacheAt = time.Now()
	z.mu.Unlock()
	return addr, nil
}

func zoomVKName(vk uint32) string {
	if vk == 0 {
		return "None"
	}

	if win.IsGamepadVK(vk) {
		if name := win.GPName(vk); name != "" {
			return name
		}
		return fmt.Sprintf("GP_%02X", vk)
	}
	for _, k := range zoomKeys {
		if k.VK == vk {
			return k.Name
		}
	}

	if name, ok := zoomAllKeyNames[vk]; ok {
		return name
	}
	return fmt.Sprintf("VK_%02X", vk)
}

func (z *ZoomV2Module) t(ru, en string) string {
	if z.localize == nil {
		return en
	}
	return z.localize(ru, en)
}

func (z *ZoomV2Module) vkName(vk uint32) string {
	if vk == 0 {
		return z.t("Нет", "None")
	}
	return zoomVKName(vk)
}

var zoomAllKeyNames = map[uint32]string{
	0x01: "LMB", 0x02: "RMB", 0x04: "MMB", 0x05: "Mouse4", 0x06: "Mouse5",
	0x08: "Backspace", 0x09: "Tab", 0x0D: "Enter",
	0x10: "Shift", 0x11: "Ctrl", 0x12: "Alt",
	0x13: "Pause", 0x14: "CapsLock",
	0x20: "Space",
	0x21: "PageUp", 0x22: "PageDown",
	0x23: "End", 0x24: "Home",
	0x25: "Left", 0x26: "Up", 0x27: "Right", 0x28: "Down",
	0x2D: "Insert", 0x2E: "Delete",
	0x30: "0", 0x31: "1", 0x32: "2", 0x33: "3", 0x34: "4",
	0x35: "5", 0x36: "6", 0x37: "7", 0x38: "8", 0x39: "9",
	0x41: "A", 0x42: "B", 0x43: "C", 0x44: "D", 0x45: "E",
	0x46: "F", 0x47: "G", 0x48: "H", 0x49: "I", 0x4A: "J",
	0x4B: "K", 0x4C: "L", 0x4D: "M", 0x4E: "N", 0x4F: "O",
	0x50: "P", 0x51: "Q", 0x52: "R", 0x53: "S", 0x54: "T",
	0x55: "U", 0x56: "V", 0x57: "W", 0x58: "X", 0x59: "Y",
	0x5A: "Z",
	0x60: "Num0", 0x61: "Num1", 0x62: "Num2", 0x63: "Num3",
	0x64: "Num4", 0x65: "Num5", 0x66: "Num6", 0x67: "Num7",
	0x68: "Num8", 0x69: "Num9",
	0x6A: "Num*", 0x6B: "Num+", 0x6D: "Num-", 0x6E: "Num.", 0x6F: "Num/",
	0x70: "F1", 0x71: "F2", 0x72: "F3", 0x73: "F4",
	0x74: "F5", 0x75: "F6", 0x76: "F7", 0x77: "F8",
	0x78: "F9", 0x79: "F10", 0x7A: "F11", 0x7B: "F12",
	0x7C: "F13", 0x7D: "F14", 0x7E: "F15", 0x7F: "F16",
	0x80: "F17", 0x81: "F18", 0x82: "F19", 0x83: "F20",
	0x84: "F21", 0x85: "F22", 0x86: "F23", 0x87: "F24",
	0x90: "NumLock", 0x91: "ScrollLock",
	0xA0: "Shift (L)", 0xA1: "Shift (R)",
	0xA2: "Ctrl (L)", 0xA3: "Ctrl (R)",
	0xA4: "Alt (L)", 0xA5: "Alt (R)",
	0xBA: ";", 0xBB: "=", 0xBC: ",", 0xBD: "-", 0xBE: ".",
	0xBF: "/", 0xC0: "`", 0xDB: "[", 0xDC: "\\", 0xDD: "]", 0xDE: "'",
}

var zoomKeys = []struct {
	Name string
	VK   uint32
}{
	{"None", 0},
	{"F1", 0x70}, {"F2", 0x71}, {"F3", 0x72}, {"F4", 0x73},
	{"F5", 0x74}, {"F6", 0x75}, {"F7", 0x76}, {"F8", 0x77},
	{"F9", 0x78}, {"F10", 0x79}, {"F11", 0x7A}, {"F12", 0x7B},
	{"Insert", 0x2D}, {"Delete", 0x2E},
	{"Home", 0x24}, {"End", 0x23},
	{"PageUp", 0x21}, {"PageDown", 0x22},
	{"Pause", 0x13}, {"ScrollLock", 0x91},
	{"C", 0x43}, {"V", 0x56}, {"X", 0x58}, {"Z", 0x5A},
	{"Alt (Left)", 0xA4}, {"Alt (Right)", 0xA5},
	{"Ctrl (Left)", 0xA2}, {"Ctrl (Right)", 0xA3},
	{"Caps Lock", 0x14},
	{"Num 0", 0x60}, {"Num 1", 0x61}, {"Num 2", 0x62}, {"Num 3", 0x63},
	{"Num 4", 0x64}, {"Num 5", 0x65}, {"Num 6", 0x66}, {"Num 7", 0x67},
	{"Num 8", 0x68}, {"Num 9", 0x69},
	{"Num *", 0x6A}, {"Num +", 0x6B}, {"Num -", 0x6D}, {"Num /", 0x6F},
}
