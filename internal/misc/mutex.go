package misc

import "sync"

type ValueWithMutex[T any] struct {
	sync.Mutex
	V T
}

type ValueWithRWMutex[T any] struct {
	sync.RWMutex
	V T
}

type RLocker interface {
	RLock()
	RUnlock()
}

func UnlockAndReturn[T any](l sync.Locker, v T) T {
	l.Unlock()
	return v
}

func RUnlockAndReturn[T any](l RLocker, v T) T {
	l.RUnlock()
	return v
}
