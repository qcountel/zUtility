package misc

func HasTrueOption(x []bool) bool {
	return MustFirstOption(x)
}

func MustFirstOptionOr[T any, S ~[]T](x S, or T) T {
	if len(x) > 0 {
		return x[0]
	}
	return or
}

func MustFirstOption[T any, S ~[]T](x S) (zero T) {
	return MustFirstOptionOr(x, zero)
}

func MustFirstOptionCastOr[T any](x []any, or T) T {
	if len(x) == 0 {
		return or
	}
	v, ok := x[0].(T)
	if !ok {
		return or
	}
	return v
}

func MustFirstOptionCast[T any](x []any) (zero T) {
	return MustFirstOptionCastOr(x, zero)
}

func DoIf(b bool, fn func()) {
	if b {
		fn()
	}
}

func DoIfFn(b func() bool, fn func()) {
	DoIf(b(), fn)
}

func DoIfFoundFn[T any, S ~[]T](x S, target func(T) bool, fn func()) {
	for _, item := range x {
		if target(item) {
			fn()
			return
		}
	}
}

func DoIfFound[T comparable, S ~[]T](x S, target T, fn func()) {
	DoIfFoundFn(x, CompareItemFunc(target), fn)
}

func CompareItemFunc[T comparable](target T) func(T) bool {
	return func(item T) bool {
		return item == target
	}
}
