package modules

import (
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
)

var (
	// Pointer chain: Minecraft.Windows.exe+018CA108 → [+118] → [+2C0] → [+0] → +1E0 = weather object
	rainBaseAddress = uintptr(0x018CA108)
	rainOffsets     = []uintptr{0x118, 0x2C0, 0x0, 0x1E0}

	// movss [rcx+34], xmm0 — server writes rainLevel
	// F3 0F 11 41 34  C3  B8 BA 0B 00 00
	rainLevelSig = []byte{0xF3, 0x0F, 0x11, 0x41, 0x34, 0xC3, 0xB8, 0xBA, 0x0B, 0x00, 0x00}

	// movss [rcx+40], xmm0 — server writes lightningLevel
	// F3 0F 11 41 40  C3  B8 BC 0B 00 00
	lightLevelSig = []byte{0xF3, 0x0F, 0x11, 0x41, 0x40, 0xC3, 0xB8, 0xBC, 0x0B, 0x00, 0x00}

	// Original bytes for restore (only 5-byte movss instruction)
	rainMovssOrig  = [5]byte{0xF3, 0x0F, 0x11, 0x41, 0x34}
	lightMovssOrig = [5]byte{0xF3, 0x0F, 0x11, 0x41, 0x40}
)

const (
	rainAddrTTL         = 5 * time.Second
	rainOffsetRain      = uintptr(0x34)
	rainOffsetLightning = uintptr(0x40)
)

var _ module.Module = (*rain)(nil)

type Rain struct {
	Process     *win.Process
	Error       func(error)
	AfterChange func()
}

func (conf Rain) Create() module.Module {
	return &rain{
		proc:        conf.Process,
		errFn:       conf.Error,
		afterChange: conf.AfterChange,
	}
}

type rain struct {
	proc        *win.Process
	errFn       func(error)
	afterChange func()

	enabled  atomic.Bool
	cancelMu sync.Mutex
	running  bool
	stopChan chan struct{}

	// Weather object address cache (TTL 5s)
	addrMu     sync.Mutex
	cachedAddr uintptr
	addrExpiry time.Time

	// NOP patch addresses (cached forever — code doesn't move)
	rainNopAddr  atomic.Uintptr
	lightNopAddr atomic.Uintptr
}

func (*rain) Name() string        { return "Rain" }
func (*rain) Description() string { return "Toggles rain. Works on servers." }

func (r *rain) CreateObjects() []fyne.CanvasObject {
	toggle := modulesutil.NewM3Toggle(r.IsEnabled())
	toggle.OnChange = func(b bool) {
		if b {
			r.Enable()
		} else {
			r.Disable()
		}
		if r.afterChange != nil {
			r.afterChange()
		}
	}
	return []fyne.CanvasObject{toggle}
}

func (r *rain) IsEnabled() bool { return r.enabled.Load() }

func (r *rain) Enable() {
	if r.enabled.Load() {
		return
	}
	if err := r.applyNops(); err != nil {
		if r.errFn != nil {
			r.errFn(err)
		}
		return
	}
	r.enabled.Store(true)
	r.startPolling()
}

func (r *rain) Disable() {
	if !r.enabled.Load() {
		return
	}
	r.stopPolling()
	// Immediately write 0 so rain disappears right away
	if addr, err := r.getWeatherAddr(); err == nil {
		_ = win.WriteMemory[float32](r.proc, addr+rainOffsetRain, 0.0)
		_ = win.WriteMemory[float32](r.proc, addr+rainOffsetLightning, 0.0)
	}
	r.restoreNops()
	r.enabled.Store(false)
}

// --- Polling loop ---

func (r *rain) startPolling() {
	r.cancelMu.Lock()
	defer r.cancelMu.Unlock()
	if r.running {
		return
	}
	r.running = true
	r.stopChan = make(chan struct{})

	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-r.stopChan:
				return
			case <-ticker.C:
				addr, err := r.getWeatherAddr()
				if err != nil {
					r.invalidateAddr()
					continue
				}
				if err := win.WriteMemory[float32](r.proc, addr+rainOffsetRain, 1.0); err != nil {
					r.invalidateAddr()
				}
			}
		}
	}()
}

func (r *rain) stopPolling() {
	r.cancelMu.Lock()
	defer r.cancelMu.Unlock()
	if !r.running {
		return
	}
	r.running = false
	close(r.stopChan)
}

// --- Pointer chain ---

func (r *rain) getWeatherAddr() (uintptr, error) {
	r.addrMu.Lock()
	defer r.addrMu.Unlock()
	if r.cachedAddr != 0 && time.Now().Before(r.addrExpiry) {
		return r.cachedAddr, nil
	}
	mod, _, err := r.proc.GetModuleInfo()
	if err != nil {
		return 0, err
	}
	addr, err := win.ResolvePointerAddress(r.proc, mod, rainBaseAddress, rainOffsets)
	if err != nil {
		return 0, err
	}
	r.cachedAddr = addr
	r.addrExpiry = time.Now().Add(rainAddrTTL)
	return addr, nil
}

func (r *rain) invalidateAddr() {
	r.addrMu.Lock()
	r.cachedAddr = 0
	r.addrMu.Unlock()
}

// --- NOP patch ---

func (r *rain) findNopAddr(cache *atomic.Uintptr, sig []byte) (uintptr, error) {
	if addr := cache.Load(); addr != 0 {
		return addr, nil
	}
	mod, size, err := r.proc.GetModuleInfo()
	if err != nil {
		return 0, err
	}
	addr, err := win.ScanSignature(r.proc, size, mod, sig)
	if err != nil {
		return 0, err
	}
	cache.Store(addr)
	return addr, nil
}

func (r *rain) applyNops() error {
	rainAddr, err := r.findNopAddr(&r.rainNopAddr, rainLevelSig)
	if err != nil {
		return err
	}
	lightAddr, err := r.findNopAddr(&r.lightNopAddr, lightLevelSig)
	if err != nil {
		return err
	}
	nop := []byte{0x90, 0x90, 0x90, 0x90, 0x90}
	if err := win.Patch(r.proc, rainAddr, nop); err != nil {
		return err
	}
	return win.Patch(r.proc, lightAddr, nop)
}

func (r *rain) restoreNops() {
	if addr := r.rainNopAddr.Load(); addr != 0 {
		_ = win.Patch(r.proc, addr, rainMovssOrig[:])
	}
	if addr := r.lightNopAddr.Load(); addr != 0 {
		_ = win.Patch(r.proc, addr, lightMovssOrig[:])
	}
}
