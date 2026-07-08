package misc

import "unsafe"

func SizeOf[T any](_ ...T) uintptr {
	var zero T
	return unsafe.Sizeof(zero)
}
