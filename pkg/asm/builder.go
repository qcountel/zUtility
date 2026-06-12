package asm

import (
	"bytes"
	"strings"
	"sync"

	"github.com/something-that-is-cool/zutil/internal/misc"
	"github.com/something-that-is-cool/zutil/pkg/win/mem"
)

var builderPool = sync.Pool{ // the pool itself is NOT currently required but when it will be required i will probably forget about it so ill make it now
	New: func() any {
		return new(Builder)
	},
}

type Builder struct {
	buf  bytes.Buffer
	mask strings.Builder
}

func Build() *Builder {
	b := builderPool.Get().(*Builder)
	b.Clear()
	return b
}

func (b *Builder) write(data ...byte) {
	b.buf.Write(data)
	b.mask.WriteString(mem.FullMask(len(data)))
}

func (b *Builder) Result() []byte {
	return b.buf.Bytes()
}

func (b *Builder) BuildSignature() mem.Signature {
	return mem.Signature{Data: b.buf.Bytes(), Mask: b.mask.String()}
}

func (b *Builder) ZeroByte() *Builder {
	return b.Raw(0x00)
}

func (b *Builder) Wildcard() *Builder {
	b.buf.WriteByte(0x00)
	b.mask.WriteByte('?')
	return b
}

// X is alias to Wildcard.
func (b *Builder) X() *Builder {
	return b.Wildcard()
}

func (b *Builder) Raw(v ...byte) *Builder {
	b.write(v...)
	return b
}

func (b *Builder) MovAxImm8(val byte) *Builder {
	return b.Raw(MovAxImm8(val)...)
}

func (b *Builder) XorChBpl() *Builder {
	return b.Raw(XorChBpl()...)
}

func (b *Builder) Jmp(from, to uintptr) *Builder {
	return b.Raw(Jmp(from, to)...)
}

func (b *Builder) Mov64(reg Register, address uintptr) *Builder {
	return b.Raw(Mov64(reg, address)...)
}

func (b *Builder) PushRax(reg Register) *Builder {
	return b.Raw(Push(reg)...)
}

func (b *Builder) Pop(reg Register) *Builder {
	return b.Raw(Pop(reg)...)
}

func (b *Builder) Pushfq() *Builder {
	return b.Raw(Pushfq())
}

func (b *Builder) Popfq() *Builder {
	return b.Raw(Popfq())
}

func (b *Builder) Ret() *Builder {
	return b.Raw(Ret())
}

func (b *Builder) MovssXmmToRax(xmmIndex byte) *Builder {
	return b.Raw(MovssXmmToRax(xmmIndex)...)
}

func (b *Builder) MovssXmm0Rax() *Builder {
	return b.Raw(MovssXmm0Rax()...)
}

func (b *Builder) MovMemsdRDX(offset byte, val float32) *Builder {
	return b.Raw(MovMemsdRDX(offset, val)...)
}

func (b *Builder) Float(val float32) *Builder {
	return b.Raw(LEFloat(val)...)
}

// Float64 is alias to Double.
func (b *Builder) Float64(val float64) *Builder {
	return b.Double(val)
}

func (b *Builder) Double(val float64) *Builder {
	return b.Raw(LEDouble(val)...)
}

func (b *Builder) Nop(amount ...int) *Builder {
	n := misc.MustFirstOptionOr(amount, 1)

	res := make([]byte, 0, n)
	for range n {
		res = append(res, Nop())
	}
	return b.Raw(res...)
}

func (b *Builder) Clear() *Builder {
	b.buf.Reset()
	b.mask.Reset()
	return b
}

func (b *Builder) ClearAndReturn() {
	b.Clear()
	builderPool.Put(b)
}
