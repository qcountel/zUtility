package winlegacy

import (
	"github.com/qcountel/zUtility/pkg/win"
)

// Type aliases for seamless integration
type Process = win.Process
type ProcessTracker = win.ProcessTracker
type ProcessTrackerConfig = win.ProcessTrackerConfig
type GetModuleInfoOptions = win.GetModuleInfoOptions
type ProcessModule = win.ProcessModule

// Forwarding functions
func OpenProcess(name string, notLoadModule ...bool) (*Process, error) {
	return win.OpenProcess(name, notLoadModule...)
}

func EnsureSingleInstance(windowTitle string) (alreadyRunning bool, cleanup func()) {
	return win.EnsureSingleInstance(windowTitle)
}

func FindPID(name string, caseInsensitive ...bool) uint32 {
	return win.FindPID(name, caseInsensitive...)
}

func ForegroundIs(targetProc string) bool {
	return win.ForegroundIs(targetProc)
}

func IsGamepadVK(vk uint32) bool {
	return win.IsGamepadVK(vk)
}

func IsGamepadVKHeld(vk uint32) bool {
	return win.IsGamepadVKHeld(vk)
}

func CaptureGamepadButton(doneCh <-chan struct{}) (uint32, bool) {
	return win.CaptureGamepadButton(doneCh)
}

func GPName(vk uint32) string {
	return win.GPName(vk)
}

func SetConsoleVisible(visible bool) {
	win.SetConsoleVisible(visible)
}

