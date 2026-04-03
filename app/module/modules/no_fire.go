package modules

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
)

const onFireBit = byte(1 << 0)

var (
	onFirePtrBase uintptr = 0x01921DF8
	onFireOffsets         = []uintptr{0x18, 0x60, 0xF0, 0x0, 0x10}
)

const addrCacheTTL = 10 * time.Second

var _ module.Module = (*noFire)(nil)

type NoFire struct {
	Process     *win.Process
	Error       func(error)
	AfterChange func()
}

func (conf NoFire) Create() module.Module {
	return &noFire{
		proc:        conf.Process,
		errFn:       conf.Error,
		afterChange: conf.AfterChange,
		stopChan:    make(chan struct{}),
		uiToggle:    modulesutil.NewM3Toggle(false),
	}
}

type noFire struct {
	proc        *win.Process
	errFn       func(error)
	afterChange func()
	enabled     bool

	stopChan chan struct{}
	looping  bool
	mu       sync.Mutex

	cachedAddr   uintptr
	cachedAddrAt time.Time
	cachedAddrMu sync.Mutex

	uiToggle *modulesutil.M3Toggle
}

func (*noFire) Name() string { return "NoFire" }
func (*noFire) Description() string {
	return "Clears the ON_FIRE flag continuously. Uses active polling and may increase CPU usage by 7-10%."
}

func (n *noFire) CreateObjects() []fyne.CanvasObject {
	n.uiToggle.OnChange = func(checked bool) {
		if checked {
			n.Enable()
		} else {
			n.Disable()
		}
		if n.afterChange != nil {
			n.afterChange()
		}
	}
	return []fyne.CanvasObject{
		container.New(layout.NewHBoxLayout(), n.uiToggle),
	}
}

func (n *noFire) Enable() {
	n.enabled = true
	if n.uiToggle != nil {
		prev := n.uiToggle.OnChange
		n.uiToggle.OnChange = nil
		n.uiToggle.SetChecked(true)
		n.uiToggle.OnChange = prev
	}
	n.startLoop()
}

func (n *noFire) Disable() {
	n.enabled = false
	n.stopLoop()
	n.clearFireBit()
	n.invalidateCache()
	if n.uiToggle != nil {
		prev := n.uiToggle.OnChange
		n.uiToggle.OnChange = nil
		n.uiToggle.SetChecked(false)
		n.uiToggle.OnChange = prev
	}
}

func (n *noFire) IsEnabled() bool { return n.enabled }

func (n *noFire) startLoop() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.looping {
		return
	}
	n.looping = true
	n.stopChan = make(chan struct{})

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		var cachedAddr uintptr
		var lastCacheTime time.Time
		const cacheInterval = 3 * time.Second

		addr, err := n.getAddr()
		if err == nil {
			cachedAddr = addr
			lastCacheTime = time.Now()
		}

		for {
			select {
			case <-n.stopChan:
				return
			default:
				if time.Since(lastCacheTime) > cacheInterval {
					if addr, err := n.getAddr(); err == nil {
						cachedAddr = addr
						lastCacheTime = time.Now()
					}
				}

				if cachedAddr != 0 {
					if b, err := win.ReadMemory[byte](n.proc, cachedAddr); err == nil {
						if b&onFireBit != 0 {
							_ = win.WriteMemory[byte](n.proc, cachedAddr, b&^onFireBit)
						}
					}
				}

<<<<<<< HEAD
				runtime.Gosched()
=======
				time.Sleep(1 * time.Millisecond)
>>>>>>> 8756ae2 (Fix bugs)
			}
		}
	}()
}

func (n *noFire) stopLoop() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.looping {
		return
	}
	n.looping = false
	close(n.stopChan)
}

func (n *noFire) clearFireBit() {
	addr, err := n.getAddr()
	if err != nil {
		n.invalidateCache()
		return
	}
	b, err := win.ReadMemory[byte](n.proc, addr)
	if err != nil {
		n.invalidateCache()
		return
	}
	if b&onFireBit != 0 {
		_ = win.WriteMemory[byte](n.proc, addr, b&^onFireBit)
	}
}

func (n *noFire) getAddr() (uintptr, error) {
	n.cachedAddrMu.Lock()
	defer n.cachedAddrMu.Unlock()
	if n.cachedAddr != 0 && time.Since(n.cachedAddrAt) < addrCacheTTL {
		return n.cachedAddr, nil
	}
	mod, _, err := n.proc.GetModuleInfo()
	if err != nil {
		return 0, fmt.Errorf("no_fire: get module info: %w", err)
	}
	addr, err := win.ResolvePointerAddress(n.proc, mod, onFirePtrBase, onFireOffsets)
	if err != nil {
		return 0, fmt.Errorf("no_fire: resolve pointer: %w", err)
	}
	n.cachedAddr = addr
	n.cachedAddrAt = time.Now()
	return addr, nil
}

func (n *noFire) invalidateCache() {
	n.cachedAddrMu.Lock()
	n.cachedAddr = 0
	n.cachedAddrMu.Unlock()
}
