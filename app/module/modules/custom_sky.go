package modules

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/pkg/asm"
	"github.com/something-that-is-cool/zutil/pkg/e"
	win "github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem/memutil"
)

const (
	offsetGetSkyColor1 = 0x00b96810
	offsetGetSkyColor2 = 0x00b96f90
	offsetGetFogColor  = 0x00b8e5b0
)

type mceColor struct {
	R, G, B, A float32
}

var _ module.Config = (*CustomSky)(nil)

type CustomSky struct {
	Process    *win.Process
	Error      func(error)
	OnToggle   func(bool, e.ActionCause)
	GetModule  func(string) (module.Module, bool)
}

func (conf *CustomSky) Create(p module.Property, cause e.ActionCause) (module.Module, error) {
	cs := &customSkyModule{
		ErrorHandler: modulesutil.ErrorHandler{Error: conf.Error},
		process:      conf.Process,
		onToggle:     conf.OnToggle,
		getModule:    conf.GetModule,
	}

	m := modulesutil.NewBaseToggleable(cs,
		"custom sky",
		"intercepts dimension sky and fog calculations to apply custom colors",
	)
	m.Edit(p, cause)
	return m, nil
}

func (conf *CustomSky) DefaultProperty() module.Property {
	return module.Property{
		Enabled: false,
	}
}

func (conf *CustomSky) Identifier() string {
	return "custom_sky"
}

type customSkyModule struct {
	modulesutil.ErrorHandler
	mu         sync.Mutex
	process    *win.Process
	onToggle   func(bool, e.ActionCause)
	enabled    bool
	cancel     context.CancelFunc
	getModule  func(string) (module.Module, bool)
	skyDetour1 *memutil.ProxiedDetour[mceColor]
	skyDetour2 *memutil.ProxiedDetour[mceColor]
	fogDetour  *memutil.ProxiedDetour[mceColor]
}

func (m *customSkyModule) UpdateState(v bool, cause e.ActionCause, opts ...any) error {
	m.mu.Lock()
	if m.enabled == v {
		m.mu.Unlock()
		return e.ErrValuesIsAlready{Value: v}
	}
	m.enabled = v
	m.mu.Unlock()

	if v {
		m.startLoop()
	} else {
		m.stopLoop()
	}

	m.onToggle(v, cause)
	return nil
}

func (m *customSkyModule) State() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

func (m *customSkyModule) Disable(cause e.ActionCause) {
	_ = m.UpdateState(false, cause)
}

func (m *customSkyModule) Edit(p module.Property, cause e.ActionCause) {
	modulesutil.SyncState(m, p, cause)
}

func (m *customSkyModule) startLoop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	go m.loop(ctx)
}

func (m *customSkyModule) stopLoop() {
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

func (m *customSkyModule) loop(ctx context.Context) {
	var (
		lastR, lastG, lastB float32 = -1, -1, -1
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
				m.HandleError("enable detours", err)
				m.stopLoop()
				return
			}
			initialized = true
		}

		r, g, b := m.getRGB()
		if r != lastR || g != lastG || b != lastB {
			m.writeColor(r, g, b)
			lastR, lastG, lastB = r, g, b
		}
	}
}

func (m *customSkyModule) enableDetours() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mod, err := m.process.GetModuleInfo()
	if err != nil {
		return fmt.Errorf("get module info: %w", err)
	}

	skyAddr1 := mod.Address + offsetGetSkyColor1
	m.skyDetour1 = memutil.NewProxiedDetour[mceColor](m.process, skyAddr1, 12)
	err = m.skyDetour1.Enable(customSkyUserCode, mceColor{R: 1.0, G: 1.0, B: 1.0, A: 1.0})
	if err != nil {
		return fmt.Errorf("enable sky detour 1: %w", err)
	}

	skyAddr2 := mod.Address + offsetGetSkyColor2
	m.skyDetour2 = memutil.NewProxiedDetour[mceColor](m.process, skyAddr2, 12)
	err = m.skyDetour2.Enable(customSkyUserCode, mceColor{R: 1.0, G: 1.0, B: 1.0, A: 1.0})
	if err != nil {
		m.skyDetour1.Disable()
		m.skyDetour1 = nil
		return fmt.Errorf("enable sky detour 2: %w", err)
	}

	fogAddr := mod.Address + offsetGetFogColor
	m.fogDetour = memutil.NewProxiedDetour[mceColor](m.process, fogAddr, 10)
	err = m.fogDetour.Enable(customSkyUserCode, mceColor{R: 1.0, G: 1.0, B: 1.0, A: 1.0})
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

func (m *customSkyModule) writeColor(r, g, b float32) {
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

func (m *customSkyModule) getRGB() (float32, float32, float32) {
	rVal, gVal, bVal := float32(1.0), float32(1.0), float32(1.0)
	if mod, ok := m.getModule("custom_sky_r"); ok {
		if valMod, ok2 := mod.(modulesutil.ModuleWithValue[float64]); ok2 {
			v, _ := valMod.Value()
			rVal = float32(v)
		}
	}
	if mod, ok := m.getModule("custom_sky_g"); ok {
		if valMod, ok2 := mod.(modulesutil.ModuleWithValue[float64]); ok2 {
			v, _ := valMod.Value()
			gVal = float32(v)
		}
	}
	if mod, ok := m.getModule("custom_sky_b"); ok {
		if valMod, ok2 := mod.(modulesutil.ModuleWithValue[float64]); ok2 {
			v, _ := valMod.Value()
			bVal = float32(v)
		}
	}
	return rVal, gVal, bVal
}

func customSkyUserCode(valAddr uintptr) []byte {
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

type customSkyColorModule struct {
	modulesutil.ErrorHandler
	mu             sync.Mutex
	onValueChanged func(float64, e.ActionCause)
	val            float64
	min, max, def  float64
	name           string
	desc           string
}

func (m *customSkyColorModule) Name() string        { return m.name }
func (m *customSkyColorModule) Description() string { return m.desc }

func (m *customSkyColorModule) Disable(cause e.ActionCause) {
	_ = m.SetValue(m.def, cause)
}

func (m *customSkyColorModule) Edit(p module.Property, cause e.ActionCause) {
	if v, ok := p.Value.(float64); ok {
		_ = m.SetValue(v, cause)
	}
}

func (m *customSkyColorModule) SetValue(v float64, cause e.ActionCause, opts ...any) error {
	m.mu.Lock()
	if m.val == v {
		m.mu.Unlock()
		return e.ErrValuesIsAlready{Value: v}
	}
	m.val = v
	m.mu.Unlock()

	m.onValueChanged(v, cause)
	return nil
}

func (m *customSkyColorModule) Value() (float64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.val, true
}

func (m *customSkyColorModule) Min() float64  { return m.min }
func (m *customSkyColorModule) Max() float64  { return m.max }
func (m *customSkyColorModule) Def() float64  { return m.def }
func (m *customSkyColorModule) Step() float64 { return 0.01 }

var _ module.Config = (*CustomSkyR)(nil)

type CustomSkyR struct {
	Process        *win.Process
	Error          func(error)
	OnValueChanged func(float64, e.ActionCause)
}

func (conf *CustomSkyR) Create(p module.Property, cause e.ActionCause) (module.Module, error) {
	m := &customSkyColorModule{
		ErrorHandler:   modulesutil.ErrorHandler{Error: conf.Error},
		onValueChanged: conf.OnValueChanged,
		min:            0.0,
		max:            1.0,
		def:            1.0,
		name:           "custom sky r",
		desc:           "Red color component of the custom sky",
	}
	m.Edit(p, cause)
	return m, nil
}

func (conf *CustomSkyR) DefaultProperty() module.Property {
	return module.Property{Enabled: true, Value: 1.0}
}

func (conf *CustomSkyR) Identifier() string {
	return "custom_sky_r"
}

var _ module.Config = (*CustomSkyG)(nil)

type CustomSkyG struct {
	Process        *win.Process
	Error          func(error)
	OnValueChanged func(float64, e.ActionCause)
}

func (conf *CustomSkyG) Create(p module.Property, cause e.ActionCause) (module.Module, error) {
	m := &customSkyColorModule{
		ErrorHandler:   modulesutil.ErrorHandler{Error: conf.Error},
		onValueChanged: conf.OnValueChanged,
		min:            0.0,
		max:            1.0,
		def:            1.0,
		name:           "custom sky g",
		desc:           "Green color component of the custom sky",
	}
	m.Edit(p, cause)
	return m, nil
}

func (conf *CustomSkyG) DefaultProperty() module.Property {
	return module.Property{Enabled: true, Value: 1.0}
}

func (conf *CustomSkyG) Identifier() string {
	return "custom_sky_g"
}

var _ module.Config = (*CustomSkyB)(nil)

type CustomSkyB struct {
	Process        *win.Process
	Error          func(error)
	OnValueChanged func(float64, e.ActionCause)
}

func (conf *CustomSkyB) Create(p module.Property, cause e.ActionCause) (module.Module, error) {
	m := &customSkyColorModule{
		ErrorHandler:   modulesutil.ErrorHandler{Error: conf.Error},
		onValueChanged: conf.OnValueChanged,
		min:            0.0,
		max:            1.0,
		def:            1.0,
		name:           "custom sky b",
		desc:           "Blue color component of the custom sky",
	}
	m.Edit(p, cause)
	return m, nil
}

func (conf *CustomSkyB) DefaultProperty() module.Property {
	return module.Property{Enabled: true, Value: 1.0}
}

func (conf *CustomSkyB) Identifier() string {
	return "custom_sky_b"
}
