package win

import "sync/atomic"

type ByteToggler struct {
	Process  *Process
	Address  uintptr
	Original []byte
	Patch    []byte

	state atomic.Bool
}

func (t *ByteToggler) Set(b bool) error {
	data := t.Original
	if b {
		data = t.Patch
	}
	if err := Patch(t.Process, t.Address, data); err != nil {
		return err
	}
	t.state.Store(b)
	return nil
}

func (t *ByteToggler) Enabled() bool {
	return t.state.Load()
}

func (t *ByteToggler) SetState(b bool) {
	t.state.Store(b)
}
