package cursor

func Focused() bool {
	i := NewInfo()
	if err := i.Init(); err != nil {
		return false
	}
	return i.Flags == 0
}
