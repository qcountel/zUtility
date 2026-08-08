package e

import "fmt"

type ActionCause interface {
	fmt.Stringer
	ActionCause()
}

var _ ActionCause = (*actionCause)(nil)

type actionCause string

func (a actionCause) String() string { return string(a) }
func (a actionCause) ActionCause()   {}

func NewActionCause(str string) ActionCause {
	return actionCause(str)
}

func ActionCauseIs(a, b ActionCause) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.String() == b.String()
}

var ActionCauseExternal = NewActionCause("external")
