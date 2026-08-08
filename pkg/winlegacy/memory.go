package winlegacy

import (
	"errors"

	"github.com/qcountel/zUtility/pkg/win/mem"
)

func ReadMemory[T any](p *Process, addr uintptr) (T, error) {
	return mem.ReadMemory[T](p, addr)
}

func WriteMemory[T any](p *Process, addr uintptr, val T) error {
	return mem.WriteMemory[T](p, addr, val)
}

func Patch(p *Process, addr uintptr, b []byte) error {
	return mem.Patch(p, addr, b)
}

func ResolvePointerAddress(proc *Process, mod, baseAddr uintptr, offsets []uintptr) (uintptr, error) {
	ptr := mem.Pointer{
		BaseAddress: baseAddr,
		Offsets:     offsets,
	}
	return mem.ResolvePointerAddress(proc, ptr)
}

func ScanSignature(p *Process, size, base uintptr, pattern []byte) (uintptr, error) {
	if len(pattern) == 0 {
		return 0, errors.New("empty pattern")
	}
	sig := mem.Signature{
		Data: pattern,
		Mask: mem.FullMask(len(pattern)),
	}
	opts := GetModuleInfoOptions{Name: p.Name}
	addr, err := mem.ScanSignature(p, sig, opts)
	if err != nil {
		return 0, err
	}
	return addr, nil
}
