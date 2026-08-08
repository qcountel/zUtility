package win

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/qcountel/zUtility/internal/misc"
	w "golang.org/x/sys/windows"
)

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

func ForegroundIs(targetProc string) bool {
	pid, ok := ForegroundPID()
	if !ok {
		return false
	}
	h, err := OpenProcessHandle(pid)
	if err != nil {
		return false
	}
	defer w.CloseHandle(h)

	name, err := ProcessNameByHandle(h)
	if err != nil {
		return false
	}
	if name == targetProc {
		return true
	}
	return false
}

func ProcessNameByHandle(h w.Handle) (string, error) {
	path, err := ProcessPathByHandle(h)
	if err != nil {
		return "", err
	}
	return filepath.Base(path), nil
}

func ProcessPathByHandle(h w.Handle) (string, error) {
	buf := make([]uint16, w.MAX_PATH)
	size := uint32(len(buf))

	if err := w.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return "", fmt.Errorf("query full proc image name: %w", err)
	}
	path := w.UTF16ToString(buf[:size])
	if path == "" {
		return "", errors.New("empty path")
	}
	return path, nil
}

func ForegroundPID() (uint32, bool) {
	var pid uint32
	if _, err := w.GetWindowThreadProcessId(w.GetForegroundWindow(), &pid); err != nil {
		return 0, false
	}
	return pid, true
}

func OpenProcessHandle(pid uint32, perms ...uint32) (w.Handle, error) {
	h, err := w.OpenProcess(misc.MustFirstOptionOr(perms, w.PROCESS_QUERY_LIMITED_INFORMATION), false, pid)
	if err != nil {
		return 0, err
	}
	return h, nil
}
