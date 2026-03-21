package win

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/go-vgo/robotgo"
	w "golang.org/x/sys/windows"
)

func FindPID(name string) uint32 {
	processes, err := robotgo.Process()
	if err != nil {
		return 0
	}
	for _, proc := range processes {
		if proc.Name == name {
			return uint32(proc.Pid)
		}
	}
	return 0
}

type Process struct {
	PID    uint32
	Handle w.Handle
	Name   string

	Module     uintptr
	ModuleSize uintptr
}

func OpenProcess(name string) (*Process, error) {
	pid := FindPID(name)
	if pid <= 0 {
		return nil, errors.New("no process by name")
	}
	h, err := w.OpenProcess(w.PROCESS_ALL_ACCESS, false, pid)
	if err != nil {
		return nil, err
	}
	proc := &Process{Name: name, PID: pid, Handle: h}
	base, size, err := proc.GetModuleInfo()
	if err != nil {
		return nil, fmt.Errorf("get module info: %w", err)
	}
	proc.Module, proc.ModuleSize = base, size
	return proc, nil
}

func (proc *Process) GetModuleInfo() (uintptr, uintptr, error) {
	if !proc.Active() {
		return 0, 0, errors.New("process is not active")
	}
	if proc.Module != 0 && proc.ModuleSize != 0 {
		return proc.Module, proc.ModuleSize, nil
	}
	snapshot, err := w.CreateToolhelp32Snapshot(w.TH32CS_SNAPMODULE|w.TH32CS_SNAPMODULE32, proc.PID)
	if err != nil {
		return 0, 0, err
	}
	defer w.CloseHandle(snapshot)

	var me w.ModuleEntry32
	me.Size = uint32(unsafe.Sizeof(me))

	if err = w.Module32First(snapshot, &me); err != nil {
		return 0, 0, err
	}
	for {
		if w.UTF16ToString(me.Module[:]) == proc.Name {
			return me.ModBaseAddr, uintptr(me.ModBaseSize), nil
		}
		if err = w.Module32Next(snapshot, &me); err != nil {
			break
		}
	}
	return 0, 0, errors.New("no such module")
}

const StillActive = 259

func (proc *Process) Active() bool {
	var exitCode uint32
	if err := w.GetExitCodeProcess(proc.Handle, &exitCode); err != nil {
		return false
	}
	return exitCode == StillActive
}

func (proc *Process) Close() error {
	return w.CloseHandle(proc.Handle)
}
