package modules

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/pkg/e"
	win "github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
)

var (
	_zoomUser32               = windows.NewLazySystemDLL("user32.dll")
	_procEnumDisplaySettingsW = _zoomUser32.NewProc("EnumDisplaySettingsW")
	_procGetCursorInfo        = _zoomUser32.NewProc("GetCursorInfo")
	_procGetAsyncKeyState     = _zoomUser32.NewProc("GetAsyncKeyState")
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

var _ module.Config = (*Zoom)(nil)

type Zoom struct {
	modulesutil.DefaultDisabled
	Process    *win.Process
	Error      func(error)
	OnToggle   func(bool, e.ActionCause)
	GetModule  func(string) (module.Module, bool)
	GetBindKey func() string
}

func (conf *Zoom) Create(p module.Property, cause e.ActionCause) (module.Module, error) {
	z := &zoomModule{
		ErrorHandler: modulesutil.ErrorHandler{Error: conf.Error},
		process:      conf.Process,
		onToggle:     conf.OnToggle,
		getModule:    conf.GetModule,
		getBindKey:   conf.GetBindKey,
	}

	m := modulesutil.NewBaseToggleable(z,
		"zoom",
		"Approximates game camera view when hotkey is held",
	)
	m.Edit(p, cause)
	return m, nil
}

func (conf *Zoom) Identifier() string {
	return "zoom"
}

type zoomModule struct {
	modulesutil.ErrorHandler
	mu          sync.Mutex
	process     *win.Process
	onToggle    func(bool, e.ActionCause)
	enabled     bool
	cancel      context.CancelFunc
	addrCache   uintptr
	addrCacheAt time.Time
	getModule   func(string) (module.Module, bool)
	getBindKey  func() string
}

func (z *zoomModule) UpdateState(v bool, cause e.ActionCause, opts ...any) error {
	z.mu.Lock()
	if z.enabled == v {
		z.mu.Unlock()
		return e.ErrValuesIsAlready{Value: v}
	}
	z.enabled = v
	z.mu.Unlock()

	if v {
		z.startLoop()
	} else {
		z.stopLoop()
	}

	z.onToggle(v, cause)
	return nil
}

func (z *zoomModule) State() bool {
	z.mu.Lock()
	defer z.mu.Unlock()
	return z.enabled
}

func (z *zoomModule) Disable(cause e.ActionCause) {
	_ = z.UpdateState(false, cause)
}

func (z *zoomModule) Edit(p module.Property, cause e.ActionCause) {
	modulesutil.SyncState(z, p, cause)
}

func (z *zoomModule) startLoop() {
	z.mu.Lock()
	defer z.mu.Unlock()
	if z.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	z.cancel = cancel
	go z.loop(ctx)
}

func (z *zoomModule) stopLoop() {
	z.mu.Lock()
	cancel := z.cancel
	z.cancel = nil
	z.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (z *zoomModule) invalidateAddrCache() {
	z.mu.Lock()
	z.addrCache = 0
	z.addrCacheAt = time.Time{}
	z.mu.Unlock()
}

func (z *zoomModule) loop(ctx context.Context) {
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
					if noDyn, ok := z.getModule("no_dynamic_fov"); ok {
						noDyn.Disable(e.ActionCauseExternal)
					}
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
					if noDyn, ok := z.getModule("no_dynamic_fov"); ok {
						noDyn.Disable(e.ActionCauseExternal)
					}
					patchedDynFov = false
				}
			}
			hasLastKnown = false
			continue
		}

		bindKey := z.getBindKey()
		vk := charToVK(bindKey)

		if vk == 0 {
			if wasPressed {
				z.writeFOV(originalFOV)
				wasPressed = false
				if patchedDynFov {
					if noDyn, ok := z.getModule("no_dynamic_fov"); ok {
						noDyn.Disable(e.ActionCauseExternal)
					}
					patchedDynFov = false
				}
			}
			continue
		}

		var isPressed bool
		r, _, _ := _procGetAsyncKeyState.Call(uintptr(vk))
		isPressed = (r & 0x8000) != 0

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
			if noDyn, ok := z.getModule("no_dynamic_fov"); ok {
				if t, ok2 := noDyn.(modulesutil.ToggleableModule); ok2 && !t.State() {
					_ = t.UpdateState(true, e.ActionCauseExternal)
					patchedDynFov = true
				}
			}
			if hasLastKnown {
				originalFOV = lastKnownFOV
			} else {
				fov, err := z.readFOV()
				if err != nil {
					z.HandleError("read fov", err)
					z.invalidateAddrCache()
					if patchedDynFov {
						if noDyn, ok := z.getModule("no_dynamic_fov"); ok {
							noDyn.Disable(e.ActionCauseExternal)
						}
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
				if noDyn, ok := z.getModule("no_dynamic_fov"); ok {
					noDyn.Disable(e.ActionCauseExternal)
				}
				patchedDynFov = false
			}
		}
	}
}

func (z *zoomModule) readFOV() (float32, error) {
	addr, err := z.resolveAddr()
	if err != nil {
		return 0, err
	}
	val, err := mem.ReadMemory[float32](z.process, addr)
	if err != nil {
		z.invalidateAddrCache()
		return 0, fmt.Errorf("read fov memory: %w", err)
	}
	return val, nil
}

func (z *zoomModule) writeFOV(val float32) {
	addr, err := z.resolveAddr()
	if err != nil {
		return
	}
	if err := mem.WriteMemory[float32](z.process, addr, val); err != nil {
		z.invalidateAddrCache()
	}
}

func (z *zoomModule) resolveAddr() (uintptr, error) {
	z.mu.Lock()
	cached := z.addrCache
	cachedAt := z.addrCacheAt
	z.mu.Unlock()
	const addrCacheTTL = 2 * time.Second
	if cached != 0 && time.Since(cachedAt) < addrCacheTTL {
		return cached, nil
	}
	addr, err := mem.ResolvePointerAddress(
		z.process,
		mem.MustParsePointer("01921DF8", "30 D8 20 BE0"),
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

func charToVK(char string) uint32 {
	char = strings.ToLower(char)
	if len(char) == 1 {
		r := char[0]
		if r >= 'a' && r <= 'z' {
			return uint32(r - 'a' + 0x41)
		}
		if r >= '0' && r <= '9' {
			return uint32(r - '0' + 0x30)
		}
	}
	switch char {
	case "shift", "lshift":
		return 0xA0
	case "rshift":
		return 0xA1
	case "ctrl", "lctrl":
		return 0xA2
	case "rctrl":
		return 0xA3
	case "alt", "lalt":
		return 0xA4
	case "ralt":
		return 0xA5
	case "space":
		return 0x20
	case "tab":
		return 0x09
	case "enter":
		return 0x0D
	}
	return 0
}
