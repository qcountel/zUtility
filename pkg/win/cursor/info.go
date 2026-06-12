package cursor

import (
	"errors"
	"syscall"
	"unsafe"

	"github.com/tailscale/win"
)

var (
	user32            = syscall.NewLazyDLL("user32.dll")
	procGetCursorInfo = user32.NewProc("GetCursorInfo")
)

type Info struct {
	CbSize uint32
	Flags  uint32

	CursorHandle uintptr   // TODO: i dont actually remember if these are right
	PtScreenPos  win.POINT // ...
}

func NewInfo() *Info {
	i := new(Info)
	i.CbSize = uint32(unsafe.Sizeof(*i))
	return i
}

func (i *Info) Init() error {
	ret, _, err := procGetCursorInfo.Call(uintptr(unsafe.Pointer(i)))
	if ret != 0 {
		return nil
	}
	if err == nil {
		err = errors.New("cannot get cursor info")
	}
	return err
}
