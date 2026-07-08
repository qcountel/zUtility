package asm

import "fmt"

const (
	opAddss = 0x58
	opMulss = 0x59
	opSubss = 0x5C
	opDivss = 0x5E
)

func DivssXmmMem64(xmmIndex byte, reg Register) []byte {
	return sseXmmMem64(opDivss, xmmIndex, reg)
}

func AddssXmmMem64(xmmIndex byte, reg Register) []byte {
	return sseXmmMem64(opAddss, xmmIndex, reg)
}

func SubssXmmMem64(xmmIndex byte, reg Register) []byte {
	return sseXmmMem64(opSubss, xmmIndex, reg)
}

func MulssXmmMem64(xmmIndex byte, reg Register) []byte {
	return sseXmmMem64(opMulss, xmmIndex, reg)
}

func sseXmmMem64(op byte, xmmIndex byte, reg Register) []byte {
	panicOnInvalidReg(reg)
	if xmmIndex > 15 {
		panic(fmt.Errorf("invalid xmm register: %d", xmmIndex))
	}
	res := append(make([]byte, 0, 5), 0xF3)
	res = append(calcRex(res, xmmIndex, reg), 0x0f, op)

	regField := (xmmIndex & 7) << 3
	idx := regIdx(reg)

	switch reg {
	case Rsp, R12:
		return append(res, 0x00|regField|0x04, 0x24)
	case Rbp, R13:
		return append(res, 0x40|regField|idx, 0x00)
	default:
		return append(res, 0x00|regField|idx)
	}
}

func calcRex(x []byte, xmmIndex byte, reg Register) []byte {
	rex := byte(0x40)
	if xmmIndex >= 8 {
		rex |= 0x04
	}
	if reg >= R8 {
		rex |= 0x01
	}
	if rex > 0x40 {
		return append(x, rex)
	}
	return x
}

func regIdx(reg Register) byte {
	if reg >= R8 {
		return byte(reg - R8)
	}
	return byte(reg - Rax)
}
