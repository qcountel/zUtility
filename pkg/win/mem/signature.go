package mem

import (
	"encoding/hex"
	"fmt"
	"strings"
)

type Signature struct {
	Data []byte
	Mask string // "xxx??x"
}

func (sig Signature) Empty() bool {
	return len(sig.Data) == 0 && sig.Mask == ""
}

func (sig Signature) ExtendFirst(x []byte) Signature {
	if len(x) > len(sig.Data) {
		panic(fmt.Errorf("mem.Signature: ExtendFirst: input length %d exceeds signature length %d", len(x), len(sig.Data)))
	}
	data := make([]byte, len(sig.Data))
	copy(data, sig.Data)

	mask := []byte(sig.Mask)
	for i := 0; i < len(x); i++ {
		data[i] = x[i]
		mask[i] = 'x'
	}
	return Signature{
		Data: data,
		Mask: string(mask),
	}
}

func MustParseSignature(from string) Signature {
	sig, err := ParseSignature(from)
	if err != nil {
		panic(fmt.Errorf("mem MustParseSignature: %w", err))
	}
	return sig
}

func ParseSignature(from string) (sig Signature, err error) {
	for _, part := range strings.Fields(from) {
		if strings.Contains(part, "?") {
			sig.Data = append(sig.Data, 0)
			sig.Mask += "?"
			continue
		}
		if len(part) == 1 {
			part = "0" + part
		}
		b, err := hex.DecodeString(part)
		if err != nil {
			return Signature{}, fmt.Errorf("invalid hex byte %q: %w", part, err)
		}
		sig.Data = append(sig.Data, b...)
		sig.Mask += "x"
	}
	return sig, nil
}

func matchSignature(data []byte, sig Signature) bool {
	for i := 0; i < len(sig.Data); i++ {
		if sig.Mask[i] == 'x' && data[i] != sig.Data[i] {
			return false
		}
	}
	return true
}

func FullMask(n int) string {
	return strings.Repeat("x", n)
}
