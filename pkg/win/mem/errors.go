package mem

import (
	"errors"
	"fmt"
)

var (
	ErrZeroAddress    = errors.New("zero address")
	ErrZeroSize       = errors.New("zero size")
	ErrEmptySignature = errors.New("empty signature")
	ErrCannotCallProc = errors.New("cannot call proc")
)

type ErrPatchAt struct {
	Address uintptr
	Parent  error
}

func (err ErrPatchAt) Error() string {
	return fmt.Sprintf("patch at 0x%x: %s", err.Address, err.Parent.Error())
}
