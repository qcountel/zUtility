package memutil

import (
	"fmt"
	"math"

	"github.com/something-that-is-cool/zutil/internal/misc"
	"github.com/something-that-is-cool/zutil/pkg/asm"
	"github.com/something-that-is-cool/zutil/pkg/e"
	"github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
)

type Detour struct {
	proc *win.Process

	data misc.ValueWithMutex[struct {
		init bool

		target uintptr
		dest   uintptr
		size   uint

		cave      uintptr
		origBytes []byte
	}]
}

func NewDetour(p *win.Process, target, dest uintptr, size uint) *Detour {
	d := &Detour{proc: p}
	d.data.V.target, d.data.V.dest, d.data.V.size = target, dest, size
	return d
}

func (d *Detour) Enable() (err error) {
	d.data.Lock()
	defer d.data.Unlock()

	if err = d.init(); err != nil {
		return fmt.Errorf("init: %w", err)
	}
	if err := d.writeCaveCode(); err != nil {
		return fmt.Errorf("write cave code: %w", err)
	}
	if err := d.applyTargetPatch(); err != nil {
		return fmt.Errorf("apply target patch: %w", err)
	}
	d.data.V.init = true
	return nil
}

func (d *Detour) EnableWithCode(userCode []byte) (err error) {
	d.data.Lock()
	defer d.data.Unlock()

	if err = d.init(); err != nil {
		return fmt.Errorf("init: %w", err)
	}
	finalPayload := append([]byte(nil), userCode...)
	finalPayload = append(finalPayload, d.data.V.origBytes...)
	retAddr := d.data.V.target + uintptr(d.data.V.size)
	jmpBack := asm.Jmp(d.data.V.cave+uintptr(len(finalPayload)), retAddr)
	finalPayload = append(finalPayload, jmpBack...)

	if err := mem.Patch(d.proc, d.data.V.cave, finalPayload); err != nil {
		return fmt.Errorf("patch cave failed: %w", err)
	}
	if err := d.applyTargetPatch(); err != nil {
		return fmt.Errorf("apply target patch: %w", err)
	}
	d.data.V.init = true
	return nil
}

func (d *Detour) init() (err error) {
	if d.data.V.init {
		return e.ErrAlreadyInitialized
	}
	d.data.V.origBytes, err = mem.ReadBytes(d.proc, d.data.V.target, d.data.V.size)
	if err != nil {
		return fmt.Errorf("read target: %w", err)
	}
	d.data.V.cave, err = mem.AllocNear(d.proc, d.data.V.target, 1024)
	if err != nil {
		return fmt.Errorf("alloc cave near failed: %w", err)
	}
	return nil
}

func (d *Detour) EnableProxy(proxyAddr uintptr, xmmIndex byte) error {
	shell := append([]byte(nil), asm.Pushfq())
	shell = append(shell, asm.Push(asm.Rax)...)
	shell = append(shell, asm.Mov64(asm.Rax, proxyAddr)...)
	shell = append(shell, asm.MovssXmmToRax(xmmIndex)...)
	shell = append(shell, asm.Pop(asm.Rax)...)
	return d.EnableWithCode(append(shell, asm.Popfq()))
}

func (d *Detour) applyTargetPatch() error {
	dist := int64(d.data.V.cave) - int64(d.data.V.target+5)
	if dist < math.MinInt32 || dist > math.MaxInt32 {
		return fmt.Errorf("cave out of range for relative jump (dist: %d bytes)", dist)
	}
	payload := asm.Jmp(d.data.V.target, d.data.V.cave)
	if uint(len(payload)) < d.data.V.size {
		payload = append(payload, mem.NopBytes(int(d.data.V.size)-len(payload))...)
	}
	return mem.Patch(d.proc, d.data.V.target, payload)
}

func (d *Detour) writeCaveCode() error {
	userCode, err := mem.ReadBytes(d.proc, d.data.V.dest, 64)
	if err != nil {
		return fmt.Errorf("read user code: %w", err)
	}
	code := append([]byte(nil), d.data.V.origBytes...)
	code = append(code, userCode...)

	returnAddr := d.data.V.target + uintptr(d.data.V.size)
	jmpBack := asm.Jmp(d.data.V.cave+uintptr(len(code)), returnAddr)

	code = append(code, jmpBack...)
	return mem.Patch(d.proc, d.data.V.cave, code)
}

func (d *Detour) Disable() error {
	d.data.Lock()
	defer d.data.Unlock()

	if !d.data.V.init {
		return nil
	}
	if err := mem.Patch(d.proc, d.data.V.target, d.data.V.origBytes); err != nil {
		return fmt.Errorf("restore original: %w", err)
	}
	_, err := mem.Unalloc(d.proc, d.data.V.cave)
	d.data.V.init = false
	return err
}
