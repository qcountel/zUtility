package win

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/something-that-is-cool/zutil/internal/misc"
	"github.com/something-that-is-cool/zutil/pkg/e"
	w "golang.org/x/sys/windows"
)

type ProcessTrackerConfig struct {
	OnClose []func()
	Process *Process
}

func (conf ProcessTrackerConfig) New() (tr *ProcessTracker, err error) {
	if conf.Process == nil {
		return nil, errors.New("nil process")
	}
	return &ProcessTracker{pr: conf.Process, handlers: conf.OnClose}, nil
}

type ProcessTracker struct {
	pr *Process

	handlers []func()

	ctx    context.Context
	cancel context.CancelFunc

	closed  atomic.Bool
	running misc.ValueWithMutex[bool]

	stop w.Handle
}

func (tr *ProcessTracker) Process() *Process {
	return tr.pr
}

func (tr *ProcessTracker) Close() bool {
	if !tr.closed.CompareAndSwap(false, true) {
		return false //e.ErrAlreadyClosed
	}
	tr.running.Lock()
	defer tr.running.Unlock()

	if !tr.running.V {
		return true
	}
	tr.cancel()
	tr.closeStopHandle()
	return true
}

func (tr *ProcessTracker) closeStopHandle() {
	_ = w.SetEvent(tr.stop)    // signal WaitForMultipleObjects to exit
	_ = w.CloseHandle(tr.stop) // then close the handle
}

func (tr *ProcessTracker) Run(parent context.Context) (err error) {
	if tr.closed.Load() {
		return e.ErrClosed
	}
	if !tr.pr.Active() {
		return ErrProcessInactive
	}
	select {
	case <-parent.Done():
		return parent.Err()
	default:
	}
	tr.running.Lock()
	if tr.running.V {
		tr.running.Unlock()
		return e.ErrAlreadyRunning
	}
	tr.stop, err = w.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return fmt.Errorf("create stop event: %w", err)
	}
	tr.ctx, tr.cancel = context.WithCancel(parent)
	defer tr.cancel()

	tr.running.V = true
	tr.running.Unlock()

	err = tr.loop() //blocks
	// call handlers as soon as loop returned
	misc.CallMethods(tr.handlers)
	return err
}

func (tr *ProcessTracker) loop() error {
	go func() {
		<-tr.ctx.Done()
		tr.closeStopHandle()
	}()
	idx, err := w.WaitForMultipleObjects([]w.Handle{tr.pr.Handle, tr.stop}, false, w.INFINITE)
	if err != nil {
		return err
	}
	if idx == w.WAIT_OBJECT_0+1 {
		return tr.ctx.Err()
	}
	return nil
}

func (tr *ProcessTracker) CloseWithProcess() error {
	if !tr.Close() {
		return e.ErrClosed
	}
	if err := tr.pr.Close(); err != nil {
		return fmt.Errorf("close process: %w", err)
	}
	return nil
}
