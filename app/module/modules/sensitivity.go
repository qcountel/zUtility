package modules

import (
	"fmt"
	"sync"

	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/app/module/modules/modulesutil"
	"github.com/something-that-is-cool/zutil/pkg/asm"
	"github.com/something-that-is-cool/zutil/pkg/e"
	win "github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem/memutil"
)

const offsetSensitivity = 0x004956a6

var _ module.Config = (*Sensitivity)(nil)

type Sensitivity struct {
	Process        *win.Process
	Error          func(error)
	OnValueChanged func(float64, e.ActionCause)
}

func (conf *Sensitivity) Create(p module.Property, cause e.ActionCause) (module.Module, error) {
	s := &sensitivityModule{
		ErrorHandler:   modulesutil.ErrorHandler{Error: conf.Error},
		process:        conf.Process,
		onValueChanged: conf.OnValueChanged,
		min:            0.1,
		max:            3.0,
		def:            1.0,
		val:            1.0,
	}

	m := modulesutil.NewBaseValue(s,
		"sensitivity",
		"Allows to modify mouse and controller sensitivity to values higher or lower than default",
	)
	m.Edit(p, cause)
	return m, nil
}

func (conf *Sensitivity) DefaultProperty() module.Property {
	return module.Property{
		Enabled: true,
		Value:   1.0,
	}
}

func (conf *Sensitivity) Identifier() string {
	return "sensitivity"
}

type sensitivityModule struct {
	modulesutil.ErrorHandler
	mu             sync.Mutex
	process        *win.Process
	onValueChanged func(float64, e.ActionCause)
	val            float64
	min, max, def  float64
	enabled        bool
	det            *memutil.ProxiedDetour[float32]
}

func (m *sensitivityModule) Disable(cause e.ActionCause) {
	_ = m.SetValue(m.def, cause)
}

func (m *sensitivityModule) Edit(p module.Property, cause e.ActionCause) {
	if v, ok := p.Value.(float64); ok {
		_ = m.SetValue(v, cause)
	}
}

func (m *sensitivityModule) SetValue(v float64, cause e.ActionCause, opts ...any) error {
	m.mu.Lock()
	if m.val == v {
		m.mu.Unlock()
		return e.ErrValuesIsAlready{Value: v}
	}
	m.val = v
	m.mu.Unlock()

	err := m.write(v)
	if err != nil {
		m.HandleError("write sensitivity", err)
		return err
	}

	m.onValueChanged(v, cause)
	return nil
}

func (m *sensitivityModule) Value() (float64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.val, true
}

func (m *sensitivityModule) Min() float64  { return m.min }
func (m *sensitivityModule) Max() float64  { return m.max }
func (m *sensitivityModule) Def() float64  { return m.def }
func (m *sensitivityModule) Step() float64 { return 0.05 }

func (m *sensitivityModule) write(val float64) error {
	det, err := m.lazyDetour()
	if err != nil {
		return err
	}
	return det.WriteValue(float32(val))
}

func (m *sensitivityModule) lazyDetour() (*memutil.ProxiedDetour[float32], error) {
	m.mu.Lock()
	d := m.det
	m.mu.Unlock()

	if d != nil {
		return d, nil
	}

	mod, err := m.process.GetModuleInfo()
	if err != nil {
		return nil, fmt.Errorf("get module info: %w", err)
	}

	addr := mod.Address + offsetSensitivity
	det := memutil.NewProxiedDetour[float32](m.process, addr, 11)
	if err = det.Enable(sensitivityUserCode, float32(m.def)); err != nil {
		return nil, fmt.Errorf("enable detour: %w", err)
	}

	m.mu.Lock()
	m.det = det
	m.mu.Unlock()

	return det, nil
}

func sensitivityUserCode(valAddr uintptr) []byte {
	return asm.Build().
		RawStr("F3 0F 10 40 14").
		Mov64(asm.Rax, valAddr).
		MulssXmm(0, asm.Rax).
		RawStr("48 83 C4 10").
		Pop(asm.Rbx).
		Ret().
		Result()
}
