package memutil

import (
	"errors"
	"fmt"

	"github.com/something-that-is-cool/zutil/internal/misc"
	"github.com/something-that-is-cool/zutil/pkg/asm"
	"github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
)

type ProxiedDetour[T any] struct {
	d       *Detour
	valAddr uintptr
}

func NewProxiedDetour[T any](p *win.Process, target uintptr, size uint) *ProxiedDetour[T] {
	return &ProxiedDetour[T]{d: NewDetour(p, target, 0, size)}
}

type UserCodeFunc = func(valAddr uintptr) []byte

func (d *ProxiedDetour[T]) Enable(code UserCodeFunc, def ...T) (err error) {
	d.d.data.Lock()
	defer d.d.data.Unlock()

	if d.valAddr == 0 {
		if err := d.initUnsafe(def...); err != nil {
			return fmt.Errorf("init: %w", err)
		}
	}
	if d.d.data.V.init {
		return nil
	}
	retAddr := d.d.data.V.target + uintptr(d.d.data.V.size)
	payload := append([]byte(nil), code(d.valAddr)...)
	payload = append(payload, asm.Jmp(d.d.data.V.cave+uintptr(len(payload)), retAddr)...)

	if err = mem.Patch(d.d.proc, d.d.data.V.cave, payload); err != nil {
		d.cleanupUnsafe()
		return fmt.Errorf("patch cave: %w", err)
	}
	if err := d.d.applyTargetPatch(); err != nil {
		d.cleanupUnsafe()
		return fmt.Errorf("underlying apply target patch: %w", err)
	}
	d.d.data.V.init = true
	return nil
}

func (d *ProxiedDetour[T]) initUnsafe(val ...T) (err error) {
	if d.valAddr != 0 {
		return nil
	}
	if err = d.d.init(); err != nil {
		return fmt.Errorf("init underlying detour: %w", err)
	}
	d.valAddr, err = mem.AllocNear(d.d.proc, d.d.data.V.target, misc.SizeOf(val))
	if err != nil {
		_, _ = mem.Unalloc(d.d.proc, d.d.data.V.cave)
		return fmt.Errorf("alloc mem for proxy value: %w", err)
	}
	if err = mem.WriteMemory(d.d.proc, d.valAddr, misc.MustFirstOption(val)); err != nil {
		d.cleanupUnsafe()
		return fmt.Errorf("write initial val: %w", err)
	}
	return nil
}

var ErrNotInitialized = errors.New("not initialized")

func (d *ProxiedDetour[T]) WriteValue(val T) error {
	d.d.data.Lock()
	defer d.d.data.Unlock()

	if d.valAddr == 0 {
		return ErrNotInitialized
	}
	return mem.WriteMemory(d.d.proc, d.valAddr, val)
}

func (d *ProxiedDetour[T]) ReadValue() (T, error) {
	d.d.data.Lock()
	defer d.d.data.Unlock()

	if d.valAddr == 0 {
		var zero T
		return zero, ErrNotInitialized
	}
	return mem.ReadMemory[T](d.d.proc, d.valAddr)
}

func (d *ProxiedDetour[T]) Disable() error {
	d.d.data.Lock()
	defer d.d.data.Unlock()

	if !d.d.data.V.init {
		return ErrNotInitialized
	}
	if err := mem.Patch(d.d.proc, d.d.data.V.target, d.d.data.V.origBytes); err != nil {
		return fmt.Errorf("restore original: %w", err)
	}
	d.cleanupUnsafe()
	d.d.data.V.init = false
	return nil
}

func (d *ProxiedDetour[T]) cleanupUnsafe() {
	if d.d.data.V.cave != 0 {
		_, _ = mem.Unalloc(d.d.proc, d.d.data.V.cave)
		d.d.data.V.cave = 0
	}
	if d.valAddr != 0 {
		_, _ = mem.Unalloc(d.d.proc, d.valAddr)
		d.valAddr = 0
	}
}
