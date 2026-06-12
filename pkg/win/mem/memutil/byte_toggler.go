package memutil

import (
	"sync/atomic"

	"github.com/something-that-is-cool/zutil/pkg/e"
	"github.com/something-that-is-cool/zutil/pkg/win"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
)

type ByteToggler struct {
	Process  *win.Process
	Address  uintptr
	Offset   uintptr //optional
	Original []byte
	Patch    []byte

	state atomic.Bool
}

func (t *ByteToggler) Set(v bool) error {
	if t.state.Load() == v {
		return e.ErrValuesIsAlready{Value: v}
	}
	data := t.Original
	if v {
		data = t.Patch
	}
	targetAddr := t.Address + t.Offset
	if err := mem.Patch(t.Process, targetAddr, data); err != nil {
		return mem.ErrPatchAt{Address: targetAddr, Parent: err}
	}
	t.state.Store(v)
	return nil
}

func (t *ByteToggler) Toggle() error {
	return t.Set(!t.Enabled())
}

func (t *ByteToggler) Enabled() bool {
	return t.state.Load()
}

func (t *ByteToggler) SetState(b bool) {
	t.state.Store(b)
}
