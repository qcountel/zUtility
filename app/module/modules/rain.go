package modules

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/internal/pkg/win"
)

var rainWeatherSig []byte // TODO: заполнить байтами из SigMaker

const (
	rainPtrBase uintptr = 0x018CA108
	rainOff0    uintptr = 0x118
	rainOff1    uintptr = 0x2C0
	rainOff2    uintptr = 0x0
	rainOff3    uintptr = 0x1E0

	offRainLevel      uintptr = 0x34
	offLightningLevel uintptr = 0x40

	rainPollInterval = 50 * time.Millisecond
	rainCacheTTL     = 5 * time.Second
)

var _ module.Module = (*rainModule)(nil)

// Rain — конфигурация модуля. Вызови Create() чтобы получить модуль.
type Rain struct {
	Process     *win.Process
	Error       func(error)
	AfterChange func()
}

func (c Rain) Create() module.Module {
	return &rainModule{
		process:     c.Process,
		errFn:       c.Error,
		afterChange: c.AfterChange,
	}
}

type rainModule struct {
	// cancelMu охраняет только поле cancel — для защиты от двойного запуска горутины.
	// Всё остальное — атомики.
	cancelMu sync.Mutex
	cancel   context.CancelFunc

	process     *win.Process
	errFn       func(error)
	afterChange func()

	enabled atomic.Bool

	// Кэш адреса weather object с TTL.
	addrCache     atomic.Uintptr
	addrCacheNano atomic.Int64

	// NOP-патч (инициализируется лениво при первом Enable, если sig задана).
	nopMu sync.Mutex
	nop   *win.SignatureNopToggler

	toggle *modulesutil.M3Toggle
}

func (*rainModule) Name() string        { return "Rain" }
func (*rainModule) Description() string { return "Включает дождь. Работает на серверах." }
func (r *rainModule) IsEnabled() bool   { return r.enabled.Load() }

// Enable включает дождь: NOP-патч (если сигнатура задана) + запуск поллинга.
func (r *rainModule) Enable() {
	r.enabled.Store(true)
	r.applyNop(true)
	r.startLoop()
	r.syncToggle(true)
}

// Disable выключает дождь: стоп поллинга → записать 0 → восстановить NOP.
func (r *rainModule) Disable() {
	r.enabled.Store(false)
	r.stopLoop()
	r.writeWeather(0.0, 0.0) // сразу очистить дождь
	r.applyNop(false)
	r.syncToggle(false)
}

func (r *rainModule) CreateObjects() []fyne.CanvasObject {
	toggle := modulesutil.NewM3Toggle(r.enabled.Load())
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
	r.toggle = toggle
	return []fyne.CanvasObject{toggle}
}

// ── внутренние методы ──────────────────────────────────────────────────────

func (r *rainModule) syncToggle(v bool) {
	if r.toggle == nil {
		return
	}
	prev := r.toggle.OnChange
	r.toggle.OnChange = nil
	r.toggle.SetChecked(v)
	r.toggle.OnChange = prev
}

func (r *rainModule) startLoop() {
	r.cancelMu.Lock()
	defer r.cancelMu.Unlock()
	if r.cancel != nil {
		return // уже запущен
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go r.loop(ctx)
}

func (r *rainModule) stopLoop() {
	r.cancelMu.Lock()
	cancel := r.cancel
	r.cancel = nil
	r.cancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (r *rainModule) loop(ctx context.Context) {
	ticker := time.NewTicker(rainPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.writeWeather(1.0, 0.0)
		}
	}
}

// writeWeather записывает rainLevel и lightningLevel в weather object.
func (r *rainModule) writeWeather(rain, lightning float32) {
	base, err := r.resolveWeatherObj()
	if err != nil {
		// молча: процесс мог ещё не запуститься
		return
	}
	if err := win.WriteMemory[float32](r.process, base+offRainLevel, rain); err != nil {
		r.invalidateCache()
		if r.errFn != nil {
			r.errFn(fmt.Errorf("rain: write rainLevel: %w", err))
		}
		return
	}
	if err := win.WriteMemory[float32](r.process, base+offLightningLevel, lightning); err != nil {
		r.invalidateCache()
		if r.errFn != nil {
			r.errFn(fmt.Errorf("rain: write lightningLevel: %w", err))
		}
	}
}

// resolveWeatherObj разрешает pointer chain и возвращает базовый адрес weather object.
func (r *rainModule) resolveWeatherObj() (uintptr, error) {
	if cached := r.addrCache.Load(); cached != 0 {
		if time.Since(time.Unix(0, r.addrCacheNano.Load())) < rainCacheTTL {
			return cached, nil
		}
	}
	addr, err := win.ResolvePointerAddress(
		r.process, r.process.Module,
		rainPtrBase,
		[]uintptr{rainOff0, rainOff1, rainOff2, rainOff3},
	)
	if err != nil {
		r.invalidateCache()
		return 0, fmt.Errorf("rain: resolve pointer chain: %w", err)
	}
	r.addrCache.Store(addr)
	r.addrCacheNano.Store(time.Now().UnixNano())
	return addr, nil
}

func (r *rainModule) invalidateCache() {
	r.addrCache.Store(0)
	r.addrCacheNano.Store(0)
}

// applyNop включает/выключает NOP-патч на инструкцию записи rainLevel.
// Безопасно вызывать когда rainWeatherSig == nil — просто ничего не делает.
func (r *rainModule) applyNop(enable bool) {
	if len(rainWeatherSig) == 0 {
		return
	}
	r.nopMu.Lock()
	defer r.nopMu.Unlock()

	if r.nop == nil {
		t, err := win.SignatureNopTogglerConfig{
			Process:   r.process,
			Module:    r.process.Module,
			Size:      r.process.ModuleSize,
			Signature: rainWeatherSig,
		}.New()
		if err != nil {
			if r.errFn != nil {
				r.errFn(fmt.Errorf("rain: nop init: %w", err))
			}
			return
		}
		r.nop = t
	}

	if err := r.nop.Set(enable); err != nil && r.errFn != nil {
		r.errFn(fmt.Errorf("rain: nop set=%v: %w", enable, err))
	}
}
