// Package mem contains a set of memory utilities.
//
// credits: github.com/Zwuiix-cmd/external-aimassist
package mem

import (
	"bytes"
	"errors"
	"fmt"
	"unsafe"

	"github.com/something-that-is-cool/zutil/internal/misc"
	"github.com/something-that-is-cool/zutil/pkg/win"
	w "golang.org/x/sys/windows"
)

func WriteMemory[T any](proc *win.Process, addr uintptr, val T) error {
	if !proc.Active() {
		return win.ErrProcessInactive
	}
	if addr <= 0 {
		return ErrZeroAddress
	}
	size := unsafe.Sizeof(val) //todo: maybe make some sort of SizeOf method that checks if this is slice, also accepting custom OptionSize

	oldProtect, err := virtualProtectUnlock(proc, addr, size)
	if err != nil {
		return fmt.Errorf("virtual protect (unlock): %w", err)
	}
	defer virtualProtectLock(proc, addr, size, oldProtect)

	err = w.WriteProcessMemory(
		proc.Handle,
		addr,
		(*byte)(unsafe.Pointer(&val)),
		size,
		nil,
	)
	return err
}

func ReadMemory[T any](p *win.Process, addr uintptr, protect ...bool) (val T, err error) {
	if !p.Active() {
		return val, win.ErrProcessInactive
	}
	if addr <= 0 {
		return val, ErrZeroAddress
	}
	size := unsafe.Sizeof(val)
	if misc.HasTrueOption(protect) {
		oldProtect, err := virtualProtectUnlock(p, addr, size)
		if err != nil {
			return val, fmt.Errorf("virtual protect (unlock): %w", err)
		}
		defer virtualProtectLock(p, addr, size, oldProtect)
	}
	err = w.ReadProcessMemory(
		p.Handle,
		addr,
		(*byte)(unsafe.Pointer(&val)),
		size,
		nil,
	)
	return val, err
}

func ReadBytes(p *win.Process, addr uintptr, size uint, protect ...bool) ([]byte, error) {
	if !p.Active() {
		return nil, win.ErrProcessInactive
	}
	if addr <= 0 {
		return nil, ErrZeroAddress
	}
	if size == 0 {
		return nil, ErrZeroSize
	}
	if misc.HasTrueOption(protect) {
		oldProtect, err := virtualProtectUnlock(p, addr, uintptr(size))
		if err != nil {
			return nil, fmt.Errorf("virtual protect (unlock): %w", err)
		}
		defer virtualProtectLock(p, addr, uintptr(size), oldProtect)
	}
	buf := make([]byte, size)

	var bytesRead uintptr
	if err := w.ReadProcessMemory(p.Handle, addr, &buf[0], uintptr(size), &bytesRead); err != nil {
		return nil, err
	}
	if bytesRead != uintptr(size) {
		return nil, fmt.Errorf("read %d bytes, expected %d", bytesRead, size)
	}
	return buf, nil
}

func Patch(p *win.Process, addr uintptr, b []byte) error {
	if !p.Active() {
		return win.ErrProcessInactive
	}
	if addr <= 0 {
		return ErrZeroAddress
	}
	if len(b) == 0 {
		return ErrZeroSize
	}
	size := uintptr(len(b))

	oldProtect, err := virtualProtectUnlock(p, addr, size)
	if err != nil {
		return fmt.Errorf("virtual protect (unlock): %w", err)
	}
	defer virtualProtectLock(p, addr, size, oldProtect)

	if err = w.WriteProcessMemory(p.Handle, addr, &b[0], size, nil); err != nil {
		return fmt.Errorf("write memory: %w", err)
	}
	return nil
}

const sigChunkSize = 1024 * 1024 * 1 // 1 mb

var ErrSignatureNotFound = errors.New("signature not found")

func ScanSignature(proc *win.Process, sig Signature, opts ...win.GetModuleInfoOptions) (uintptr, error) {
	if !proc.Active() {
		return 0, win.ErrProcessInactive
	}
	if sig.Empty() {
		return 0, ErrEmptySignature
	}
	mod, err := proc.GetModuleInfo(opts...)
	if err != nil {
		return 0, fmt.Errorf("get module info: %w", err)
	}
	buffer := make([]byte, sigChunkSize)
	for offset := uintptr(0); offset < mod.Size; {
		toRead := uintptr(sigChunkSize)
		if offset+sigChunkSize > mod.Size {
			toRead = mod.Size - offset
		}
		readable, regionRemaining := regionInfo(proc.Handle, mod.Address+offset)
		if !readable {
			offset += regionRemaining
			continue
		}
		if toRead > regionRemaining {
			toRead = regionRemaining
		}
		var bytesRead uintptr
		if err := w.ReadProcessMemory(
			proc.Handle,
			mod.Address+offset,
			&buffer[0],
			toRead,
			&bytesRead,
		); err != nil || bytesRead < uintptr(len(sig.Data)) {
			offset += toRead
			continue
		}
		if foundOffset, ok := findInChunk(buffer[:bytesRead], sig); ok {
			return mod.Address + offset + uintptr(foundOffset), nil
		}
		offset += toRead - uintptr(len(sig.Data)) + 1
	}
	return 0, ErrSignatureNotFound
}

func RegionInfo(handle w.Handle, address uintptr) (bool, uintptr) {
	if handle <= 0 {
		return false, 0
	}
	return regionInfo(handle, address)
}

func regionInfo(h w.Handle, addr uintptr) (bool, uintptr) {
	const defaultRegionSize uintptr = 4096

	var mbi w.MemoryBasicInformation
	if err := w.VirtualQueryEx(h, addr, &mbi, unsafe.Sizeof(mbi)); err != nil {
		return false, defaultRegionSize
	}
	remaining := mbi.RegionSize - (addr - mbi.BaseAddress)
	return IsReadable(mbi), remaining
}

func IsReadable(mbi w.MemoryBasicInformation) bool {
	if mbi.State != w.MEM_COMMIT {
		return false
	}
	if (mbi.Protect & uint32(w.PAGE_GUARD|w.PAGE_NOACCESS)) != 0 {
		return false
	}
	return true
}

func findInChunk(chunk []byte, sig Signature) (int, bool) {
	if len(sig.Data) == 0 {
		return 0, false
	}
	for i := 0; i <= len(chunk)-len(sig.Data); {
		if sig.Mask[0] != '?' {
			idx := bytes.IndexByte(chunk[i:len(chunk)-len(sig.Data)+1], sig.Data[0])
			if idx == -1 {
				return 0, false
			}
			i += idx
		}
		if matchSignature(chunk[i:i+len(sig.Data)], sig) {
			return i, true
		}
		i++
	}
	return 0, false
}

func virtualProtectUnlock(proc *win.Process, addr, size uintptr) (p uint32, err error) {
	err = w.VirtualProtectEx(proc.Handle, addr, size, w.PAGE_EXECUTE_READWRITE, &p)
	if err != nil {
		return 0, fmt.Errorf("virtual protect (unlock): %w", err)
	}
	return p, nil
}

func virtualProtectLock(proc *win.Process, addr, size uintptr, p uint32) {
	var temp uint32
	_ = w.VirtualProtectEx(proc.Handle, addr, size, p, &temp)
}

func NopBytes(size int) []byte {
	res := make([]byte, 0, size)
	for range size {
		res = append(res, 0x90) //fixme: asm.Nop (import cycle)
	}
	return res
}
