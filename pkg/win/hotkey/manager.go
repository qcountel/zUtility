package hotkey

import (
	"context"
	"sync/atomic"

	hook "github.com/robotn/gohook"
	"github.com/something-that-is-cool/zutil/internal/misc"
	"github.com/something-that-is-cool/zutil/pkg/e"
)

//fixme: gohook causes cpu spikes

type ManagerConfig struct {
	Handlers map[string]func()
}

// New ...
func (conf ManagerConfig) New() *Manager {
	if conf.Handlers == nil {
		conf.Handlers = make(map[string]func())
	}
	m := &Manager{Events: misc.ValueWithRWMutex[map[string]func()]{V: make(map[string]func())}}
	for k, h := range conf.Handlers {
		m.Events.V[k] = h
	}
	return m
}

type Manager struct {
	ctx    context.Context
	cancel context.CancelFunc

	closed  atomic.Bool
	running misc.ValueWithMutex[bool]

	Events misc.ValueWithRWMutex[map[string]func()]
}

// Run ...
func (m *Manager) Run(ctx context.Context) error {
	if err := m.initIfStarted(ctx); err != nil {
		return err
	}
	defer m.cancel()

	select {
	case <-m.ctx.Done():
		return m.ctx.Err()
	case <-hook.Process(hook.Start()):
	}
	return nil
}

func (m *Manager) initIfStarted(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	m.running.Lock()
	defer m.running.Unlock()

	if m.running.V {
		return e.ErrAlreadyRunning
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.running.V = true

	hook.Register(hook.KeyUp, nil, m.handleEvent)
	return nil
}

func (m *Manager) Handle(key string, fn func()) {
	if key == "" || fn == nil {
		return
	}
	m.Events.Lock()
	defer m.Events.Unlock()
	m.HandleUnsafe(key, fn)
}

func (m *Manager) HandleUnsafe(key string, fn func()) {
	m.Events.V[key] = fn
}

func (m *Manager) DeleteHandler(key string) {
	if key == "" {
		return
	}
	m.Events.Lock()
	defer m.Events.Unlock()
	m.DeleteHandlerUnsafe(key)
}

func (m *Manager) DeleteHandlerUnsafe(key string) {
	delete(m.Events.V, key)
}

func (m *Manager) ClearHandlers() {
	m.Events.Lock()
	defer m.Events.Unlock()
	m.ClearHandlersUnsafe()
}

func (m *Manager) ClearHandlersUnsafe() {
	clear(m.Events.V)
}

func (m *Manager) Close() error {
	if !m.closed.CompareAndSwap(false, true) {
		return e.ErrAlreadyClosed
	}
	m.running.Lock()
	defer m.running.Unlock()

	if !m.running.V {
		return nil
	}
	m.cancel()
	return nil
}

func (m *Manager) handleEvent(ev hook.Event) {
	s, ok := KeycodeToChar[ev.Keycode]
	if !ok {
		return
	}
	h, ok := func() (func(), bool) {
		m.Events.RLock()
		defer m.Events.RUnlock()

		h, ok := m.Events.V[s]
		return h, ok
	}()
	if !ok {
		return
	}
	// call handler within lock
	h()
}
