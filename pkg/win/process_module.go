package win

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"github.com/something-that-is-cool/zutil/internal/misc"
	w "golang.org/x/sys/windows"
)

type ProcessModule struct {
	Address uintptr
	Size    uintptr
}

var ErrProcessInactive = errors.New("process is inactive")

type GetModuleInfoOptions struct {
	Name string
	// some future options
}

func (proc *Process) GetModuleInfo(opts ...GetModuleInfoOptions) (ProcessModule, error) {
	if !proc.Active() {
		return ProcessModule{}, ErrProcessInactive
	}
	opt := proc.extractGetModuleOptions(opts)

	proc.modules.Lock()
	defer proc.modules.Unlock()

	if m, ok := proc.modules.V.Get(opt.Name); ok {
		return m, nil
	}
	mod, err := proc.findModule(opt.Name)
	if err != nil {
		return ProcessModule{}, err
	}
	proc.modules.V.Set(opt.Name, mod)
	return mod, nil
}

func (proc *Process) findModule(name string) (ProcessModule, error) {
	snapshot, err := w.CreateToolhelp32Snapshot(w.TH32CS_SNAPMODULE|w.TH32CS_SNAPMODULE32, proc.PID)
	if err != nil {
		return ProcessModule{}, fmt.Errorf("create toolhelp snapshot: %w", err)
	}
	defer w.CloseHandle(snapshot) //nolint:errcheck

	var me w.ModuleEntry32
	me.Size = uint32(unsafe.Sizeof(me))

	if err = w.Module32First(snapshot, &me); err != nil {
		return ProcessModule{}, fmt.Errorf("start iterating (m32 first): %w", err)
	}
	for {
		currentModule := w.UTF16ToString(me.Module[:])
		if strings.ToLower(currentModule) == name {
			mod := ProcessModule{Address: me.ModBaseAddr, Size: uintptr(me.ModBaseSize)}
			return mod, nil
		}
		if err = w.Module32Next(snapshot, &me); err != nil {
			break
		}
	}
	return ProcessModule{}, errors.New("module not found")
}

func (proc *Process) extractGetModuleOptions(x []GetModuleInfoOptions) GetModuleInfoOptions {
	opt := misc.MustFirstOption(x)
	if opt.Name == "" {
		opt.Name = proc.Name
	}
	opt.Name = strings.ToLower(opt.Name)
	return opt
}
