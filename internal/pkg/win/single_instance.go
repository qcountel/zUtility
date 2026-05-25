package win

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modKernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procCreateMutexExW       = modKernel32.NewProc("CreateMutexExW")
	modUser32                = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows2         = modUser32.NewProc("EnumWindows")
	procGetWindowTextW2      = modUser32.NewProc("GetWindowTextW")
	procShowWindow2          = modUser32.NewProc("ShowWindow")
	procSetForegroundWindow2 = modUser32.NewProc("SetForegroundWindow")
	procIsIconic2            = modUser32.NewProc("IsIconic")
)

const (
	singleInstMutexName = "Global\\zUtility_SingleInstance_v115_b3"
	swRestore2          = 9
	mutexModifyState    = 0x0001
	synchronize         = 0x00100000
)

func EnsureSingleInstance(windowTitle string) (alreadyRunning bool, cleanup func()) {
	namePtr, _ := windows.UTF16PtrFromString(singleInstMutexName)

	handle, _, lastErr := procCreateMutexExW.Call(
		0,
		uintptr(unsafe.Pointer(namePtr)),
		1,
		uintptr(synchronize|mutexModifyState),
	)

	if handle == 0 {
		focusWindow(windowTitle)
		return true, func() {}
	}

	if windows.Errno(lastErr.(windows.Errno)) == windows.ERROR_ALREADY_EXISTS {
		windows.CloseHandle(windows.Handle(handle))
		focusWindow(windowTitle)
		return true, func() {}
	}

	return false, func() {
		windows.ReleaseMutex(windows.Handle(handle))
		windows.CloseHandle(windows.Handle(handle))
	}
}

func focusWindow(title string) {
	cb := windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var buf [512]uint16
		procGetWindowTextW2.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 512)
		text := windows.UTF16ToString(buf[:])
		if text == title {
			iconic, _, _ := procIsIconic2.Call(hwnd)
			if iconic != 0 {
				procShowWindow2.Call(hwnd, swRestore2)
			}
			procSetForegroundWindow2.Call(hwnd)
			return 0
		}
		return 1
	})
	procEnumWindows2.Call(cb, 0)
}
