package win

import (
	"log/slog"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")

	getConsoleWindow = kernel32.NewProc("GetConsoleWindow")
	showWindow       = user32.NewProc("ShowWindow")
	allocConsole     = kernel32.NewProc("AllocConsole")
)

const (
	SW_HIDE = 0
	SW_SHOW = 5
)

func SetConsoleVisible(visible bool) {
	hwnd, _, _ := getConsoleWindow.Call()
	if visible {
		if hwnd == 0 {
			r, _, _ := allocConsole.Call()
			if r != 0 {
				stdout, err := windows.Open("CONOUT$", windows.O_RDWR, 0)
				if err == nil {
					windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, stdout)
					os.Stdout = os.NewFile(uintptr(stdout), "/dev/stdout")
				}
				stderr, err := windows.Open("CONOUT$", windows.O_RDWR, 0)
				if err == nil {
					windows.SetStdHandle(windows.STD_ERROR_HANDLE, stderr)
					os.Stderr = os.NewFile(uintptr(stderr), "/dev/stderr")
				}
				// Re-initialize slog to write to the new os.Stderr
				slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
				hwnd, _, _ = getConsoleWindow.Call()
			}
		}
		if hwnd != 0 {
			showWindow.Call(hwnd, SW_SHOW)
		}
	} else {
		if hwnd != 0 {
			showWindow.Call(hwnd, SW_HIDE)
		}
	}
}
