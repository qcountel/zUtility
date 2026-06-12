package asm

type Register byte

const (
	RNone Register = 0xB7 + iota
	Rax            //start with 0xB8
	Rcx
	Rdx
	Rbx
	Rsp
	Rbp
	Rsi
	Rdi
	R8
	R9
	R10
	R11
	R12
	R13
	R14
	R15
	RCount
)

// Valid ...
func (reg Register) Valid() bool {
	return reg > RNone && reg < RCount
}
