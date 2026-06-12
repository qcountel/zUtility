package win

import (
	"errors"
	"fmt"

	"github.com/go-vgo/robotgo"
	"github.com/something-that-is-cool/zutil/internal/misc"
	w "golang.org/x/sys/windows"
)

func FindPID(name string, caseInsensitive ...bool) uint32 {
	processes, err := robotgo.Process()
	if err != nil {
		return 0
	}
	for _, proc := range processes {
		if misc.CompareString(proc.Name, name, caseInsensitive...) {
			return uint32(proc.Pid)
		}
	}
	return 0
}

type Process struct {
	PID    uint32
	Handle w.Handle
	Name   string

	modules misc.ValueWithMutex[misc.CaseInsensitiveMap[ProcessModule]]
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
		Name:   name,
		PID:    pid,
		Handle: h,
		modules: misc.ValueWithMutex[misc.CaseInsensitiveMap[ProcessModule]]{
			V: make(misc.CaseInsensitiveMap[ProcessModule]),
		},
	}
	if !misc.HasTrueOption(notLoadModule) {
		mod, err := proc.GetModuleInfo()
		if err != nil {
			return nil, fmt.Errorf("get module info: %w", err)
		}
		proc.modules.V.Set(proc.Name, mod)
	}
	return proc, nil
}

//goland:noinspection GoSnakeCaseUsage: win api constant
const STILL_ACTIVE = 259

func (proc *Process) Active() bool {
	if proc.Handle == w.InvalidHandle {
		return false
	}
	var exitCode uint32
	if err := w.GetExitCodeProcess(proc.Handle, &exitCode); err != nil {
		return false
	}
	return exitCode == STILL_ACTIVE
}

func (proc *Process) Close() error {
	return w.CloseHandle(proc.Handle)
}
