package modulesutil

import (
	"errors"
	"fmt"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/something-that-is-cool/zutil/app/module"
	"github.com/something-that-is-cool/zutil/internal/misc"
	"github.com/something-that-is-cool/zutil/pkg/e"
	"github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
)

type Float32Module struct {
	Process *win.Process
	Error   func(error)

	Min, Max, Default, Step float64
	ShowRemainer            bool

	SliderToMemory func(float64) float32
	MemoryToSlider func(float32) float64

	FinalAddress   uintptr
	ResolveAddress func() (uintptr, error)

	OnValueChanged func(float64, e.ActionCause)

	ErrorOnInitialRead bool
}

// New ...
func (conf Float32Module) New(initialCause e.ActionCause) (ModuleWithValue[float64], error) {
	if conf.FinalAddress <= 0 && conf.ResolveAddress == nil {
		return nil, errors.New("must either enter final address or resolve func")
	}
	if conf.SliderToMemory == nil {
		conf.SliderToMemory = func(f float64) float32 { return float32(f) }
	}
	if conf.MemoryToSlider == nil {
		conf.MemoryToSlider = func(f float32) float64 { return float64(f) }
	}
	f := &float32Module{
		ErrorHandler:   ErrorHandler{Error: conf.Error},
		proc:           conf.Process,
		sToM:           conf.SliderToMemory,
		mToS:           conf.MemoryToSlider,
		min:            conf.Min,
		max:            conf.Max,
		def:            conf.Default,
		step:           conf.Step,
		a:              conf.FinalAddress,
		resolve:        conf.ResolveAddress,
		onValueChanged: conf.OnValueChanged,
	}
	v, err := f.initialRead()
	if err != nil {
		err := fmt.Errorf("initial read: %w", err)
		if conf.ErrorOnInitialRead {
			return nil, err
		}
		conf.Error(err)
		v = conf.Default
	}
	f.val.V.v = v
	f.val.V.notFirst = true

	// Run initial value check/sync
	if f.forceWrite(v) {
		f.onValueChanged(v, initialCause)
	}

	return f, nil
}

var _ ModuleWithValue[float64] = (*float32Module)(nil)

type float32Module struct {
	e.ErrorHandler

	proc *win.Process

	sToM func(float64) float32
	mToS func(float32) float64

	min, max, def float64
	step          float64

	onValueChanged func(float64, e.ActionCause)

	val misc.ValueWithRWMutex[struct {
		v        float64
		notFirst bool
	}]
	a uintptr

	resolve func() (uintptr, error)
}

// SetValue ...
func (m *float32Module) SetValue(v float64, cause e.ActionCause, opts ...any) error {
	currentVal, ok := m.Value()
	if ok && mgl64.FloatEqual(currentVal, v) {
		return e.ErrValuesIsAlready{Value: v}
	}
	if cause == nil {
		cause = e.ActionCauseExternal
	}
	if !m.forceWrite(v) {
		return errors.New("cannot write value")
	}
	m.onValueChanged(v, cause)
	return nil
}

// Value ...
func (m *float32Module) Value() (float64, bool) {
	m.val.RLock()
	defer m.val.RUnlock()

	if !m.val.V.notFirst {
		return 0, false
	}
	return m.val.V.v, true
}

func (m *float32Module) Min() float64 { return m.min }
func (m *float32Module) Max() float64 { return m.max }
func (m *float32Module) Def() float64 { return m.def }
func (m *float32Module) Step() float64 { return m.step }

// Disable ...
func (m *float32Module) Disable(e.ActionCause) {
	m.forceWrite(m.def)
}

// Edit ...
func (m *float32Module) Edit(p module.Property, cause e.ActionCause) {
	SyncValue[float64](m, p, cause)
}

func (m *float32Module) write(val float64) error {
	m.val.Lock()
	defer m.val.Unlock()

	if m.val.V.notFirst && mgl64.FloatEqual(m.val.V.v, val) {
		return e.ErrValuesIsAlready{Value: val}
	}
	addr, err := m.resolveAddress()
	if err != nil {
		return fmt.Errorf("resolve address: %w", err)
	}
	toWrite := m.sToM(val)
	if err = mem.WriteMemory[float32](m.proc, addr, toWrite); err != nil {
		m.a = 0
		return fmt.Errorf("write memory: %w", err)
	}
	m.val.V.v = val
	m.val.V.notFirst = true
	return nil
}

func (m *float32Module) forceWrite(val float64) bool {
	err := m.write(val)
	if err != nil {
		m.HandleError(fmt.Sprintf("write %g", val), err)
		return false
	}
	return true
}

func (m *float32Module) initialRead() (float64, error) {
	addr, err := m.resolveAddress()
	if err != nil {
		return 0, fmt.Errorf("resolve address: %w", err)
	}
	v, err := mem.ReadMemory[float32](m.proc, addr)
	if err != nil {
		m.a = 0
		return 0, fmt.Errorf("read memory: %w", err)
	}
	return m.mToS(v), nil
}

func (m *float32Module) resolveAddress() (uintptr, error) {
	if m.a != 0 {
		return m.a, nil
	}
	a, err := m.resolve()
	if err != nil {
		return 0, err
	}
	m.a = a
	return a, nil
}
