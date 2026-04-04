package win

import (
	"fmt"
	"sync/atomic"
)

// NopSig returns a slice of n NOP (0x90) bytes.
func NopSig(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = 0x90
	}
	return b
}

// SignatureNopTogglerConfig is used to create a SignatureNopToggler.
// Signature must be the exact original bytes of the instruction to patch.
type SignatureNopTogglerConfig struct {
	Process   *Process
	Module    uintptr
	Size      uintptr
	Signature []byte
}

// New scans memory for Signature and creates a SignatureNopToggler.
// It automatically detects whether the patch (NOP sled) is already applied.
func (c SignatureNopTogglerConfig) New() (*SignatureNopToggler, error) {
	if c.Process == nil {
		return nil, fmt.Errorf("sig_nop_toggler: nil process")
	}
	if len(c.Signature) == 0 {
		return nil, fmt.Errorf("sig_nop_toggler: empty signature")
	}

	nops := NopSig(len(c.Signature))

	// Try to find the original (un-patched) instruction first.
	addr, err := ScanSignature(c.Process, c.Size, c.Module, c.Signature)
	if err == nil {
		t := &SignatureNopToggler{
			process:  c.Process,
			address:  addr,
			original: c.Signature,
			nops:     nops,
		}
		// state is false (disabled) — original bytes are present
		return t, nil
	}

	// Original not found; check if the region is already NOP'd.
	addr, err = ScanSignature(c.Process, c.Size, c.Module, nops)
	if err == nil {
		t := &SignatureNopToggler{
			process:  c.Process,
			address:  addr,
			original: c.Signature,
			nops:     nops,
		}
		t.enabled.Store(true)
		return t, nil
	}

	return nil, fmt.Errorf("sig_nop_toggler: signature not found (original or patched): %w", err)
}

// SignatureNopToggler patches a memory region with NOP bytes to enable a cheat
// effect and restores the original bytes to disable it.
type SignatureNopToggler struct {
	process  *Process
	address  uintptr
	original []byte
	nops     []byte

	enabled atomic.Bool
}

// Set applies the patch (b=true) or restores original bytes (b=false).
func (t *SignatureNopToggler) Set(b bool) error {
	data := t.original
	if b {
		data = t.nops
	}
	if err := Patch(t.process, t.address, data); err != nil {
		return err
	}
	t.enabled.Store(b)
	return nil
}

// Enabled reports whether the NOP patch is currently active.
func (t *SignatureNopToggler) Enabled() bool {
	return t.enabled.Load()
}
