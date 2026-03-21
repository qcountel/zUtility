package win

import (
	"errors"
	"fmt"
	"sync/atomic"
)

func NopSig(n int) (x []byte) {
	x = make([]byte, 0, n)
	for range n {
		x = append(x, 0x90)
	}
	return
}

type SignatureNopTogglerConfig struct {
	Process      *Process
	Module, Size uintptr
	Signature    []byte
}

func (conf SignatureNopTogglerConfig) New() (*SignatureNopToggler, error) {
	toggler := &SignatureNopToggler{pr: conf.Process, mod: conf.Module, sig: conf.Signature}
	if err := toggler.scanAddress(conf.Size); err != nil {
		return nil, fmt.Errorf("initial sig scan: %w", err)
	}
	return toggler, nil
}

type SignatureNopToggler struct {
	pr  *Process
	mod uintptr
	sig []byte

	addr  uintptr
	state atomic.Bool
}

func (t *SignatureNopToggler) Set(state bool) error {
	if !state {
		if err := t.disable(); err != nil {
			return fmt.Errorf("disable: %w", err)
		}
		return nil
	}
	if err := t.enable(); err != nil {
		return fmt.Errorf("enable: %w", err)
	}
	return nil
}

func (t *SignatureNopToggler) Toggle() error {
	return t.Set(!t.Enabled())
}

func (t *SignatureNopToggler) Enabled() bool {
	return t.state.Load()
}

func (t *SignatureNopToggler) enable() error {
	if err := Patch(t.pr, t.addr, NopSig(len(t.sig))); err != nil {
		return fmt.Errorf("patch (nop bytes): %w", err)
	}
	t.state.Store(true)
	return nil
}

func (t *SignatureNopToggler) disable() error {
	if err := Patch(t.pr, t.addr, t.sig); err != nil {
		return fmt.Errorf("patch (original sig): %w", err)
	}
	t.state.Store(false)
	return nil
}

func (t *SignatureNopToggler) scanAddress(size uintptr) error {
	if t.addr != 0 {
		return nil
	}
	addr, err := ScanSignature(t.pr, size, t.mod, t.sig)
	if err == nil {
		t.addr = addr
		return nil
	}
	addr, err = ScanSignature(t.pr, size, t.mod, NopSig(len(t.sig)))
	if err == nil {
		t.addr = addr

		t.state.Store(true)
		return nil
	}
	return errors.New("cannot find signature")
}
