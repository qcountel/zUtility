package win

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"github.com/qcountel/zUtility/internal/misc"
	w "golang.org/x/sys/windows"
)

type ProcessModule struct {
	Address uintptr
	Size    uintptr
}

var ErrProcessInactive = errors.New("process is inactive")

type GetModuleInfoOptions struct {
	Name string
}

func (proc *Process) GetModuleInfo(opts ...GetModuleInfoOptions) (ProcessModule, error) {
	if !proc.Active() {
		return ProcessModule{}, ErrProcessInactive
	}
	opt := proc.extractGetModuleOptions(opts)

	proc.modulesMu.Lock()
	defer proc.modulesMu.Unlock()

	if m, ok := proc.modules.Get(opt.Name); ok {
		return m, nil
	}
	mod, err := proc.findModule(opt.Name)
	if err != nil {
		return ProcessModule{}, err
	}
	proc.modules.Set(opt.Name, mod)
	return mod, nil
}

func (proc *Process) findModule(name string) (ProcessModule, error) {
	var cbNeeded uint32
	hMods := make([]w.Handle, 1024)
	err := w.EnumProcessModulesEx(proc.Handle, &hMods[0], uint32(len(hMods))*uint32(unsafe.Sizeof(hMods[0])), &cbNeeded, w.LIST_MODULES_ALL)
	if err != nil {
		return ProcessModule{}, fmt.Errorf("enum process modules: %w", err)
	}

	numMods := cbNeeded / uint32(unsafe.Sizeof(hMods[0]))
	if numMods > uint32(len(hMods)) {
		numMods = uint32(len(hMods))
	}

	for i := uint32(0); i < numMods; i++ {
		hMod := hMods[i]
		var buf [256]uint16
		err := w.GetModuleBaseName(proc.Handle, hMod, &buf[0], uint32(len(buf)))
		if err != nil {
			continue
		}
		currentModule := w.UTF16ToString(buf[:])
		if strings.ToLower(currentModule) == name {
			var mi w.ModuleInfo
			err = w.GetModuleInformation(proc.Handle, hMod, &mi, uint32(unsafe.Sizeof(mi)))
			if err != nil {
				return ProcessModule{}, fmt.Errorf("get module info: %w", err)
			}
			return ProcessModule{Address: mi.BaseOfDll, Size: uintptr(mi.SizeOfImage)}, nil
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
