package modules

import (
	"sync/atomic"

	"fyne.io/fyne/v2"
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
)

var _ module.Module = (*rain)(nil)

// movss [rcx+34], xmm0 — серверная запись rainLevel
// Bytes: F3 0F 11 41 34  C3  B8 BA 0B 00 00
var rainLevelWriteSig = []byte{0xF3, 0x0F, 0x11, 0x41, 0x34, 0xC3, 0xB8, 0xBA, 0x0B, 0x00, 0x00}

// movss [rcx+40], xmm0 — серверная запись lightningLevel
// Bytes: F3 0F 11 41 40  C3  B8 BC 0B 00 00
var lightLevelWriteSig = []byte{0xF3, 0x0F, 0x11, 0x41, 0x40, 0xC3, 0xB8, 0xBC, 0x0B, 0x00, 0x00}

// Оригинальные байты инструкций (только 5 байт movss)
var rainMovssOrig  = [5]byte{0xF3, 0x0F, 0x11, 0x41, 0x34}
var lightMovssOrig = [5]byte{0xF3, 0x0F, 0x11, 0x41, 0x40}

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

	// Адреса NOP-патчей (кэшируются после первого ScanSignature)
	rainNopAddr  atomic.Uintptr
	lightNopAddr atomic.Uintptr

	enabled atomic.Bool
}

func (*rain) Name() string        { return "Rain" }
func (*rain) Description() string { return "Включает дождь. Работает и на серверах." }

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
}

func (r *rain) Disable() {
	if !r.enabled.Load() {
		return
	}
	r.restoreNops()
	r.enabled.Store(false)
}

// findSigAddr ищет адрес инструкции по сигнатуре (кэшируется навсегда —
// код модуля не перемещается в памяти).
func (r *rain) findSigAddr(cache *atomic.Uintptr, sig []byte) (uintptr, error) {
	if addr := cache.Load(); addr != 0 {
		return addr, nil
	}
	addr, err := win.ScanSignature(r.proc, r.proc.ModuleSize, r.proc.Module, sig)
	if err != nil {
		return 0, err
	}
	cache.Store(addr)
	return addr, nil
}

func (r *rain) applyNops() error {
	rainAddr, err := r.findSigAddr(&r.rainNopAddr, rainLevelWriteSig)
	if err != nil {
		return err
	}
	lightAddr, err := r.findSigAddr(&r.lightNopAddr, lightLevelWriteSig)
	if err != nil {
		return err
	}
	nop := win.NopSig(5)
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
