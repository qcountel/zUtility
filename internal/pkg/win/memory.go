package win

import (
	"bytes"
	"errors"
	"fmt"
	"unsafe"

	w "golang.org/x/sys/windows"
)

func WriteMemory[T any](p *Process, addr uintptr, val T) error {
	size := unsafe.Sizeof(val)
	return withUnlockedPageProtection(p.Handle, addr, size, func() error {
		return w.WriteProcessMemory(
			p.Handle,
			addr,
			(*byte)(unsafe.Pointer(&val)),
			size,
			nil,
		)
	})
}

func ReadMemory[T any](p *Process, addr uintptr) (val T, err error) {
	size := unsafe.Sizeof(val)
	err = withUnlockedPageProtection(p.Handle, addr, size, func() error {
		return w.ReadProcessMemory(
			p.Handle,
			addr,
			(*byte)(unsafe.Pointer(&val)),
			size,
			nil,
		)
	})
	return val, err
}

func ResolvePointerAddress(proc *Process, mod, baseAddr uintptr, offsets []uintptr) (uintptr, error) {
	addr, err := ReadMemory[uintptr](proc, mod+baseAddr)
	if err != nil {
		return 0, fmt.Errorf("read base addr: %w", err)
	}
	for i := 0; i < len(offsets)-1; i++ {
		addr, err = ReadMemory[uintptr](proc, addr+offsets[i])
		if err != nil {
			return 0, fmt.Errorf("read offset at step %d: %w", i, err)
		}
	}
	return addr + offsets[len(offsets)-1], nil
}

func Patch(p *Process, addr uintptr, b []byte) error {
	if len(b) == 0 {
		return errors.New("empty slice")
	}
	return withUnlockedPageProtection(p.Handle, addr, uintptr(len(b)), func() error {
		if err := w.WriteProcessMemory(p.Handle, addr, &b[0], uintptr(len(b)), nil); err != nil {
			return fmt.Errorf("write memory: %w", err)
		}
		return nil
	})
}

func ScanSignature(p *Process, size, base uintptr, pattern []byte) (uintptr, error) {
	if len(pattern) == 0 {
		return 0, errors.New("empty pattern")
	}
	const chunkSize = 1 << 20

	moduleEnd := base + size
	for addr := base; addr < moduleEnd; {
		var info w.MemoryBasicInformation
		infoSize := unsafe.Sizeof(info)
		if err := w.VirtualQueryEx(p.Handle, addr, &info, infoSize); err != nil || info.RegionSize == 0 {
			addr += 0x1000
			continue
		}

		regionStart := maxUintptr(addr, info.BaseAddress)
		regionEnd := minUintptr(info.BaseAddress+info.RegionSize, moduleEnd)
		if regionEnd <= regionStart {
			addr += 0x1000
			continue
		}

		if isReadableRegion(info) {
			if matchAddr, ok := scanRegionForPattern(p.Handle, regionStart, regionEnd, pattern, chunkSize); ok {
				return matchAddr, nil
			}
		}

		addr = regionEnd
	}
	return 0, errors.New("signature not found in module memory")
}

func withUnlockedPageProtection(handle w.Handle, addr, size uintptr, do func() error) (err error) {
	if size == 0 {
		return errors.New("empty memory operation")
	}

	var oldProtect uint32
	if err = w.VirtualProtectEx(handle, addr, size, w.PAGE_EXECUTE_READWRITE, &oldProtect); err != nil {
		return fmt.Errorf("virtual protect unlock: %w", err)
	}

	defer func() {
		var tmp uint32
		if rerr := w.VirtualProtectEx(handle, addr, size, oldProtect, &tmp); rerr != nil && err == nil {
			err = fmt.Errorf("virtual protect restore: %w", rerr)
		}
	}()

	err = do()
	return
}

func isReadableRegion(info w.MemoryBasicInformation) bool {
	if info.State != w.MEM_COMMIT {
		return false
	}
	if info.Protect&w.PAGE_GUARD != 0 || info.Protect == w.PAGE_NOACCESS {
		return false
	}
	switch info.Protect & 0xFF {
	case w.PAGE_READONLY, w.PAGE_READWRITE, w.PAGE_WRITECOPY, w.PAGE_EXECUTE_READ, w.PAGE_EXECUTE_READWRITE, w.PAGE_EXECUTE_WRITECOPY:
		return true
	default:
		return false
	}
}

func scanRegionForPattern(handle w.Handle, start, end uintptr, pattern []byte, chunkSize uintptr) (uintptr, bool) {
	if len(pattern) == 0 || start >= end {
		return 0, false
	}

	patternLen := uintptr(len(pattern))
	for chunkStart := start; chunkStart < end; chunkStart += chunkSize {
		readSize := minUintptr(chunkSize, end-chunkStart)
		if end > chunkStart+readSize {
			readSize = minUintptr(readSize+patternLen-1, end-chunkStart)
		}
		if readSize == 0 {
			continue
		}

		buf := make([]byte, readSize)
		var bytesRead uintptr
		err := w.ReadProcessMemory(handle, chunkStart, &buf[0], readSize, &bytesRead)
		if err != nil && bytesRead == 0 {
			continue
		}
		buf = buf[:bytesRead]

		if idx := findPatternIndex(buf, pattern); idx >= 0 {
			return chunkStart + uintptr(idx), true
		}
	}
	return 0, false
}

func findPatternIndex(data, pattern []byte) int {
	if len(pattern) == 0 || len(data) < len(pattern) {
		return -1
	}

	limit := len(data) - len(pattern)
	first := pattern[0]
	for i := 0; i <= limit; {
		pos := bytes.IndexByte(data[i:limit+1], first)
		if pos < 0 {
			return -1
		}
		i += pos
		if bytes.Equal(data[i:i+len(pattern)], pattern) {
			return i
		}
		i++
	}
	return -1
}

func minUintptr(a, b uintptr) uintptr {
	if a < b {
		return a
	}
	return b
}

func maxUintptr(a, b uintptr) uintptr {
	if a > b {
		return a
	}
	return b
}
