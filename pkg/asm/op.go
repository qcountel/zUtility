package asm

import (
	"encoding/binary"
	"fmt"
	"math"
	"unsafe"

	"github.com/something-that-is-cool/zutil/internal/misc"
)

func Jmp(from, to uintptr, abs ...bool) []byte {
	if misc.HasTrueOption(abs) {
		return AbsJmp(to)
	}
	offset := int32(to - (from + 5))
	res := make([]byte, 5)
	res[0] = 0xE9
	binary.LittleEndian.PutUint32(res[1:], uint32(offset))
	return res
}

func AbsJmp(address uintptr) []byte {
	return append(Mov64(Rax, address), 0xFF, 0xE0)
}

func Mov64(reg Register, value uintptr) []byte {
	panicOnInvalidReg(reg)
	res := make([]byte, 10)
	copy(res, movPrefix(reg))
	binary.LittleEndian.PutUint64(res[2:], uint64(value))
	return res
}

func Push(reg Register) []byte {
	panicOnInvalidReg(reg)
	res, offset := pushPrefix(reg)
	res[len(res)-1] += byte(reg - offset)
	return res
}

func Pop(reg Register) []byte {
	panicOnInvalidReg(reg)
	res, offset := popPrefix(reg)
	res[len(res)-1] += byte(reg - offset)
	return res
}

func Pushfq() byte {
	return 0x9C
}

func Popfq() byte {
	return 0x9D
}

func Ret() byte {
	return 0xC3
}

func Nop() byte {
	return 0x90
}

func MovAxImm8(val byte) []byte {
	return []byte{0x66, 0xB8, val}
}

func XorChBpl() []byte {
	return []byte{0x40, 0x30, 0xED}
}

func MovssXmmToRax(xmm byte) []byte {
	if xmm <= 7 {
		return []byte{0xF3, 0x0F, 0x11, xmm << 3}
	}
	return []byte{0xF3, 0x44, 0x0F, 0x11, (xmm & 7) << 3}
}

func MovssXmm0Rax() []byte {
	return []byte{0xF3, 0x0F, 0x10, 0x00}
}

func MovMemsdRDX(offset byte, val float32) []byte {
	res := []byte{0xC7, 0x42, offset}
	return append(res, LEFloat(val)...)
}

//todo: add, sub, xor

func LEFloat(val float32) []byte {
	res := make([]byte, unsafe.Sizeof(val))
	binary.LittleEndian.PutUint32(res, math.Float32bits(val))
	return res
}

func LEDouble(val float64) []byte {
	res := make([]byte, unsafe.Sizeof(val))
	binary.LittleEndian.PutUint64(res, math.Float64bits(val))
	return res
}

func panicOnInvalidReg(reg Register) {
	if !reg.Valid() {
		panic(fmt.Errorf("invalid register %d", reg))
	}
}

func movPrefix(reg Register) []byte {
	res := make([]byte, 2)
	if reg >= R8 {
		res[0] = 0x49
		res[1] = 0xB8 + byte(reg-R8)
		return res
	}
	res[0] = 0x48
	res[1] = byte(reg)
	return res
}

func pushPrefix(reg Register) ([]byte, Register) {
	if reg >= R8 {
		return []byte{0x41, 0x50}, R8
	}
	return []byte{0x50}, Rax
}

func popPrefix(reg Register) ([]byte, Register) {
	if reg >= R8 {
		return []byte{0x41, 0x58}, R8
	}
	return []byte{0x58}, Rax
}
