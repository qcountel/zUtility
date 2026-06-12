package mem

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/something-that-is-cool/zutil/pkg/win"
)

type Pointer struct {
	BaseAddress uintptr
	Offsets     []uintptr
}

func ParsePointer(addrStr, offsetsStr string) (ptr Pointer, err error) {
	if !strings.HasPrefix(addrStr, "0x") {
		addrStr = "0x" + addrStr
	}
	i, err := strconv.ParseUint(addrStr, 0, 64)
	if err != nil {
		return Pointer{}, fmt.Errorf("parse addr: %w", err)
	}
	ptr.BaseAddress = uintptr(i)
	for idx, part := range strings.Fields(offsetsStr) {
		if !strings.HasPrefix(part, "0x") {
			part = "0x" + part
		}
		offset, err := strconv.ParseUint(part, 0, 64)
		if err != nil {
			return Pointer{}, fmt.Errorf("parse offset %d (%s): %w", idx, part, err)
		}
		ptr.Offsets = append(ptr.Offsets, uintptr(offset))
	}
	return ptr, nil
}

func MustParsePointer(addrStr, offsetsStr string) Pointer {
	ptr, err := ParsePointer(addrStr, offsetsStr)
	if err != nil {
		panic(fmt.Errorf("mem: MustParsePointer: %w", err))
	}
	return ptr
}

func ResolvePointerValue[T any](proc *win.Process, ptr Pointer, opts ...any) (zero T, addr uintptr, err error) {
	finalAddr, o, err := resolvePointerAddress(proc, ptr, opts...)
	if err != nil {
		return zero, 0, fmt.Errorf("resolve pointer address: %w", err)
	}
	val, err := ReadMemory[T](proc, finalAddr, o.Protect)
	if err != nil {
		return zero, 0, fmt.Errorf("read final value: %w", err)
	}
	return val, finalAddr, nil
}

func ResolvePointerAddress(proc *win.Process, ptr Pointer, opts ...any) (uintptr, error) {
	addr, _, err := resolvePointerAddress(proc, ptr, opts...)
	return addr, err
}

func resolvePointerAddress(proc *win.Process, ptr Pointer, opts ...any) (uintptr, options, error) {
	o := handleOptions(opts)

	mod, err := proc.GetModuleInfo(o.GetModuleOpts...)
	if err != nil {
		return 0, o, fmt.Errorf("get proc module info: %w", err)
	}
	addr, err := ReadMemory[uintptr](proc, ptr.BaseAddress+mod.Address, o.Protect)
	if err != nil {
		return 0, o, fmt.Errorf("read base addr: %w", err)
	}
	for i := 0; i < len(ptr.Offsets)-1; i++ {
		addr, err = ReadMemory[uintptr](proc, addr+ptr.Offsets[i], o.Protect)
		if err != nil {
			return 0, options{}, fmt.Errorf("read offset at step %d: %w", i, err)
		}
	}
	return addr + ptr.Offsets[len(ptr.Offsets)-1], o, nil
}

type (
	OptionCustomModule struct{ Options win.GetModuleInfoOptions }
	OptionProtect      struct{}
)

type options struct {
	GetModuleOpts []win.GetModuleInfoOptions
	Protect       bool
}

func handleOptions(opts []any) (o options) {
	for _, opt := range opts {
		switch v := opt.(type) {
		case OptionProtect:
			o.Protect = true
		case OptionCustomModule:
			o.GetModuleOpts = append(o.GetModuleOpts, v.Options)
		}
	}
	return
}
