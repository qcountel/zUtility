package win

import (
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	_xinputDLL          *windows.LazyDLL
	_procXInputGetState *windows.LazyProc
	_xinputOnce         sync.Once
)

func xinputInit() {
	_xinputOnce.Do(func() {

		for _, name := range []string{"xinput1_4.dll", "xinput1_3.dll", "xinput9_1_0.dll"} {
			dll := windows.NewLazySystemDLL(name)
			proc := dll.NewProc("XInputGetState")
			if err := proc.Find(); err == nil {
				_xinputDLL = dll
				_procXInputGetState = proc
				return
			}
		}
	})
}

type xinputGamepad struct {
	Buttons      uint16
	LeftTrigger  uint8
	RightTrigger uint8
	ThumbLX      int16
	ThumbLY      int16
	ThumbRX      int16
	ThumbRY      int16
}

type xinputState struct {
	PacketNumber uint32
	Gamepad      xinputGamepad
}

func xinputGetState(controllerIndex uint32) (xinputState, bool) {
	xinputInit()
	if _procXInputGetState == nil {
		return xinputState{}, false
	}
	var state xinputState
	r, _, _ := _procXInputGetState.Call(
		uintptr(controllerIndex),
		uintptr(unsafe.Pointer(&state)),
	)
	return state, r == 0
}

const (
	GPDPadUp        uint32 = 0x200
	GPDPadDown      uint32 = 0x201
	GPDPadLeft      uint32 = 0x202
	GPDPadRight     uint32 = 0x203
	GPStart         uint32 = 0x204
	GPBack          uint32 = 0x205
	GPLeftThumb     uint32 = 0x206
	GPRightThumb    uint32 = 0x207
	GPLeftShoulder  uint32 = 0x208
	GPRightShoulder uint32 = 0x209
	GPA             uint32 = 0x20A
	GPB             uint32 = 0x20B
	GPX             uint32 = 0x20C
	GPY             uint32 = 0x20D
	GPLeftTrigger   uint32 = 0x20E
	GPRightTrigger  uint32 = 0x20F
)

const gpTriggerThreshold = 30

var gpButtonMap = map[uint32]uint16{
	GPDPadUp:        0x0001,
	GPDPadDown:      0x0002,
	GPDPadLeft:      0x0004,
	GPDPadRight:     0x0008,
	GPStart:         0x0010,
	GPBack:          0x0020,
	GPLeftThumb:     0x0040,
	GPRightThumb:    0x0080,
	GPLeftShoulder:  0x0100,
	GPRightShoulder: 0x0200,
	GPA:             0x1000,
	GPB:             0x2000,
	GPX:             0x4000,
	GPY:             0x8000,
}

func GPName(vk uint32) string {
	names := map[uint32]string{
		GPDPadUp:        "D↑",
		GPDPadDown:      "D↓",
		GPDPadLeft:      "D←",
		GPDPadRight:     "D→",
		GPStart:         "Start",
		GPBack:          "Back",
		GPLeftThumb:     "L3",
		GPRightThumb:    "R3",
		GPLeftShoulder:  "LB",
		GPRightShoulder: "RB",
		GPA:             "A",
		GPB:             "B",
		GPX:             "X",
		GPY:             "Y",
		GPLeftTrigger:   "LT",
		GPRightTrigger:  "RT",
	}
	if n, ok := names[vk]; ok {
		return "GP:" + n
	}
	return ""
}

func IsGamepadVK(vk uint32) bool {
	return vk >= 0x200 && vk <= 0x2FF
}

func IsGamepadVKHeld(vk uint32) bool {
	for idx := uint32(0); idx < 4; idx++ {
		state, ok := xinputGetState(idx)
		if !ok {
			continue
		}
		if isGPVKHeld(vk, state) {
			return true
		}
	}
	return false
}

func isGPVKHeld(vk uint32, state xinputState) bool {
	switch vk {
	case GPLeftTrigger:
		return state.Gamepad.LeftTrigger > gpTriggerThreshold
	case GPRightTrigger:
		return state.Gamepad.RightTrigger > gpTriggerThreshold
	default:
		if mask, ok := gpButtonMap[vk]; ok {
			return state.Gamepad.Buttons&mask != 0
		}
	}
	return false
}

func CaptureGamepadButton(doneCh <-chan struct{}) (uint32, bool) {

	type prevState struct {
		buttons      uint16
		leftTrigger  uint8
		rightTrigger uint8
	}
	prev := [4]prevState{}
	for idx := uint32(0); idx < 4; idx++ {
		if s, ok := xinputGetState(idx); ok {
			prev[idx] = prevState{s.Gamepad.Buttons, s.Gamepad.LeftTrigger, s.Gamepad.RightTrigger}
		}
	}

	pollTicker := time.NewTicker(16 * time.Millisecond)
	defer pollTicker.Stop()

	for {
		select {
		case <-doneCh:
			return 0, false
		case <-pollTicker.C:
		}

		for idx := uint32(0); idx < 4; idx++ {
			state, ok := xinputGetState(idx)
			if !ok {
				continue
			}
			p := prev[idx]

			for vk, mask := range gpButtonMap {
				if state.Gamepad.Buttons&mask != 0 && p.buttons&mask == 0 {

					for {
						select {
						case <-doneCh:
							return 0, false
						default:
						}
						s2, ok2 := xinputGetState(idx)
						if !ok2 || s2.Gamepad.Buttons&mask == 0 {
							break
						}
					}
					return vk, true
				}
			}

			if state.Gamepad.LeftTrigger > gpTriggerThreshold && p.leftTrigger <= gpTriggerThreshold {
				for {
					select {
					case <-doneCh:
						return 0, false
					default:
					}
					s2, ok2 := xinputGetState(idx)
					if !ok2 || s2.Gamepad.LeftTrigger <= gpTriggerThreshold {
						break
					}
				}
				return GPLeftTrigger, true
			}
			if state.Gamepad.RightTrigger > gpTriggerThreshold && p.rightTrigger <= gpTriggerThreshold {
				for {
					select {
					case <-doneCh:
						return 0, false
					default:
					}
					s2, ok2 := xinputGetState(idx)
					if !ok2 || s2.Gamepad.RightTrigger <= gpTriggerThreshold {
						break
					}
				}
				return GPRightTrigger, true
			}

			prev[idx] = prevState{state.Gamepad.Buttons, state.Gamepad.LeftTrigger, state.Gamepad.RightTrigger}
		}
	}
	return 0, false
}
