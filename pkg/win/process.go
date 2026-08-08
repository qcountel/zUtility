package win

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/qcountel/zUtility/internal/misc"
	w "golang.org/x/sys/windows"
)

func FindPID(name string, caseInsensitive ...bool) uint32 {
	snapshot, err := w.CreateToolhelp32Snapshot(w.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0
	}
	defer w.CloseHandle(snapshot)

	var pe w.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))

	if err = w.Process32First(snapshot, &pe); err != nil {
		return 0
	}

	for {
		procName := w.UTF16ToString(pe.ExeFile[:])
		if misc.CompareString(procName, name, caseInsensitive...) {
			return pe.ProcessID
		}
		if err = w.Process32Next(snapshot, &pe); err != nil {
			break
		}
	}
	return 0
}

type Process struct {
	PID        uint32
	Handle     w.Handle
	Name       string
	Module     uintptr
	ModuleSize uintptr

	modules   misc.CaseInsensitiveMap[ProcessModule]
	modulesMu sync.RWMutex
}

func OpenProcess(name string, notLoadModule ...bool) (*Process, error) {
	pid := FindPID(name)
	if pid <= 0 {
		return nil, errors.New("no process by name")
	}
	h, err := w.OpenProcess(w.PROCESS_ALL_ACCESS|w.SYNCHRONIZE, false, pid)
	if err != nil {
		return nil, err
	}
	proc := &Process{
		Handle:  h,
		Name:    name,
		PID:     pid,
		modules: make(misc.CaseInsensitiveMap[ProcessModule]),
	}
	if !misc.HasTrueOption(notLoadModule) {
		mod, err := proc.GetModuleInfo()
		if err != nil {
			return nil, fmt.Errorf("get module info: %w", err)
		}
		proc.modules.Set(proc.Name, mod)
		proc.Module = mod.Address
		proc.ModuleSize = mod.Size
	}
	return proc, nil
}

func (proc *Process) Close() error {
	return w.CloseHandle(proc.Handle)
}
