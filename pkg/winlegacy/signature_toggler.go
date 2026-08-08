package winlegacy

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/qcountel/zUtility/pkg/win/mem"
)

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
	nopBytes := mem.NopBytes(len(t.sig))
	if err := mem.Patch(t.pr, t.addr, nopBytes); err != nil {
		return fmt.Errorf("patch (nop bytes): %w", err)
	}
	t.state.Store(true)
	return nil
}

func (t *SignatureNopToggler) disable() error {
	if err := mem.Patch(t.pr, t.addr, t.sig); err != nil {
		return fmt.Errorf("patch (original sig): %w", err)
	}
	t.state.Store(false)
	return nil
}

func (t *SignatureNopToggler) scanAddress(size uintptr) error {
	if t.addr != 0 {
		return nil
	}
	signature := mem.Signature{Data: t.sig, Mask: mem.FullMask(len(t.sig))}
	addr, err := mem.ScanSignature(t.pr, signature, GetModuleInfoOptions{Name: t.pr.Name})
	if err == nil {
		t.addr = addr
		return nil
	}

	nopSig := mem.Signature{Data: mem.NopBytes(len(t.sig)), Mask: mem.FullMask(len(t.sig))}
	addr, err = mem.ScanSignature(t.pr, nopSig, GetModuleInfoOptions{Name: t.pr.Name})
	if err == nil {
		t.addr = addr
		t.state.Store(true)
		return nil
	}
	return errors.New("cannot find signature")
}
