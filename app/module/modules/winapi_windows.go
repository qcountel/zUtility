package modules

import "golang.org/x/sys/windows"

var (
	_sharedUser32         = windows.NewLazySystemDLL("user32.dll")
	_procGetAsyncKeyState = _sharedUser32.NewProc("GetAsyncKeyState")
)
