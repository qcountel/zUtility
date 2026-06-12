package misc

import "strings"

type CaseInsensitiveValue[V any] struct {
	Value       V
	OriginalKey string
}

type CaseInsensitiveMap[V any] map[string]CaseInsensitiveValue[V]

func (m CaseInsensitiveMap[V]) Set(k string, v V) {
	m[strings.ToLower(k)] = CaseInsensitiveValue[V]{
		Value:       v,
		OriginalKey: k,
	}
}

func (m CaseInsensitiveMap[V]) Put(k string, v V) bool {
	if _, ok := m.Get(k); ok {
		return false
	}
	m.Set(k, v)
	return true
}

func (m CaseInsensitiveMap[V]) Get(k string) (V, bool) {
	v, ok := m.RawGet(k)
	return v.Value, ok
}

func (m CaseInsensitiveMap[V]) MustGet(k string) V {
	v, _ := m.Get(k)
	return v
}

func (m CaseInsensitiveMap[V]) RawGet(k string) (CaseInsensitiveValue[V], bool) {
	v, ok := m[strings.ToLower(k)]
	return v, ok
}

func (m CaseInsensitiveMap[V]) MustRawGet(k string) CaseInsensitiveValue[V] {
	v, _ := m.RawGet(k)
	return v
}
