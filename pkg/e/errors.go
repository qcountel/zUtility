package e

import (
	"errors"
	"fmt"
)

var ErrAlreadyRunning = errors.New("already running")

var ErrAlreadyInitialized = errors.New("already initialized")

var ErrAlreadyClosed = errors.New("already closed")

var ErrClosed = errors.New("closed")

var ErrNotRunning = errors.New("not running")

type ErrValuesIsAlready struct {
	Value any
}

func (err ErrValuesIsAlready) Error() string {
	return fmt.Sprintf("value is already %v", err.Value)
}
